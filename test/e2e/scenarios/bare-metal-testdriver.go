package scenarios

import (
	"fmt"

	"github.com/onsi/ginkgo"
	v12 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

type baremetalDriver struct {
	driverInfo testsuites.DriverInfo
}

func (n *baremetalDriver) GetClaimSize() string {
	return "100Mi"
}

var BaremetalDriver = InitBaremetalDriver

func initBaremetalDriver(name string) testsuites.TestDriver {
	return &baremetalDriver{
		driverInfo: testsuites.DriverInfo{
			Name:        name,
			MaxFileSize: testpatterns.FileSizeSmall,
			Capabilities: map[testsuites.Capability]bool{
				testsuites.CapPersistence: true,
			},
			SupportedFsType: sets.NewString(
				"", // Default fsType
			),
		},
	}
}

func InitBaremetalDriver() testsuites.TestDriver {
	return initBaremetalDriver("baremetal-csi")
}

var _ testsuites.TestDriver = &baremetalDriver{}
var _ testsuites.DynamicPVTestDriver = &baremetalDriver{}

func (n *baremetalDriver) GetDriverInfo() *testsuites.DriverInfo {
	return &n.driverInfo
}

func (n *baremetalDriver) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
	if pattern.VolType == testpatterns.InlineVolume || pattern.VolType == testpatterns.PreprovisionedPV {
		framework.Skipf("Baremetal Driver does not support InlineVolume and PreprovisionedPV -- skipping")
	}
}
func (n *baremetalDriver) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	ginkgo.By("deploying baremetal driver")

	cancelLogging := testsuites.StartPodLogs(f)

	manifests := []string{
		"controller-rbac.yaml",
		"node-rbac.yaml",
		"baremetal-csi-controller.yaml",
		"baremetal-csi-node.yaml",
		"baremetal-csi-sc.yaml",
	}

	cleanup, err := f.CreateFromManifests(nil, manifests...)

	if err != nil {
		framework.Failf("deploying csi baremetal driver: %v", err)
	}

	return &testsuites.PerTestConfig{
			Driver:    n,
			Prefix:    "baremetal",
			Framework: f,
		}, func() {
			ginkgo.By("uninstalling baremetal driver")
			cleanup()
			cancelLogging()
		}
}

func (n *baremetalDriver) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig, fsType string) *v12.StorageClass {
	ns := config.Framework.Namespace.Name
	provisioner := n.driverInfo.Name
	suffix := fmt.Sprintf("%s-sc", n.driverInfo.Name)
	delayedBinding := v12.VolumeBindingWaitForFirstConsumer

	return testsuites.GetStorageClass(provisioner, map[string]string{}, &delayedBinding, ns, suffix)
}
