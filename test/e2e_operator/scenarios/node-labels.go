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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/dell/csi-baremetal/test/e2e_operator/common"
)

const nodeName = "csi-baremetal-node"

// DefineLabeledDeployTestSuite defines label tests
func DefineLabeledDeployTestSuite() {
	ginkgo.Context("Baremetal-csi labels are used", func() {
		labeledDeployTestSuite()
	})
}

func labeledDeployTestSuite() {
	var (
		f                  = framework.NewDefaultFramework("node-label")
		label              = "labeltag"
		tag                = "csi"
		setNodeSelectorArg = fmt.Sprintf(" --set nodeSelector.key=%s --set nodeSelector.value=%s", label, tag)
	)

	ginkgo.It("CSI should use label on nodes", func() {
		nodes, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			ginkgo.Fail(err.Error())
		}
		node := nodes.Items[1]
		node.Labels[label] = tag
		if _, err := f.ClientSet.CoreV1().Nodes().Update(&node); err != nil {
			ginkgo.Fail(err.Error())
		}

		driverCleanup, err := common.DeployCSIWithArgs(f, setNodeSelectorArg)
		defer driverCleanup()

		framework.ExpectNoError(err)

		np, err := getNodePodsNames(f)
		if err != nil {
			ginkgo.Fail(err.Error())
		}
		Expect(len(np)).To(Equal(2))

		for _, node := range nodes.Items {
			node.Labels[label] = tag
			if _, err := f.ClientSet.CoreV1().Nodes().Update(&node); err != nil {
				ginkgo.Fail(err.Error())
			}
		}

		err = e2epod.WaitForPodsRunningReady(f.ClientSet, f.Namespace.Name, 0, 0,
			3*time.Minute, nil)

		np, err = getNodePodsNames(f)
		if err != nil {
			ginkgo.Fail(err.Error())
		}
		e2elog.Logf("nodePODS\n%+v\n", np)
		Expect(len(np)).To(Equal(7))
	})
}
