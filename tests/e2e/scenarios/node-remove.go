/*
Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

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

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
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
		ctx           context.Context
		ns            string
		taintNodeName string
		f             = framework.NewDefaultFramework("node-remove-test")
	)

	init := func() {
		var (
			perTestConf *storageframework.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name
		ctx = context.Background()

		perTestConf = driver.PrepareTest(ctx, f)
		k8sSC = driver.GetDynamicProvisionStorageClass(ctx, perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, k8sSC, metav1.CreateOptions{})
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		framework.Logf("Starting cleanup for test NodeRemoval")

		if taintNodeName != "" {
			podsBefore, err := e2epod.GetPodsInNamespace(ctx, f.ClientSet, f.Namespace.Name, map[string]string{})
			framework.ExpectNoError(err)

			_, _, err = executor.RunCmd(fmt.Sprintf("docker stop %s", taintNodeName))
			framework.ExpectNoError(err)
			_, _, err = executor.RunCmd(fmt.Sprintf("docker start %s", taintNodeName))
			framework.ExpectNoError(err)

			if !e2enode.WaitForNodeToBeReady(ctx, f.ClientSet, taintNodeName, time.Minute*5) {
				framework.Failf("Node %s is not ready", taintNodeName)
			}

			pods, err := e2epod.GetPodsInNamespace(ctx, f.ClientSet, f.Namespace.Name, map[string]string{})
			framework.ExpectNoError(err)

			framework.Logf("Count of pods before test was %d, after - %d", len(podsBefore), len(pods))
			if len(pods)-len(podsBefore) <= 0 {
				framework.Failf("Csi-baremetal-node not ready")
			}
		}
		common.CleanupAfterCustomTest(ctx, f, nil, []*corev1.Pod{pod}, []*corev1.PersistentVolumeClaim{pvc})
	}

	ginkgo.It("CSI node resources should be deleted after node removal", func(ctx context.Context) {
		init()
		ginkgo.DeferCleanup(cleanup)

		var err error
		// create pvc
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(ctx,
			constructPVC(ns, driver.GetClaimSize(), k8sSC.Name, pvcName),
			metav1.CreateOptions{})
		framework.ExpectNoError(err)

		// create pod with pvc
		pod, err = common.CreatePod(ctx, f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		framework.Logf("Pod %s with PVC %s created.", pod.Name, pvc.Name)

		taint := corev1.Taint{
			Key:    "node.dell.com/drain",
			Value:  "drain",
			Effect: "NoSchedule",
		}

		taintNodeName = pod.Spec.NodeName
		taintedNodeId, err := foundCsibmnodeByNodeName(f, taintNodeName)
		framework.ExpectNoError(err)

		// taint node
		cmd := fmt.Sprintf("kubectl taint node %s %s=%s:%s", taintNodeName, taint.Key, taint.Value, taint.Effect)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		// check taint
		_, err = e2enode.NodeHasTaint(ctx, f.ClientSet, taintNodeName, &taint)
		framework.ExpectNoError(err)

		// wait until csibmnode labeled with node.dell.com/drain=drain
		for start := time.Now(); time.Since(start) < time.Minute*10; time.Sleep(time.Second * 10) {
			if csibmnodeHasLabel(f, taintedNodeId, &taint) {
				break
			}
		}
		framework.Logf("csibmnode %s labeled with %s=%s", taintedNodeId, taint.Key, taint.Value)

		// delete node
		cmd = fmt.Sprintf("kubectl delete node %s", taintNodeName)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		framework.Logf("Waiting for csibmnode to be deleted...")
		for start := time.Now(); time.Since(start) < time.Minute*10; time.Sleep(time.Second * 30) {
			if !isNodeExist(f, taintedNodeId) {
				break
			}
		}
		_, _, err = executor.RunCmd("kubectl get drive")
		framework.ExpectNoError(err)

		_, _, err = executor.RunCmd("kubectl get ac")
		framework.ExpectNoError(err)

		// time end or deleted
		gomega.Expect(isNodeExist(f, taintedNodeId)).To(gomega.BeFalse())
		gomega.Expect(isRecourseExistOnNode(f, common.DriveGVR, taintedNodeId)).To(gomega.BeFalse())
		gomega.Expect(isRecourseExistOnNode(f, common.ACGVR, taintedNodeId)).To(gomega.BeFalse())
		gomega.Expect(isRecourseExistOnNode(f, common.VolumeGVR, taintedNodeId)).To(gomega.BeFalse())
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
			framework.Logf("Node %s has nodeID %s", taintedNodeName, nodeUUID)
			taintedCsibmnode = nodeUUID
			break
		}
	}
	return taintedCsibmnode, nil
}

func csibmnodeHasLabel(f *framework.Framework, nodeID string, taint *corev1.Taint) bool {
	allNodes := getUObjList(f, common.CsibmnodeGVR)

	for _, node := range allNodes.Items {
		nodeUUID, _, err := unstructured.NestedString(node.UnstructuredContent(), "spec", "UUID")
		framework.ExpectNoError(err)
		if nodeUUID == nodeID {
			labels, _, error := unstructured.NestedStringMap(node.UnstructuredContent(), "metadata", "labels")
			framework.ExpectNoError(error)
			value, ok := labels[taint.Key]
			return ok && value == taint.Value
		}
	}
	return false
}

func isNodeExist(f *framework.Framework, nodeID string) bool {
	allNodes := getUObjList(f, common.CsibmnodeGVR)

	for _, node := range allNodes.Items {
		nodeUUID, _, err := unstructured.NestedString(node.UnstructuredContent(), "spec", "UUID")
		framework.ExpectNoError(err)
		if nodeUUID == nodeID {
			framework.Logf("Node %s exist", nodeID)
			return true
		}
	}
	return false
}

func isRecourseExistOnNode(f *framework.Framework, resource schema.GroupVersionResource, nodeID string) bool {
	list := getUObjList(f, resource)
	for _, el := range list.Items {
		specNodeID, _, err := unstructured.NestedString(el.UnstructuredContent(), "spec", "NodeId")
		framework.ExpectNoError(err)
		if specNodeID == nodeID {
			framework.Logf("On taintedNode %s exist %s", nodeID, resource)
			return true
		}
	}
	return false
}
