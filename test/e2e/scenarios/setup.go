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

/*test package includes baremetal test storage class definition for e2e tests
  and definition of e2e test suites with ginkgo library
  main file for e2e tests is in cmd/tests directory
  we can run defined test suites with following command:
  go test cmd/tests/baremetal_e2e.go -ginkgo.v -ginkgo.progress --kubeconfig=<kubeconfig>
*/
package scenarios

import (
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"path"

	"github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"
	"sigs.k8s.io/yaml"

	"github.com/dell/csi-baremetal/test/e2e/common"
)

var CSITestSuites = []func() testsuites.TestSuite{
	testsuites.InitVolumesTestSuite,
	testsuites.InitVolumeIOTestSuite,
	testsuites.InitEphemeralTestSuite,
	testsuites.InitProvisioningTestSuite,
	testsuites.InitMultiVolumeTestSuite,
	testsuites.InitVolumeModeTestSuite,
}

var _ = utils.SIGDescribe("CSI Volumes", func() {
	logrus.Infof("RepoRoot: %s", common.BMDriverTestContext.RepoRoot)

	pathToTheManifests := path.Join(
		common.BMDriverTestContext.RepoRoot, "/tmp/")

	testfiles.AddFileSource(testfiles.RootFileSource{
		Root: pathToTheManifests,
	})

	curDriver := BaremetalDriver()

	patcherCleanup := func() {}
	csibmOperatorCleanup := func() {}
	ginkgo.BeforeSuite(func() {
		c, err := common.GetGlobalClientSet()
		if err != nil {
			ginkgo.Fail(err.Error())
		}

		if common.BMDriverTestContext.BMDeploySchedulerPatcher {
			patcherCleanup, err = common.DeployPatcher(c, "kube-system")
			if err != nil {
				ginkgo.Fail(err.Error())
			}
		}
		if common.BMDriverTestContext.BMDeployCSIBMNodeOperator {
			e2elog.Logf("===== INSTALLING CSIBMNODECONTROLLER")
			file, err := ioutil.ReadFile(path.Join(chartsDir, operatorManifestsFolder, "csibm-controller.yaml"))
			if err != nil {
				ginkgo.Fail(err.Error())
			}

			deployment := &appsv1.Deployment{}
			err = yaml.Unmarshal(file, deployment)
			if err != nil {
				ginkgo.Fail(err.Error())
			}

			depl, err := c.AppsV1().Deployments("default").Create(deployment)
			if err != nil {
				ginkgo.Fail(err.Error())
			}

			// TODO: wait until nodes will be tagged

			csibmOperatorCleanup = func() {
				e2elog.Logf("=========== DELETING CSIBMNODEDEPL %s", depl.Name)
				if err := c.AppsV1().Deployments("default").Delete(depl.Name, &metav1.DeleteOptions{}); err != nil {
					e2elog.Logf("Failed to delete deployment %s: %v", depl.Name, err)
				}
			}
		}
	})

	ginkgo.AfterSuite(func() {
		patcherCleanup()
		csibmOperatorCleanup()
	})

	ginkgo.Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
		testsuites.DefineTestSuite(curDriver, CSITestSuites)
		DefineDriveHealthChangeTestSuite(curDriver)
		DefineControllerNodeFailTestSuite(curDriver)
		DefineNodeRebootTestSuite(curDriver)
		DefineDifferentSCTestSuite(curDriver)
		DefineStressTestSuite(curDriver)
		DefineSchedulerTestSuite(curDriver)
		DefineNodeReplacementTestSuite(curDriver)
	})
})
