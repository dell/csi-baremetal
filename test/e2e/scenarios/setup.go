/*test package includes baremetal test storage class definition for e2e tests
  and definition of e2e test suites with ginkgo library
  main file for e2e tests is in cmd/tests directory
  we can run defined test suites with following command:
  go test cmd/tests/baremetal_e2e.go -ginkgo.v -ginkgo.progress --kubeconfig=<kubeconfig>
*/
package scenarios

import (
	"path"

	"github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"

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
	ginkgo.Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
		testsuites.DefineTestSuite(curDriver, CSITestSuites)
		DefineDriveHealthChangeTestSuite(curDriver)
		DefineControllerNodeFailTestSuite(curDriver)
		DefineNodeRebootTestSuite(curDriver)
		DefineDifferentSCTestSuite(curDriver)
		DefineStressTestSuite(curDriver)
		DefineSchedulerTestSuite(curDriver)
	})
})
