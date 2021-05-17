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
	"io/ioutil"
	"path"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"sigs.k8s.io/yaml"
)

const nodeName = "csi-baremetal-node"

// DefineLabeledDeployTestSuite defines label tests
func DefineLabeledDeployTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi labels are used", func() {
		labeledDeployTestSuite(driver)
	})
}

func labeledDeployTestSuite(driver testsuites.TestDriver) {
	var (
		chartsDir               = "/tmp"
		operatorManifestsFolder = "csi-baremetal-operator/templates"
		f                       = framework.NewDefaultFramework("node-label")
		label                   = "labeltag"
		tag                     = "csi"
	)

	ginkgo.It("CSI should use label on nodes", func() {
		file, err := ioutil.ReadFile(path.Join(chartsDir, operatorManifestsFolder, "csibm-controller.yaml"))
		if err != nil {
			ginkgo.Fail(err.Error())
		}

		deployment := &appsv1.Deployment{}
		err = yaml.Unmarshal(file, deployment)
		if err != nil {
			ginkgo.Fail(err.Error())
		}

		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--nodeselector=%s:%s", label, tag))

		_, err = f.ClientSet.AppsV1().Deployments("default").Update(deployment)
		if err != nil {
			ginkgo.Fail(err.Error())
		}

		err = e2epod.WaitForPodsRunningReady(f.ClientSet, f.Namespace.Name, 0, 0,
			1*time.Minute, nil)
		if err != nil {
			framework.Failf("Pods not ready, error: %s", err.Error())
		}

		perTestConf, driverCleanup := driver.PrepareTest(f)
		defer driverCleanup()

		k8sSC := driver.(*baremetalDriver).GetDynamicProvisionStorageClass(perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(k8sSC)
		framework.ExpectNoError(err)

		err = e2epod.WaitForPodsRunningReady(f.ClientSet, f.Namespace.Name, 0, 0,
			1*time.Minute, nil)
		if err != nil {
			framework.Failf("Pods not ready, error: %s", err.Error())
		}

		nodeDep, err := f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Get(nodeName, metav1.GetOptions{})
		Expect(nodeDep).ToNot(BeNil())

		if nodeDep.Spec.Template.Spec.NodeSelector == nil {
			nodeDep.Spec.Template.Spec.NodeSelector = map[string]string{label: tag}
		}

		_, err = f.ClientSet.AppsV1().DaemonSets(f.Namespace.Name).Update(nodeDep)

		// give some time to put nodes to termination state
		time.Sleep(10 * time.Second)

		err = e2epod.WaitForPodsRunningReady(f.ClientSet, f.Namespace.Name, 0, 0,
			1*time.Minute, nil)
		if err != nil {
			framework.Failf("Pods not ready, error: %s", err.Error())
		}
		np, err := getNodePodsNames(f)
		if err != nil {
			ginkgo.Fail(err.Error())
		}

		Expect(len(np)).To(Equal(0))
		nodes, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			ginkgo.Fail(err.Error())
		}
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
		Expect(len(np)).To(Not(Equal(0)))
	})

}
