package scenarios

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"sigs.k8s.io/yaml"
)

type baremetalDriver struct {
	driverInfo testsuites.DriverInfo
	scManifest map[string]string
}

func (n *baremetalDriver) GetClaimSize() string {
	return "100Mi"
}

var (
	BaremetalDriver = InitBaremetalDriver
	cmName          = "loopback-config"
)

func initBaremetalDriver(name string) testsuites.TestDriver {
	return &baremetalDriver{
		driverInfo: testsuites.DriverInfo{
			Name:        name,
			MaxFileSize: testpatterns.FileSizeSmall,
			Capabilities: map[testsuites.Capability]bool{
				testsuites.CapPersistence:      true,
				testsuites.CapExec:             true,
				testsuites.CapMultiPODs:        true,
				testsuites.CapFsGroup:          true,
				testsuites.CapSingleNodeVolume: true,
			},
			SupportedFsType: sets.NewString(
				"", // Default fsType
				"xfs",
				"ext4",
				"ext3",
			),
		},
	}
}

func InitBaremetalDriver() testsuites.TestDriver {
	return initBaremetalDriver("baremetal-csi")
}

var _ testsuites.TestDriver = &baremetalDriver{}
var _ testsuites.DynamicPVTestDriver = &baremetalDriver{}
var _ testsuites.EphemeralTestDriver = &baremetalDriver{}

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
		"baremetal-csi-node.yaml",
	}
	file, err := ioutil.ReadFile("/tmp/baremetal-csi-plugin/templates/baremetal-csi-controller.yaml")
	framework.ExpectNoError(err)

	deployment := &v1.Deployment{}
	err = yaml.Unmarshal(file, deployment)
	framework.ExpectNoError(err)

	ns := f.Namespace.Name
	f.PatchNamespace(&deployment.ObjectMeta.Namespace)
	_, err = f.ClientSet.AppsV1().Deployments(ns).Create(deployment)
	framework.ExpectNoError(err)

	// CreateFromManifests doesn't support ConfigMaps so deploy it from framework's client
	_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(n.constructDefaultLoopbackConfig(ns))
	framework.ExpectNoError(err)

	cleanup, err := f.CreateFromManifests(nil, manifests...)

	if err != nil {
		framework.Failf("deploying csi baremetal driver: %v", err)
	}

	return &testsuites.PerTestConfig{
			Driver:    n,
			Prefix:    "baremetal",
			Framework: f,
		}, func() {
			//wait until ephemeral volume will be deleted
			time.Sleep(time.Second * 20)
			err = f.ClientSet.AppsV1().Deployments(ns).Delete(deployment.Name, &metav1.DeleteOptions{})
			framework.ExpectNoError(err)
			ginkgo.By("uninstalling baremetal driver")
			cleanup()
			cancelLogging()
		}
}

func (n *baremetalDriver) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig, fsType string) *v12.StorageClass {
	var scFsType string
	switch strings.ToLower(fsType) {
	case "", "xfs":
		scFsType = "xfs"
	default:
		scFsType = fsType
	}
	ns := config.Framework.Namespace.Name
	provisioner := n.driverInfo.Name
	suffix := fmt.Sprintf("%s-sc", n.driverInfo.Name)
	delayedBinding := v12.VolumeBindingWaitForFirstConsumer
	scParams := map[string]string{
		"storageType": "HDD",
		"fsType":      scFsType,
	}

	return testsuites.GetStorageClass(provisioner, scParams, &delayedBinding, ns, suffix)
}

func (n *baremetalDriver) GetVolume(config *testsuites.PerTestConfig, volumeNumber int) (attributes map[string]string, shared bool, readOnly bool) {
	attributes = make(map[string]string)
	attributes["size"] = n.GetClaimSize()
	attributes["storageType"] = "HDD"
	return attributes, false, false
}

func (n *baremetalDriver) GetCSIDriverName(config *testsuites.PerTestConfig) string {
	return n.GetDriverInfo().Name
}

// constructDefaultLoopbackConfig constructs default ConfigMap for LoopBackManager
// Receives namespace where cm should be deployed
func (n *baremetalDriver) constructDefaultLoopbackConfig(namespace string) *coreV1.ConfigMap {
	cm := coreV1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"config.yaml": "\n" +
				"defaultDrivePerNodeCount: 3\n" +
				"nodes:\n",
		},
	}

	return &cm
}
