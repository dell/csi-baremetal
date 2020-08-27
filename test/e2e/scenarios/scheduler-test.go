package scenarios

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/test/e2e/common"
)

// DefineSchedulerTestSuite defines tests for scheduler extender
func DefineSchedulerTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi driver scheduling tests", func() {
		schedulingTest(driver)
	})
}

var (
	storageClassAny    = apiV1.StorageClassAny
	storageClassHDD    = apiV1.StorageClassHDD
	storageClassSSD    = apiV1.StorageClassSSD
	storageClassNVMe   = apiV1.StorageClassNVMe
	storageClassHDDLVG = apiV1.StorageClassHDDLVG
	storageClassSSDLVG = apiV1.StorageClassSSDLVG

	driveTypeHDD  = apiV1.DriveTypeHDD
	driveTypeSSD  = apiV1.DriveTypeSSD
	driveTypeNVMe = apiV1.DriveTypeNVMe
)

// schedulingTest test custom extender for scheduler
func schedulingTest(driver testsuites.TestDriver) {
	var (
		testPODs      []*corev1.Pod
		testPVCs      []*corev1.PersistentVolumeClaim
		updateM       sync.Mutex
		driverCleanup func()
		ns            string
		f             = framework.NewDefaultFramework("scheduling-test")
		availableSC   = []string{storageClassAny, storageClassHDD, storageClassSSD,
			storageClassNVMe, storageClassHDDLVG, storageClassSSDLVG}
		storageClasses = make(map[string]*storagev1.StorageClass)
	)

	init := func(lmConf *common.LoopBackManagerConfig) {
		var (
			perTestConf *testsuites.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name

		if lmConf != nil {
			lmConfigMap, err := common.BuildLoopBackManagerConfigMap(ns, cmName, *lmConf)
			framework.ExpectNoError(err)
			_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(lmConfigMap)
		}

		perTestConf, driverCleanup = driver.PrepareTest(f)

		for _, scName := range availableSC {
			sc := driver.(*baremetalDriver).GetStorageClassWithStorageType(perTestConf, scName)
			sc, err = f.ClientSet.StorageV1().StorageClasses().Create(sc)
			framework.ExpectNoError(err)
			storageClasses[scName] = sc
		}

		// wait for csi pods to be running and ready
		err = e2epod.WaitForPodsRunningReady(f.ClientSet, ns,
			2, 0, 90*time.Second, nil)
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test SchedulingTests")
		common.CleanupAfterCustomTest(f, driverCleanup, testPODs, testPVCs)
		testPODs, testPVCs = nil, nil
		storageClasses = make(map[string]*storagev1.StorageClass)
	}

	createTestPod := func(podSCList []string) (*corev1.Pod, []*corev1.PersistentVolumeClaim) {
		var podPVCs []*corev1.PersistentVolumeClaim
		for _, scKey := range podSCList {
			sc := storageClasses[scKey]
			pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(
				constructPVC(ns, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(),
					sc.Name, pvcName+"-"+uuid.New().String()))
			framework.ExpectNoError(err)
			podPVCs = append(podPVCs, pvc)
		}
		pod := startAndWaitForPodWithPVCRunning(f, ns, podPVCs)
		updateM.Lock()
		testPODs = append(testPODs, pod)
		testPVCs = append(testPVCs, podPVCs...)
		updateM.Unlock()
		return pod, podPVCs
	}

	createTestPods := func(testPodsCount int, testPodsDisksPerPod int) {
		wg := sync.WaitGroup{}
		var podSCList []string
		for i := 0; i < testPodsDisksPerPod; i++ {
			podSCList = append(podSCList, "ANY")
		}
		for i := 0; i < testPodsCount; i++ {
			wg.Add(1)
			go func() {
				defer ginkgo.GinkgoRecover()
				defer wg.Done()
				_, _ = createTestPod(podSCList)
			}()
		}
		wg.Wait()
	}

	ginkgo.It("One node has all capacity", func() {
		testPodsCount := 3
		testPodsDisksPerPod := 2

		nodes := getSchedulableNodesNamesOrSkipTest(f.ClientSet, 2)
		nodeWithDisksID := nodes[0]
		nodeWithDisksDriveCount := testPodsCount * testPodsDisksPerPod
		defaultDriveCount := 0
		lmConfig := &common.LoopBackManagerConfig{
			DefaultDriveCount: &defaultDriveCount,
			Nodes: &[]common.LoopBackManagerConfigNode{
				{
					NodeID:     &nodeWithDisksID,
					DriveCount: &nodeWithDisksDriveCount},
			},
		}
		init(lmConfig)
		defer cleanup()
		createTestPods(testPodsCount, testPodsDisksPerPod)
	})

	ginkgo.It("PODs should distribute across nodes", func() {
		framework.Skipf("skip test. See ATLDEF-81 for details")
		testPodsCount := 3
		testPodsDisksPerPod := 3
		nodes := getSchedulableNodesNamesOrSkipTest(f.ClientSet, testPodsCount)

		defaultDriveCount := 0
		var lmNodes []common.LoopBackManagerConfigNode
		for _, n := range nodes[:testPodsCount] {
			nodeName := n
			nodeDriveCount := testPodsDisksPerPod
			lmNodes = append(lmNodes,
				common.LoopBackManagerConfigNode{
					NodeID:     &nodeName,
					DriveCount: &nodeDriveCount})
		}
		lmConfig := &common.LoopBackManagerConfig{
			DefaultDriveCount: &defaultDriveCount,
			Nodes:             &lmNodes}
		init(lmConfig)
		defer cleanup()
		createTestPods(testPodsCount, testPodsDisksPerPod)
	})

	ginkgo.It("Scheduler should respect SC", func() {
		nodes := getSchedulableNodesNamesOrSkipTest(f.ClientSet, 3)

		node1, node2, node3 := nodes[0], nodes[1], nodes[2]

		defaultDriveCount := 0
		lmConfig := &common.LoopBackManagerConfig{
			DefaultDriveCount: &defaultDriveCount,
			Nodes: &[]common.LoopBackManagerConfigNode{
				*buildLMDrivesConfig(node1, []common.LoopBackManagerConfigDevice{
					{DriveType: &driveTypeHDD}, {DriveType: &driveTypeSSD}}),
				*buildLMDrivesConfig(node2, []common.LoopBackManagerConfigDevice{
					{DriveType: &driveTypeHDD}, {DriveType: &driveTypeNVMe}, {DriveType: &driveTypeHDD}}),
				*buildLMDrivesConfig(node3, []common.LoopBackManagerConfigDevice{
					{DriveType: &driveTypeHDD}, {DriveType: &driveTypeHDD}}),
			}}
		init(lmConfig)
		defer cleanup()

		createTestPod([]string{storageClassHDD, storageClassSSD})
		createTestPod([]string{storageClassHDD, storageClassNVMe})
		createTestPod([]string{storageClassHDD, storageClassHDD})
		createTestPod([]string{storageClassAny})
	})

	ginkgo.It("2 LVM PV on one drive", func() {
		framework.Skipf("skip test. See ATLDEF-83 for details")
		nodes := getSchedulableNodesNamesOrSkipTest(f.ClientSet, 2)
		defaultDriveCount := 0
		node1, node2 := nodes[0], nodes[1]
		driveSize := "250Mi"
		lmConfig := &common.LoopBackManagerConfig{
			DefaultDriveCount: &defaultDriveCount,
			Nodes: &[]common.LoopBackManagerConfigNode{
				*buildLMDrivesConfig(node1, []common.LoopBackManagerConfigDevice{
					{DriveType: &driveTypeHDD, Size: &driveSize}}),
				*buildLMDrivesConfig(node2, []common.LoopBackManagerConfigDevice{
					{DriveType: &driveTypeSSD, Size: &driveSize}}),
			}}
		init(lmConfig)
		defer cleanup()

		createTestPod([]string{storageClassHDDLVG, storageClassHDDLVG})
		createTestPod([]string{storageClassSSDLVG, storageClassSSDLVG})
	})

}

func buildLMDrivesConfig(node string, drives []common.LoopBackManagerConfigDevice) *common.LoopBackManagerConfigNode {
	drivesCount := len(drives)
	return &common.LoopBackManagerConfigNode{
		NodeID:     &node,
		DriveCount: &drivesCount,
		Drives:     &drives,
	}
}

func getSchedulableNodesNamesOrSkipTest(client clientset.Interface, minNodeCount int) []string {
	result, err := getSchedulableNodesNames(client, minNodeCount)
	if err != nil {
		framework.Skipf("test's prerequisites not met: %s", err.Error())
	}
	return result
}

// getSchedulableNodesNames returns list of schedulable nodes
// will return error if schedulable nodes count < minNodeCount
// minNodeCount == 0 mean no limit
func getSchedulableNodesNames(client clientset.Interface, minNodeCount int) ([]string, error) {
	nodes, err := e2enode.GetReadySchedulableNodesOrDie(client)
	framework.ExpectNoError(err)
	var nodeNames []string
	for _, item := range nodes.Items {
		nodeNames = append(nodeNames, item.Name)
	}
	if minNodeCount > 0 && minNodeCount > len(nodeNames) {
		return nil, fmt.Errorf("not enough schedulabel nodes, required: %d, found: %d",
			minNodeCount, len(nodeNames))
	}
	return nodeNames, nil
}
