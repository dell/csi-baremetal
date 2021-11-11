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
	"strconv"

	"github.com/google/uuid"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal/test/e2e/common"
)

const (
	RawPartModeKey   = "isPartitioned"
	RawPartModeValue = "true"
)

// DefineDifferentSCTestSuite defines different SCs tests
func DefineDifferentSCTestSuite(driver *baremetalDriver) {
	ginkgo.Context("Baremetal-csi driver different SCs tests", func() {
		// It consists of 3 suites with following.
		// 1) Create StorageClass with defined type
		// 2) Change ConfigMap data by overriding the value of driveType for all loopback devices for SSD SC test suite
		//(by default loopback devices have HDD driveType)
		// 3) Create Pod with 3 PVC
		differentSCTypesTest(driver)
	})
}

const (
	needPartitioned = "needPartitioned"
)

// differentSCTypesTest test work of different SCs of CSI driver
func differentSCTypesTest(driver *baremetalDriver) {
	ginkgo.BeforeEach(skipIfNotAllTests)

	var (
		testPod       *corev1.Pod
		pods          []*corev1.Pod
		pvcs          []*corev1.PersistentVolumeClaim
		k8sSC         *storagev1.StorageClass
		driverCleanup func()
		f             = framework.NewDefaultFramework("different-scs")
		ns            string
	)

	init := func(scType string, args ...string) {
		var (
			perTestConf *testsuites.PerTestConfig
			driverType  string
			// Allows to create SC with parameter for partitioned block mode
			isNeedBlockPartitioned = false
		)

		// Check extra args for CSI installation
		for _, arg := range args {
			if arg == needPartitioned {
				isNeedBlockPartitioned = true
			}
		}

		ns = f.Namespace.Name
		pods = make([]*corev1.Pod, 0)
		pvcs = make([]*corev1.PersistentVolumeClaim, 0)

		if scType == "SSD" {
			driverType = driveTypeSSD
		} else {
			driverType = driveTypeHDD
		}

		nodes, err := e2enode.GetReadySchedulableNodes(f.ClientSet)
		framework.ExpectNoError(err)

		configMap := constructLoopbackConfigWithDriveType(ns, nodes.Items, driverType)
		_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(context.TODO(), configMap, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		perTestConf, driverCleanup = PrepareCSI(driver, f, false)

		k8sSC = driver.GetStorageClassWithStorageType(perTestConf, scType)
		if isNeedBlockPartitioned {
			addRawBlockPartitionedParameter(k8sSC)
		}
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(context.TODO(), k8sSC, metav1.CreateOptions{})
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test DifferentScTest")
		common.CleanupAfterCustomTest(f, driverCleanup, pods, pvcs)

		err := f.ClientSet.CoreV1().ConfigMaps(ns).Delete(context.TODO(), cmName, metav1.DeleteOptions{})
		if err != nil {
			e2elog.Logf("Configmap %s deletion failed: %v", cmName, err)
		}
	}

	ginkgo.It("should create Pod with PVC with SSD type", func() {
		scType := "SSD"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.GetClaimSize(), k8sSC.Name, f.Namespace.Name)
		testPod = startAndWaitForPodWithPVCRunning(f, f.Namespace.Name, pvcs)
		if testPod != nil {
			pods = append(pods, testPod)
		}
	})

	ginkgo.It("should create Pod with PVC with ANY type", func() {
		scType := "ANY"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.GetClaimSize(), k8sSC.Name, ns)
		testPod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
		if testPod != nil {
			pods = append(pods, testPod)
		}
	})

	ginkgo.It("should create Pod with PVC with HDD type", func() {
		scType := "HDD"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.GetClaimSize(), k8sSC.Name, ns)
		testPod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
		if testPod != nil {
			pods = append(pods, testPod)
		}
	})

	// test for logical volume group storage class
	ginkgo.It("should create Pod with PVC with HDDLVG type", func() {
		scType := "HDDLVG"
		init(scType)
		defer cleanup()
		pvcs = createPVCs(f, 3, driver.GetClaimSize(), k8sSC.Name, ns)
		testPod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
		if testPod != nil {
			pods = append(pods, testPod)
		}
	})
	// test for raw block volumes
	ginkgo.It("should create Pod with raw block volume HDD", func() {
		scType := "HDD"
		init(scType)
		defer cleanup()
		pvcs = []*corev1.PersistentVolumeClaim{createBlockPVC(
			f, 1, driver.GetClaimSize(), k8sSC.Name, ns)}
		testPod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
		if testPod != nil {
			pods = append(pods, testPod)
		}
	})

	ginkgo.It("should create Pod with raw block volume HDDLVG", func() {
		scType := "HDDLVG"
		init(scType)
		defer cleanup()
		pvcs = []*corev1.PersistentVolumeClaim{createBlockPVC(
			f, 1, driver.GetClaimSize(), k8sSC.Name, ns)}
		testPod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
		if testPod != nil {
			pods = append(pods, testPod)
		}
	})

	ginkgo.It("should create Pod with partitioned raw block volume HDD", func() {
		scType := "HDD"
		init(scType, needPartitioned)
		defer cleanup()
		pvcs = []*corev1.PersistentVolumeClaim{createBlockPVC(
			f, 1, driver.GetClaimSize(), k8sSC.Name, ns)}
		testPod = startAndWaitForPodWithPVCRunning(f, ns, pvcs)
		if testPod != nil {
			pods = append(pods, testPod)
		}
	})
}

