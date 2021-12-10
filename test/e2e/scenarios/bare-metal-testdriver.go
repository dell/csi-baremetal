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
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal/test/e2e/common"
)

type baremetalDriver struct {
	driverInfo   testsuites.DriverInfo
	needAllTests bool
}

var (
	cmName                    = "loopback-config"
	PersistentVolumeClaimSize = "100Mi"
)

func initBaremetalDriverInfo(name string) testsuites.DriverInfo {
	return testsuites.DriverInfo{
		Name:        name,
		MaxFileSize: testpatterns.FileSizeSmall,
		Capabilities: map[testsuites.Capability]bool{
			testsuites.CapPersistence:      true,
			testsuites.CapExec:             true,
			testsuites.CapMultiPODs:        true,
			testsuites.CapFsGroup:          true,
			testsuites.CapSingleNodeVolume: true,
			testsuites.CapBlock:            true,
		},
		SupportedFsType: sets.NewString(
			"", // Default fsType
			"xfs",
			"ext4",
			"ext3",
		),
	}
}

func InitBaremetalDriver(needAllTests bool) *baremetalDriver {
	return &baremetalDriver{
		driverInfo:   initBaremetalDriverInfo("csi-baremetal"),
		needAllTests: needAllTests,
	}
}

var _ testsuites.TestDriver = &baremetalDriver{}
var _ testsuites.DynamicPVTestDriver = &baremetalDriver{}
var _ testsuites.EphemeralTestDriver = &baremetalDriver{}
var _ testsuites.PreprovisionedPVTestDriver = &baremetalDriver{}

// GetDriverInfo is implementation of TestDriver interface method
func (d *baremetalDriver) GetDriverInfo() *testsuites.DriverInfo {
	return &d.driverInfo
}

// SkipUnsupportedTest is implementation of TestDriver interface method
func (d *baremetalDriver) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
	if !d.needAllTests {
		// Block volume tests takes much time (20+ minutes). They should be skipped in short CI suite
		if pattern.VolMode == corev1.PersistentVolumeBlock {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}

		// We have volume and exec pvc test for default fs (equals to xfs) in short CI
		// Not need to perform them for other filesystems
		if pattern.FsType == "xfs" || pattern.FsType == "ext4" || pattern.FsType == "ext3" {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}
	}

	/*if pattern.VolType == testpatterns.PreprovisionedPV {
		e2eskipper.Skipf("Baremetal Driver does not support PreprovisionedPV -- skipping")
	}*/
}

// PrepareCSI deploys CSI and enables logging for containers
func PrepareCSI(d *baremetalDriver, f *framework.Framework, deployConfig bool) (*testsuites.PerTestConfig, func()) {
	ginkgo.By("Deploying CSI Baremetal")

	installArgs := ""
	if deployConfig {
		installArgs += "--set driver.drivemgr.deployConfig=true"
	}
	cleanup, err := common.DeployCSIComponents(f, installArgs)
	framework.ExpectNoError(err)

	testConf := &testsuites.PerTestConfig{
		Driver:    d,
		Prefix:    "baremetal",
		Framework: f,
	}

	return testConf, func() {
		// wait until ephemeral volume will be deleted
		time.Sleep(time.Second * 20)
		ginkgo.By("Uninstalling CSI Baremetal")
		cleanup()
	}
}

// PrepareTest is implementation of TestDriver interface method
func (d *baremetalDriver) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	return PrepareCSI(d, f, true)
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
	return "100Mi"
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

// CreateVolume is implementation of PreprovisionedPVTestDriver interface method
func (d *baremetalDriver) CreateVolume(config *testsuites.PerTestConfig, volumeType testpatterns.TestVolType) testsuites.TestVolume {
	panic("implement me")
}

// CreateVolume is implementation of GetPersistentVolumeSource interface method
func (d *baremetalDriver) GetPersistentVolumeSource(readOnly bool, fsType string, testVolume testsuites.TestVolume) (*corev1.PersistentVolumeSource, *corev1.VolumeNodeAffinity) {
	panic("implement me")
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
