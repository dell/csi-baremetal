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

/*
  test package includes baremetal test storage class definition for e2e tests
  and definition of e2e test suites with ginkgo library
  main file for e2e tests is in cmd/tests directory
  we can run defined test suites with following command:
  go test cmd/tests/baremetal_e2e.go -ginkgo.v -ginkgo.progress --kubeconfig=<kubeconfig>
*/
package scenarios

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	"github.com/dell/csi-baremetal/test/e2e/common"
)

// CSITestSuites is a community tests
var CSITestSuites = []func() testsuites.TestSuite{
	testsuites.InitVolumesTestSuite,
	testsuites.InitVolumeIOTestSuite,
	testsuites.InitEphemeralTestSuite,
	testsuites.InitProvisioningTestSuite,
	testsuites.InitMultiVolumeTestSuite,
	testsuites.InitVolumeModeTestSuite,
	testsuites.InitVolumeExpandTestSuite,
}

var _ = utils.SIGDescribe("CSI Volumes", func() {
	ginkgo.AfterEach(failTestIfTimeout)

	ginkgo.Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
		testsuites.DefineTestSuite(curDriver, CSITestSuites)
		DefineDriveHealthChangeTestSuite(curDriver)
		DefineControllerNodeFailTestSuite(curDriver)
		DefineNodeRebootTestSuite(curDriver)
		DefineStressTestSuite(curDriver)
		DefineDifferentSCTestSuite(curDriver)
		DefineSchedulerTestSuite(curDriver)
		DefineLabeledDeployTestSuite()
	})
})

func skipIfNotAllTests() {
	if !common.BMDriverTestContext.NeedAllTests {
		e2eskipper.Skipf("Short CI suite -- skipping")
	}
}

func failTestIfTimeout() {
	if common.BMDriverTestContext.NeedAllTests {
		e2elog.Logf("Skip timeout due to all tests suite")
		return
	}
	if common.BMDriverTestContext.Timeout == 0 {
		e2elog.Logf("Timeout is not set")
		return
	}

	endTime := startTime.Add(common.BMDriverTestContext.Timeout)
	isTimeoutPassed := time.Now().After(endTime)

	if isTimeoutPassed {
		massage := fmt.Sprintf("Timeout %v passed", &common.BMDriverTestContext.Timeout)
		ginkgo.Fail(massage)
	}
}
