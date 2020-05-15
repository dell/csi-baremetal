package scenarios

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	pode2e "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

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

	pvcName            = "baremetal-csi-pvc"
	configKey          = "config.yaml"
)

// DefineCustomTestSuite defines custom baremetal-csi e2e tests
func DefineCustomTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi custom tests", func() {
		// It consists of two steps. 1) Set random drive to Failed state and see that amount of ACs reduced by 1.
		// 2) Install pod with pvc. Set drive which is used by pvc to Failed state. See that appropriate VolumeCR
		// changes its status too.
		healthCheckTest(driver)
	})
}

// healthCheckTest test checks behavior of driver when drives change health from GOOD to BAD
func healthCheckTest(driver testsuites.TestDriver) {
	var (
		pod           *corev1.Pod
		pvc           *corev1.PersistentVolumeClaim
		sc            *v12.StorageClass
		driverCleanup func()
	)

	f := framework.NewDefaultFramework("health")

	init := func() {
		var perTestConf *testsuites.PerTestConfig
		perTestConf, driverCleanup = driver.PrepareTest(f)
		sc = driver.(*baremetalDriver).GetDynamicProvisionStorageClass(perTestConf, "xfs")
		var err error
		sc, err = f.ClientSet.StorageV1().StorageClasses().Create(sc)
		framework.ExpectNoError(err)
	}

	// This function deletes pod if it was installed during test. And waits for its correct deletion to perform
	// NodeUnpublish and NodeUnstage properly. Next it deletes PVC and waits for correctly deletion of bounded PV
	// to clear device for next tests (CSI performs wipefs during PV deletion). The last step is the deletion of driver.
	cleanup := func() {
		ns := f.Namespace.Name

		if pod != nil {
			_ = pode2e.DeletePodWithWait(f.ClientSet, pod)
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

		// Wait for csi pods to be running and ready
		err := pode2e.WaitForPodsRunningReady(f.ClientSet, ns, 2, 0, 90*time.Second, nil)
		framework.ExpectNoError(err)

		// Get ACs from the cluster
		acUnstructuredList, err := f.DynamicClient.Resource(acGVR).Namespace(ns).List(metav1.ListOptions{})
		framework.ExpectNoError(err)

		// Save amount of ACs before drive's health changing
		amountOfACBeforeDiskFailure := len(acUnstructuredList.Items)
		e2elog.Logf("found %d ac", amountOfACBeforeDiskFailure)

		// Prepare variables to find serialNumber of drive which should be unhealthy
		drivesUnstructuredList, _ := f.DynamicClient.Resource(driveGVR).Namespace(ns).List(metav1.ListOptions{})
		acToDelete, _, err := unstructured.NestedString(acUnstructuredList.Items[0].Object, "spec", "Location")
		framework.ExpectNoError(err)
		nodeOfAC, _, err := unstructured.NestedString(acUnstructuredList.Items[0].Object, "spec", "NodeId")
		framework.ExpectNoError(err)

		// Get current loopback-config from the cluster
		cm, err := f.ClientSet.CoreV1().ConfigMaps(ns).Get(cmName, metav1.GetOptions{})
		framework.ExpectNoError(err)
		// Append bad-health drive to this config and update config on the cluster side
		appendBadHealthDriveToConfig(cm, findSNByDriveLocation(drivesUnstructuredList.Items, acToDelete), nodeOfAC)
		cm, err = f.ClientSet.CoreV1().ConfigMaps(ns).Update(cm)
		framework.ExpectNoError(err)

		// k8s docs say that time from updating ConfigMap to receiving updated ConfigMap in the pod where it's mounted
		// could last 1 minute by default
		time.Sleep(time.Minute)

		// Read ACs one more time
		acUnstructuredList, err = f.DynamicClient.Resource(acGVR).Namespace(ns).List(metav1.ListOptions{})
		framework.ExpectNoError(err)

		// Check that amount of ACs reduced by one
		Expect(len(acUnstructuredList.Items)).To(Equal(amountOfACBeforeDiskFailure - 1))

		// Create test pvc on the cluster
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).
			Create(constructPVC(ns, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), sc.Name))
		framework.ExpectNoError(err)

		// Create test pod that consumes the pvc
		pod, err = pode2e.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		// Get Volume CRs and save variables to identify on which drive the pod's Volume based on
		volumesUnstructuredList, _ := f.DynamicClient.Resource(volumeGVR).List(metav1.ListOptions{})
		location, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "spec", "Location")
		framework.ExpectNoError(err)
		volumeName, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "metadata", "name")
		framework.ExpectNoError(err)
		nodeOfVolume, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "spec", "NodeId")
		framework.ExpectNoError(err)

		// Make the drive of the Volume unhealthy
		appendBadHealthDriveToConfig(cm, findSNByDriveLocation(drivesUnstructuredList.Items, location), nodeOfVolume)
		cm, err = f.ClientSet.CoreV1().ConfigMaps(ns).Update(cm)
		framework.ExpectNoError(err)

		// k8s docs say that time from updating ConfigMap to receiving updated ConfigMap in the pod where it's mounted
		// could last 1 minute by default
		time.Sleep(time.Minute)

		// Read Volume one more time
		changedVolume, err := f.DynamicClient.Resource(volumeGVR).Namespace(ns).Get(volumeName, metav1.GetOptions{})
		framework.ExpectNoError(err)
		health, _, err := unstructured.NestedString(changedVolume.Object, "spec", "Health")

		// Check that Volume is marked as unhealthy
		Expect(health).To(Equal(v1.HealthBad))
	})
}

// constructPVC constructs pvc for test purposes
// Receives PVC size and namespace
func constructPVC(ns string, claimSize string, storageClass string) *corev1.PersistentVolumeClaim {
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
			StorageClassName: &storageClass,
		},
	}

	return &claim
}

// appendBadHealthDriveToConfig appends spec of bad-health drive to LoopBackMgr's ConfigMap
// Receives current state of ConfigMap, serialNumber of drive to make it unhealthy, nodeID where the drive is placed
func appendBadHealthDriveToConfig(cm *corev1.ConfigMap, serialNumber string, nodeID string) {
	cm.Data[configKey] = cm.Data[configKey] + fmt.Sprintf("- nodeID: %s\n", nodeID) +
		"  drives:\n" +
		fmt.Sprintf("  - serialNumber: %s\n", serialNumber) +
		"    health: BAD\n"
}

// findSNByDriveLocation finds SerialNumber of the drive which is used by the volume using its location
// Receives unstructured list of drives and location
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
