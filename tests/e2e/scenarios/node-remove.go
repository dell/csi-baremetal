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
	"time"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"

	"github.com/dell/csi-baremetal-e2e-tests/e2e/common"
)

// DefineNodeRemovalTestSuite defines custom csi-baremetal node removal test
func DefineNodeRemovalTestSuite(driver *baremetalDriver) {
	ginkgo.Context("Baremetal-csi node remove test", func() {
		defineNodeRemovalTest(driver)
	})
}

func defineNodeRemovalTest(driver *baremetalDriver) {
	ginkgo.BeforeEach(skipIfNotAllTests)

	var (
		pod           *corev1.Pod
		pvc           *corev1.PersistentVolumeClaim
		k8sSC         *storagev1.StorageClass
		executor      = common.GetExecutor()
		driverCleanup func()
		ns            string
		taintNodeName string
		ctx           context.Context
		f             = framework.NewDefaultFramework("node-remove-test")
	)

	init := func() {
		var (
			perTestConf *storageframework.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name
		ctx = context.Background()
		perTestConf, driverCleanup = driver.PrepareTest(f)
		k8sSC = driver.GetDynamicProvisionStorageClass(perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(context.TODO(), k8sSC, metav1.CreateOptions{})
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test NodeRemoval")

		if taintNodeName != "" {
			_, _, err := executor.RunCmd(fmt.Sprintf("docker start %s", taintNodeName))
			framework.ExpectNoError(err)
		}
		common.CleanupAfterCustomTest(f, driverCleanup, []*corev1.Pod{pod}, []*corev1.PersistentVolumeClaim{pvc})
	}

	ginkgo.It("CSI node resources should be deleted after node removal", func() {
		init()
		defer cleanup()

		var err error
		// create pvc
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(),
			constructPVC(ns, driver.GetClaimSize(), k8sSC.Name, pvcName),
			metav1.CreateOptions{})
		framework.ExpectNoError(err)

		// create pod with pvc
		pod, err = common.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		e2elog.Logf("Pod %s with PVC %s created.", pod.Name, pvc.Name)

		taint := corev1.Taint{
			Key:    "node.dell.com/drain",
			Value:  "drain:NoSchedule",
			Effect: "NoSchedule",
		}

		taintNodeName = pod.Spec.NodeName
		taintedNodeId, err := foundCsibmnodeByNodeName(f, taintNodeName)
		framework.ExpectNoError(err)

		// taint node
		cmd := fmt.Sprintf("kubectl taint node %s %s=%s", taintNodeName, taint.Key, taint.Value)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		// check taint
		_, err = framework.NodeHasTaint(f.ClientSet, taintNodeName, &taint)
		framework.ExpectNoError(err)

		// delete node
		cmd = fmt.Sprintf("kubectl delete node %s", taintNodeName)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		e2elog.Logf("Waiting for all drives to be removed from node...")
		for start := time.Now(); time.Since(start) < time.Minute*10; time.Sleep(time.Second * 30) {
			if !isDriveExist(f, taintedNodeId) {
				break
			}
		}
		_, _, err = executor.RunCmd("kubectl get drive")
		framework.ExpectNoError(err)

		_, _, err = executor.RunCmd("kubectl get ac")
		framework.ExpectNoError(err)

		// time end or deleted
		framework.ExpectEqual(isDriveExist(f, taintedNodeId), false)
		framework.ExpectEqual(isACExist(f, taintedNodeId), false)
		framework.ExpectEqual(isNodeExist(f, taintedNodeId), false)
	})
}

func foundCsibmnodeByNodeName(f *framework.Framework, nodeName string) (string, error) {
	allNodes := getUObjList(f, common.CsibmnodeGVR)
	var taintedCsibmnode string

	for _, node := range allNodes.Items {
		nodeUUID, _, err := unstructured.NestedString(node.UnstructuredContent(), "spec", "UUID")
		if err != nil {
			return "", err
		}
		taintedNodeName, _, err := unstructured.NestedString(
			node.UnstructuredContent(), "spec", "Addresses", "Hostname")
		if err != nil {
			return "", err
		}
		if taintedNodeName == nodeName {
			e2elog.Logf("Node %s has nodeID %s", taintedNodeName, nodeUUID)
			taintedCsibmnode = nodeUUID
			break
		}
	}
	return taintedCsibmnode, nil
}

func isDriveExist(f *framework.Framework, nodeID string) bool {
	allDrives := getUObjList(f, common.DriveGVR)
	exist := false

	for _, drive := range allDrives.Items {
		specNodeID, _, err := unstructured.NestedString(drive.UnstructuredContent(), "spec", "NodeId")
		framework.ExpectNoError(err)
		if specNodeID == nodeID {
			e2elog.Logf("On taintedNode %s exist drive %s", nodeID, drive)
			exist = true
			break
		}
	}
	return exist
}

func isNodeExist(f *framework.Framework, nodeID string) bool {
	allNodes := getUObjList(f, common.CsibmnodeGVR)
	exist := false

	for _, node := range allNodes.Items {
		nodeUUID, _, err := unstructured.NestedString(node.UnstructuredContent(), "spec", "UUID")
		framework.ExpectNoError(err)
		if nodeUUID == nodeID {
			e2elog.Logf("Node %s exist", nodeID)
			exist = true
			break
		}
	}
	return exist
}

func isACExist(f *framework.Framework, nodeID string) bool {
	allACs := getUObjList(f, common.ACGVR)
	exist := false

	for _, ac := range allACs.Items {
		nodeUUID, _, err := unstructured.NestedString(ac.UnstructuredContent(), "spec", "NodeId")
		framework.ExpectNoError(err)
		if nodeUUID == nodeID {
			e2elog.Logf("AC %s on node %s exist", ac, nodeID)
			exist = true
			break
		}
	}
	return exist
}

