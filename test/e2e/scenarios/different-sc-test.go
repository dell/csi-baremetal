package scenarios

import (
	"fmt"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal.git/test/e2e/common"
)

// DefineDifferentSCTestSuite defines different SCs tests
func DefineDifferentSCTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi driver different SCs tests", func() {
		// It consists of 3 suites with following.
		// 1) Create StorageClass with defined type
		// 2) Change ConfigMap data by overriding the value of driveType for all loopback devices for SSD SC test suite
		//(by default loopback devices have HDD driveType)
		// 3) Create Pod with 3 PVC
		differentSCTypesTest(driver)
	})
}

// differentSCTypesTest test work of different SCs of CSI driver
func differentSCTypesTest(driver testsuites.TestDriver) {
	var (
		pod           *corev1.Pod
		pvcs          []*corev1.PersistentVolumeClaim
		k8sSC         *storagev1.StorageClass
		driverCleanup func()
		ns            string
		f             = framework.NewDefaultFramework("different-scs")
	)

	init := func(scType string) {
		var (
			perTestConf *testsuites.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name
		if scType == "SSD" {
			nodes, err := e2enode.GetReadySchedulableNodesOrDie(f.ClientSet)
			framework.ExpectNoError(err)
			var nodeNames []string
			for _, item := range nodes.Items {
				nodeNames = append(nodeNames, item.Name)
			}
			_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(constructLoopbackConfigWithDriveType(ns, nodeNames, scType))
			framework.ExpectNoError(err)
		}
		perTestConf, driverCleanup = driver.PrepareTest(f)

		k8sSC = driver.(*baremetalDriver).GetStorageClassWithStorageType(perTestConf, scType)
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(k8sSC)
		framework.ExpectNoError(err)

		// wait for csi pods to be running and ready
		err = e2epod.WaitForPodsRunningReady(f.ClientSet, ns, 2, 0, 90*time.Second, nil)
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test DriveHealthChange")
		common.CleanupAfterCustomTest(f, driverCleanup, pod, pvcs)
	}

	ginkgo.It("should create Pod with PVC with SSD type", func() {
		scType := "SSD"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), k8sSC.Name, ns)
		pod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
	})

	ginkgo.It("should create Pod with PVC with ANY type", func() {
		scType := "ANY"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), k8sSC.Name, ns)
		pod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
	})

	ginkgo.It("should create Pod with PVC with HDD type", func() {
		scType := "HDD"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), k8sSC.Name, ns)
		pod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
	})

	// test for logical volume group storage class
	ginkgo.It("should create Pod with PVC with HDDLVG type", func() {
		scType := "HDDLVG"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), k8sSC.Name, ns)
		pod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
	})
}

//createPVCs create PVCs in Kubernetes
//Params: E2E test framework, numberOfPVC to create, size of PVC, name of PVC storageClass, PVC namespace
//Returns: slice of created PVCs
func createPVCs(f *framework.Framework, numberOfPVC int, size string, scName string, ns string) []*corev1.PersistentVolumeClaim {
	var pvcs []*corev1.PersistentVolumeClaim
	for i := 0; i < numberOfPVC; i++ {
		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(constructPVC(ns, size, scName, pvcName+"-"+strconv.Itoa(i)))
		framework.ExpectNoError(err)
		pvcs = append(pvcs, pvc)
	}
	return pvcs
}

//startAndWaitForPodWithPVCRunning launch test Pod with PVC and wait until it has Running state
//Params: E2E test framework, Pod namespace, slice of PVC for Pod
//Returns: created Pod
func startAndWaitForPodWithPVCRunning(f *framework.Framework, ns string, pvc []*corev1.PersistentVolumeClaim) *corev1.Pod {
	// Create test pod that consumes the pvc
	pod, err := e2epod.CreatePod(f.ClientSet, ns, nil, pvc, false, "sleep 3600")
	framework.ExpectNoError(err)

	err = f.WaitForPodRunning(pod.Name)
	framework.ExpectNoError(err)
	return pod
}

// constructLoopbackConfigWithSSDDevices constructs ConfigMap with 3 drive with given driveType for LoopBackManager
// Receives namespace where cm should be deployed, nodes names, driveType
// Returns ConfigMap
func constructLoopbackConfigWithDriveType(namespace string, nodes []string, driveType string) *corev1.ConfigMap {
	var nodeConfig string
	for _, node := range nodes {
		nodeConfig += fmt.Sprintf("- nodeID: %s\n", node) +
			"  drives:\n"
		for _, sn := range []string{"LOOPBACK1", "LOOPBACK2", "LOOPBACK3"} {
			nodeConfig += fmt.Sprintf("  - serialNumber: %s\n", sn) +
				fmt.Sprintf("    driveType: %s\n", driveType)
		}
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"config.yaml": "\n" +
				"defaultDrivePerNodeCount: 3\n" +
				"nodes:\n" +
				nodeConfig,
		},
	}
	return &cm
}
