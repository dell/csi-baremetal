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
	"github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"
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
	var (
		curDriver = BaremetalDriver()
	)

	ginkgo.Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
		//testsuites.DefineTestSuite(curDriver, CSITestSuites)
		//DefineDriveHealthChangeTestSuite(curDriver)
		DefineControllerNodeFailTestSuite(curDriver)
		//DefineNodeRebootTestSuite(curDriver)
		//DefineStressTestSuite(curDriver)
		//DefineDifferentSCTestSuite(curDriver)
		//DefineSchedulerTestSuite(curDriver)
		//DefineLabeledDeployTestSuite()
	})
})
