package scenarios

import (
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	v1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
)

var (
	driveGVR = schema.GroupVersionResource{
		Group:    v1.CSICRsGroupVersion,
		Version:  "v1",
		Resource: "drives",
	}

	acGVR = schema.GroupVersionResource{
		Group:    v1.CSICRsGroupVersion,
		Version:  "v1",
		Resource: "availablecapacities",
	}

	volumeGVR = schema.GroupVersionResource{
		Group:    v1.CSICRsGroupVersion,
		Version:  "v1",
		Resource: "volumes",
	}

	storageClassPrefix = "baremetal-csi-sc"
	pvcName            = "baremetal-csi-pvc"
)

func DefineCustomTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi custom tests", func() {
		// It consists of two steps. 1) Set random drive to Failed state and see that amount of ACs reduced by 1.
		// 2) Install pod with pvc. Set drive which is used by pvc to Failed state. See that appropriate VolumeCR
		// changes its status too.
		healthCheckTest(driver)
	})
}

func healthCheckTest(driver testsuites.TestDriver) {
	var (
		pod           *corev1.Pod
		pvc           *corev1.PersistentVolumeClaim
		driverCleanup func()
	)

	f := framework.NewDefaultFramework("health")

	init := func() {
		_, driverCleanup = driver.PrepareTest(f)
	}

	// This function deletes pod if it was installed during test. And waits for its correct deletion to perform
	// NodeUnpublish and NodeUnstage properly. Next it deletes PVC and waits for correctly deletion of bounded PV
	// to clear device for next tests (CSI performs wipefs during PV deletion). The last step is the deletion of driver.
	cleanup := func() {
		ns := f.Namespace.Name

		if pod != nil {
			_ = framework.DeletePodWithWait(f, f.ClientSet, pod)
		}

		if pvc != nil {
			pv, _ := framework.GetBoundPV(f.ClientSet, pvc)
			err := framework.DeletePersistentVolumeClaim(f.ClientSet, pvcName, ns)
			if err != nil {
				e2elog.Logf("failed to delete pvc %v", err)
			}
			if pv != nil {
				//Wait for pv deletion to clear devices for future tests
				_ = framework.WaitForPersistentVolumeDeleted(f.ClientSet, pv.Name, 5*time.Second, 2*time.Minute)
			}
		}

		// Removes all driver's manifests installed during init(). (Driver, its RBACs, SC)
		if driverCleanup != nil {
			driverCleanup()
			driverCleanup = nil
		}
	}

	ginkgo.It("should discover drives' health changes and delete ac or change volume health", func() {
		init()
		defer cleanup()

		ns := f.Namespace.Name

		err := framework.WaitForPodsRunningReady(f.ClientSet, ns, 2, 0, 90*time.Second, nil)
		framework.ExpectNoError(err)

		podList, err := framework.GetPodsInNamespace(f.ClientSet, ns, nil)
		framework.ExpectNoError(err)
		csiNode := findPodNameBySubstring(podList, "baremetal-csi-node")

		acUnstructuredList, err := f.DynamicClient.Resource(acGVR).List(metav1.ListOptions{})
		framework.ExpectNoError(err)

		amountOfACBeforeDiskFailure := len(acUnstructuredList.Items)
		e2elog.Logf("found %d ac", amountOfACBeforeDiskFailure)

		// Get SN of drive from AC and set this drive to fail state
		drivesUnstructuredList, _ := f.DynamicClient.Resource(driveGVR).List(metav1.ListOptions{})
		acToDelete, _, err := unstructured.NestedString(acUnstructuredList.Items[0].Object, "spec", "Location")
		framework.ExpectNoError(err)
		f.ExecShellInContainer(csiNode, "hwmgr",
			constructHALOverrideCmd(findSNByDriveLocation(drivesUnstructuredList.Items, acToDelete)))

		// Wait until VolumeManager's Discover will see changes from HWMgr
		time.Sleep(30 * time.Second)

		acUnstructuredList, err = f.DynamicClient.Resource(acGVR).List(metav1.ListOptions{})
		framework.ExpectNoError(err)

		Expect(len(acUnstructuredList.Items)).To(Equal(amountOfACBeforeDiskFailure - 1))

		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).
			Create(constructPVC(driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), ns))
		framework.ExpectNoError(err)

		pod, err = framework.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		volumesUnstructuredList, _ := f.DynamicClient.Resource(volumeGVR).List(metav1.ListOptions{})
		location, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "spec", "Location")
		volumeName, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "metadata", "name")
		framework.ExpectNoError(err)

		f.ExecShellInContainer(csiNode, "hwmgr",
			constructHALOverrideCmd(findSNByDriveLocation(drivesUnstructuredList.Items, location)))

		// Wait until VolumeManager's Discover will see changes from HWMgr
		time.Sleep(30 * time.Second)

		changedVolume, err := f.DynamicClient.Resource(volumeGVR).Namespace(ns).Get(volumeName, metav1.GetOptions{})
		framework.ExpectNoError(err)
		health, _, err := unstructured.NestedInt64(changedVolume.Object, "spec", "Health")

		Expect(api.Health(health)).To(Equal(api.Health_BAD))
	})
}

func constructPVC(claimSize string, ns string) *corev1.PersistentVolumeClaim {
	storageClassName := storageClassPrefix + "-" + ns
	claim := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: ns,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(claimSize),
				},
			},
			StorageClassName: &storageClassName,
		},
	}

	return &claim
}

func findPodNameBySubstring(pods []*corev1.Pod, substring string) string {
	for _, pod := range pods {
		if strings.Contains(pod.Name, substring) {
			return pod.Name
		}
	}
	return ""
}

// Function to simulate drive failure
func constructHALOverrideCmd(serialNumber string) string {
	return fmt.Sprintf("cat >> /opt/emc/hal/etc/.hal_override << EOF\n"+
		"disk_status=%s,Failed\n"+
		"EOF", serialNumber)
}

// Finds SerialNumber of the drive which is used by the volume
func findSNByDriveLocation(driveList []unstructured.Unstructured, driveLocation string) string {
	for _, unstrDrive := range driveList {
		name, _, _ := unstructured.NestedString(unstrDrive.Object, "metadata", "name")
		if name == driveLocation {
			sn, _, _ := unstructured.NestedString(unstrDrive.Object, "spec", "SerialNumber")
			return sn
		}
	}
	return ""
}
