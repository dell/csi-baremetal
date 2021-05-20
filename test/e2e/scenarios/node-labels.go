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
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/dell/csi-baremetal/test/e2e/common"
)

const (
	masterNodeLabel = "node-role.kubernetes.io/master"
	label           = "labeltag"
	tag             = "csi"
)

// DefineLabeledDeployTestSuite defines label tests
func DefineLabeledDeployTestSuite() {
	ginkgo.Context("Baremetal-csi labels are used", func() {
		labeledDeployTestSuite()
	})
}

func labeledDeployTestSuite() {
	var (
		f                  = framework.NewDefaultFramework("node-label")
		setNodeSelectorArg = fmt.Sprintf(" --set nodeSelector.key=%s --set nodeSelector.value=%s", label, tag)
	)

	ginkgo.It("CSI should use label on nodes", func() {
		defer cleanNodeLabels(f.ClientSet)

		nodes := getWorkerNodes(f.ClientSet)
		nodes[0].Labels[label] = tag
		if _, err := f.ClientSet.CoreV1().Nodes().Update(&nodes[0]); err != nil {
			ginkgo.Fail(err.Error())
		}

		driverCleanup, err := common.DeployCSI(f, setNodeSelectorArg)
		defer driverCleanup()

		framework.ExpectNoError(err)

		np, err := common.GetNodePodsNames(f)
		if err != nil {
			ginkgo.Fail(err.Error())
		}
		Expect(len(np)).To(Equal(1))

		nodes = getWorkerNodes(f.ClientSet)
		for _, node := range nodes {
			node.Labels[label] = tag
			if _, err := f.ClientSet.CoreV1().Nodes().Update(&node); err != nil {
				ginkgo.Fail(err.Error())
			}
		}

		// wait till operator reconcile csi
		// operator has to receive NodeUpdate request and label new nodes to expand node-daemonset
		// if new pods aren't created in time, WaitForPodsRunningReady will be skipped
		deadline := time.Now().Add(time.Second * 30)
		for {
			np, err = common.GetNodePodsNames(f)
			if err != nil {
				ginkgo.Fail(err.Error())
			}
			if len(np) == len(nodes) {
				break
			}
			if time.Now().After(deadline) {
				ginkgo.Fail(fmt.Sprintf("Pods number is %d, should be %d", len(np), len(nodes)))
			}
			time.Sleep(time.Second * 3)
		}

		err = e2epod.WaitForPodsRunningReady(f.ClientSet, f.Namespace.Name, 0, 0,
			3*time.Minute, nil)
	})
}

func getWorkerNodes(c clientset.Interface) []corev1.Node {
	var workerNodes []corev1.Node

	nodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		ginkgo.Fail(err.Error())
	}

	for _, node := range nodes.Items {
		if _, ok := node.Labels[masterNodeLabel]; !ok {
			workerNodes = append(workerNodes, node)
		}
	}

	return workerNodes
}

func cleanNodeLabels(c clientset.Interface) {
	nodes := getWorkerNodes(c)
	for _, node := range nodes {
		if _, ok := node.Labels[label]; ok {
			delete(node.Labels, label)
			if _, err := c.CoreV1().Nodes().Update(&node); err != nil {
				e2elog.Logf("Error updating node: %s", err)
			}
		}
	}
}
