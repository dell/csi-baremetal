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
	"k8s.io/kubernetes/test/e2e/framework"
	"path"

	"github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	"github.com/dell/csi-baremetal/test/e2e_operator/common"
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
	logrus.Infof("RepoRoot: %s", framework.TestContext.RepoRoot)

	pathToTheManifests := path.Join(
		framework.TestContext.RepoRoot, "/tmp/")

	testfiles.AddFileSource(testfiles.RootFileSource{
		Root: pathToTheManifests,
	})

	curDriver := BaremetalDriver()

	operatorCleanup := func() {}
	ginkgo.BeforeSuite(func() {
		clientset, err := common.GetGlobalClientSet()
		if err != nil {
			ginkgo.Fail(err.Error())
		}

		operatorCleanup, err = common.DeployOperatorWithClient(clientset, "test")
		if err != nil {
			ginkgo.Fail(err.Error())
		}
	})

	ginkgo.AfterSuite(func() {
		operatorCleanup()
	})

	ginkgo.Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
		//testsuites.DefineTestSuite(curDriver, CSITestSuites) // work
		//DefineDriveHealthChangeTestSuite(curDriver) // work
		//DefineControllerNodeFailTestSuite(curDriver) // work
		//DefineNodeRebootTestSuite(curDriver) // work
		DefineStressTestSuite(curDriver) // work
		//DefineDifferentSCTestSuite(curDriver) // work
		//DefineSchedulerTestSuite(curDriver)
		//DefineLabeledDeployTestSuite(curDriver)
	})
})