func createBlockPVC(f *framework.Framework, numberOfPVC int, size string, scName string, ns string) *corev1.PersistentVolumeClaim {
	pvc := constructPVC(
		ns,
		size,
		scName,
		pvcName+"-"+uuid.New().String())
	blockMode := corev1.PersistentVolumeBlock
	pvc.Spec.VolumeMode = &blockMode
	pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(), pvc, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	return pvc
}

// CreatePVCs create PVCs in Kubernetes
// Params: E2E test framework, numberOfPVC to create, size of PVC, name of PVC storageClass, PVC namespace
// Returns: slice of created PVCs
func createPVCs(f *framework.Framework, numberOfPVC int, size string, scName string, ns string) []*corev1.PersistentVolumeClaim {
	var pvcs []*corev1.PersistentVolumeClaim
	for i := 0; i < numberOfPVC; i++ {
		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(
			context.TODO(),
			constructPVC(ns, size, scName, pvcName+"-"+strconv.Itoa(i)),
			metav1.CreateOptions{})
		framework.ExpectNoError(err)
		pvcs = append(pvcs, pvc)
	}
	return pvcs
}

// startAndWaitForPodWithPVCRunning launch test Pod with PVC and wait until it has Running state
// Params: E2E test framework, Pod namespace, slice of PVC for Pod
// Returns: created Pod
func startAndWaitForPodWithPVCRunning(f *framework.Framework, ns string, pvc []*corev1.PersistentVolumeClaim) *corev1.Pod {
	// Create test pod that consumes the pvc
	pod, err := common.CreatePod(f.ClientSet, ns, nil, pvc, false, "sleep 3600")
	framework.ExpectNoError(err)
	return pod
}

// constructLoopbackConfigWithSSDDevices constructs ConfigMap with 3 drive with given driveType for LoopBackManager
// Receives namespace where cm should be deployed, nodes names, driveType
// Returns ConfigMap
func constructLoopbackConfigWithDriveType(namespace string, nodes []corev1.Node, driveType string) *corev1.ConfigMap {
	var nodeConfig string
	for _, node := range nodes {
		nodeConfig += fmt.Sprintf("- nodeID: %s\n", node.Name) +
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

func addRawBlockPartitionedParameter(sc *storagev1.StorageClass) {
	sc.Parameters[RawPartModeKey] = RawPartModeValue
}
