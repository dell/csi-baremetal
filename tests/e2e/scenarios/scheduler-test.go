/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scenarios

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"

	"github.com/dell/csi-baremetal-e2e-tests/e2e/common"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

// DefineSchedulerTestSuite defines tests for scheduler extender
func DefineSchedulerTestSuite(driver *baremetalDriver) {
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
func schedulingTest(driver *baremetalDriver) {
	ginkgo.BeforeEach(skipIfNotAllTests)

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
			perTestConf *storageframework.PerTestConfig
			err         error
		)

		ns = f.Namespace.Name
		testPODs, testPVCs = nil, nil
		storageClasses = make(map[string]*storagev1.StorageClass)

		if lmConf != nil {
			lmConfigMap, err := common.BuildLoopBackManagerConfigMap(ns, cmName, *lmConf)
			framework.ExpectNoError(err)
			_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(context.TODO(), lmConfigMap, metav1.CreateOptions{})
		}

		perTestConf, driverCleanup = PrepareCSI(driver, f, false)

		for _, scName := range availableSC {
			sc := driver.GetStorageClassWithStorageType(perTestConf, scName)
			sc, err = f.ClientSet.StorageV1().StorageClasses().Create(context.TODO(), sc, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			storageClasses[scName] = sc
		}
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test SchedulingTests")
		common.CleanupAfterCustomTest(f, driverCleanup, testPODs, testPVCs)

		err := f.ClientSet.CoreV1().ConfigMaps(ns).Delete(context.TODO(), cmName, metav1.DeleteOptions{})
		if err != nil {
			e2elog.Logf("Configmap %s deletion failed: %v", cmName, err)
		}
	}

	createTestPod := func(podSCList []string) (*corev1.Pod, []*corev1.PersistentVolumeClaim) {
		var podPVCs []*corev1.PersistentVolumeClaim
		for _, scKey := range podSCList {
			sc := storageClasses[scKey]
			pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(
				context.TODO(),
				constructPVC(ns, driver.GetClaimSize(), sc.Name, pvcName+"-"+uuid.New().String()),
				metav1.CreateOptions{})
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
			Nodes: []common.LoopBackManagerConfigNode{
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
			Nodes:             lmNodes}
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
			Nodes: []common.LoopBackManagerConfigNode{
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
		nodes := getSchedulableNodesNamesOrSkipTest(f.ClientSet, 2)
		defaultDriveCount := 0
		node1, node2 := nodes[0], nodes[1]
		driveSize := "250Mi"
		lmConfig := &common.LoopBackManagerConfig{
			DefaultDriveCount: &defaultDriveCount,
			Nodes: []common.LoopBackManagerConfigNode{
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

	ginkgo.It("PODs should distribute across nodes with sequential deploy", func() {
		// TODO: change result verification https://github.com/dell/csi-baremetal/issues/153
		ginkgo.Skip("We shouldn't check prioritize work based on kube-scheduler decision, ISSUE-153")
		testPodsDisksPerPod := 1
		nodes := getSchedulableNodesNamesOrSkipTest(f.ClientSet, 0)
		testPodsCount := len(nodes)
		defaultDriveCount := 0

		var lmNodes []common.LoopBackManagerConfigNode
		for _, n := range nodes {
			nodeName := n
			// we need to make sure we have capacity for all pod in a single node
			nodeDriveCount := testPodsCount * testPodsDisksPerPod
			lmNodes = append(lmNodes,
				common.LoopBackManagerConfigNode{
					NodeID:     &nodeName,
					DriveCount: &nodeDriveCount})
		}
		lmConfig := &common.LoopBackManagerConfig{
			DefaultDriveCount: &defaultDriveCount,
			Nodes:             lmNodes}
		init(lmConfig)
		defer cleanup()

		// create pods sequential
		for p := 0; p < testPodsCount; p++ {
			createTestPod([]string{"ANY"})
		}
		volumes := getVolumesByNodes(f)
		e2elog.Logf("volumes by nodes %v", volumes)

		err := hostsNeedToHaveEqualNumberVolumes(volumes, 1)
		framework.ExpectNoError(err)
	})
}

func getVolumesByNodes(f *framework.Framework) map[string][]string {
	volumesUnstructuredList, err := f.DynamicClient.Resource(common.VolumeGVR).List(context.TODO(), metav1.ListOptions{})
	framework.ExpectNoError(err)
	volumes := make(map[string][]string)
	for _, targetVolume := range volumesUnstructuredList.Items {
		nodeUIDOfVolume, _, err := unstructured.NestedString(targetVolume.Object, "spec", "NodeId")
		framework.ExpectNoError(err)
		nodeNameOfVolume, err := findNodeNameByUID(f, nodeUIDOfVolume)
		framework.ExpectNoError(err)
		volId, _, err := unstructured.NestedString(targetVolume.Object, "spec", "Id")
		if _, ok := volumes[nodeNameOfVolume]; ok {
			volumes[nodeNameOfVolume] = append(volumes[nodeNameOfVolume], volId)
			continue
		}
		volumes[nodeNameOfVolume] = []string{volId}
	}
	return volumes
}

func hostsNeedToHaveEqualNumberVolumes(volumes map[string][]string, count int) error {
	problemHosts := make([]string, 0)
	for host, volumes := range volumes {
		if count != len(volumes) {
			problemHosts = append(problemHosts, host)
		}
	}
	if len(problemHosts) == 0 {
		return nil
	}
	return fmt.Errorf("hosts %v don't have expected number of volumes", problemHosts)
}

func buildLMDrivesConfig(node string, drives []common.LoopBackManagerConfigDevice) *common.LoopBackManagerConfigNode {
	drivesCount := len(drives)
	return &common.LoopBackManagerConfigNode{
		NodeID:     &node,
		DriveCount: &drivesCount,
		Drives:     drives,
	}
}

func getSchedulableNodesNamesOrSkipTest(client clientset.Interface, minNodeCount int) []string {
	result, err := getSchedulableNodesNames(client, minNodeCount)
	if err != nil {
		e2eskipper.Skipf("test's prerequisites not met: %s", err.Error())
	}
	return result
}

// getSchedulableNodesNames returns list of schedulable nodes
// will return error if schedulable nodes count < minNodeCount
// minNodeCount == 0 mean no limit
func getSchedulableNodesNames(client clientset.Interface, minNodeCount int) ([]string, error) {
	nodes, err := e2enode.GetReadySchedulableNodes(client)
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
