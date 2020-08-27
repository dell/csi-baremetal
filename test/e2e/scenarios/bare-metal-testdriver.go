package scenarios

import (
	"fmt"
	"github.com/dell/csi-baremetal/test/e2e/common"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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

var (
	BaremetalDriver = InitBaremetalDriver
	cmName          = "loopback-config"
	manifestsFolder = "baremetal-csi-plugin/templates/"
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

// GetDriverInfo is implementation of TestDriver interface method
func (d *baremetalDriver) GetDriverInfo() *testsuites.DriverInfo {
	return &d.driverInfo
}

// SkipUnsupportedTest is implementation of TestDriver interface method
func (d *baremetalDriver) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
	if pattern.VolType == testpatterns.InlineVolume || pattern.VolType == testpatterns.PreprovisionedPV {
		framework.Skipf("Baremetal Driver does not support InlineVolume and PreprovisionedPV -- skipping")
	}
}

// PrepareTest is implementation of TestDriver interface method
func (d *baremetalDriver) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	ginkgo.By("deploying baremetal driver")

	cancelLogging := testsuites.StartPodLogs(f)

	manifests := []string{
		manifestsFolder + "controller-rbac.yaml",
		manifestsFolder + "node-rbac.yaml",
		manifestsFolder + "baremetal-csi-node.yaml",
	}
	file, err := ioutil.ReadFile("/tmp/baremetal-csi-plugin/templates/baremetal-csi-controller.yaml")
	framework.ExpectNoError(err)

	deployment := &appsv1.Deployment{}
	err = yaml.Unmarshal(file, deployment)
	framework.ExpectNoError(err)

	ns := f.Namespace.Name
	f.PatchNamespace(&deployment.ObjectMeta.Namespace)
	_, err = f.ClientSet.AppsV1().Deployments(ns).Create(deployment)
	framework.ExpectNoError(err)

	// CreateFromManifests doesn't support ConfigMaps so deploy it from framework's client
	_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(d.constructDefaultLoopbackConfig(ns))
	if !errors.IsAlreadyExists(err) {
		framework.ExpectNoError(err)
	}

	driverCleanup, err := f.CreateFromManifests(nil, manifests...)

	if err != nil {
		framework.Failf("deploying csi baremetal driver: %v", err)
	}

	extenderCleanup := common.DeploySchedulerExtender(f)
	time.Sleep(time.Second * 30)  // quick hack, need to wait until default scheduler will be restarted

	cleanup := func() {
		driverCleanup()
		extenderCleanup()
	}

	return &testsuites.PerTestConfig{
			Driver:    d,
			Prefix:    "baremetal",
			Framework: f,
		}, func() {
			// wait until ephemeral volume will be deleted
			time.Sleep(time.Second * 20)
			err = f.ClientSet.AppsV1().Deployments(ns).Delete(deployment.Name, &metav1.DeleteOptions{})
			framework.ExpectNoError(err)
			ginkgo.By("uninstalling baremetal driver")
			cleanup()
			cancelLogging()
		}
}

// GetDynamicProvisionStorageClass is implementation of DynamicPVTestDriver interface method
func (d *baremetalDriver) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig,
	fsType string) *storagev1.StorageClass {
	var scFsType string
	switch strings.ToLower(fsType) {
	case "", "xfs":
		scFsType = "xfs"
	default:
		scFsType = fsType
	}
	ns := config.Framework.Namespace.Name
	provisioner := d.driverInfo.Name
	suffix := fmt.Sprintf("%s-sc", d.driverInfo.Name)
	delayedBinding := storagev1.VolumeBindingWaitForFirstConsumer
	scParams := map[string]string{
		"storageType": "HDD",
		"fsType":      scFsType,
	}

	return testsuites.GetStorageClass(provisioner, scParams, &delayedBinding, ns, suffix)
}

// GetStorageClassWithStorageType allows to create SC with different storageType
func (d *baremetalDriver) GetStorageClassWithStorageType(config *testsuites.PerTestConfig,
	storageType string) *storagev1.StorageClass {
	ns := config.Framework.Namespace.Name
	provisioner := d.driverInfo.Name
	suffix := fmt.Sprintf("%s-sc", d.driverInfo.Name)
	delayedBinding := storagev1.VolumeBindingWaitForFirstConsumer
	scParams := map[string]string{
		"storageType": storageType,
		"fsType":      "xfs",
	}
	return testsuites.GetStorageClass(provisioner, scParams, &delayedBinding, ns, suffix)
}

// GetClaimSize is implementation of DynamicPVTestDriver interface method
func (d *baremetalDriver) GetClaimSize() string {
	// for LVM need to align with VG PE size
	// todo address this issue in https://jira.cec.lab.emc.com:8443/browse/ATLDEF-56
	return "96Mi"
}

// GetVolume is implementation of EphemeralTestDriver interface method
func (d *baremetalDriver) GetVolume(config *testsuites.PerTestConfig,
	volumeNumber int) (attributes map[string]string, shared bool, readOnly bool) {
	attributes = make(map[string]string)
	attributes["size"] = d.GetClaimSize()
	attributes["storageType"] = "HDD"
	return attributes, false, false
}

// GetCSIDriverName is implementation of EphemeralTestDriver interface method
func (d *baremetalDriver) GetCSIDriverName(config *testsuites.PerTestConfig) string {
	return d.GetDriverInfo().Name
}

// constructDefaultLoopbackConfig constructs default ConfigMap for LoopBackManager
// Receives namespace where cm should be deployed
func (d *baremetalDriver) constructDefaultLoopbackConfig(namespace string) *corev1.ConfigMap {
	cm := corev1.ConfigMap{
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
