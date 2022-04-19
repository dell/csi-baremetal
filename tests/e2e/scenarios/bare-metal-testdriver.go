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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dell/csi-baremetal-e2e-tests/e2e/common"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

type baremetalDriver struct {
	driverInfo storageframework.DriverInfo
}

var (
	volumeExpandTag           = "volume-expand"
	cmName                    = "loopback-config"
	persistentVolumeClaimSize = "100Mi"
	xfsFs                     = "xfs"
	ext4Fs                    = "ext4"
	ext3Fs                    = "ext3"
	hddStorageType            = "HDD"
	// default value for expansion is hardcoded to 1Gi in e2e test framework
	maxDriveSize = "2.1Gi"
)

func initBaremetalDriverInfo(name string) storageframework.DriverInfo {
	return storageframework.DriverInfo{
		Name:               name,
		SupportedSizeRange: volume.SizeRange{Min: persistentVolumeClaimSize, Max: maxDriveSize},
		MaxFileSize:        storageframework.FileSizeSmall,
		Capabilities: map[storageframework.Capability]bool{
			storageframework.CapPersistence:         true,
			storageframework.CapExec:                true,
			storageframework.CapMultiPODs:           true,
			storageframework.CapFsGroup:             true,
			storageframework.CapSingleNodeVolume:    true,
			storageframework.CapBlock:               true,
			storageframework.CapControllerExpansion: true,
		},
		SupportedFsType: sets.NewString(
			"", // Default fsType
			xfsFs,
			ext4Fs,
			ext3Fs,
		),
	}
}

// InitBaremetalDriver initialize driver with short-ci flag
func initBaremetalDriver() *baremetalDriver {
	return &baremetalDriver{
		driverInfo: initBaremetalDriverInfo("csi-baremetal"),
	}
}

var _ storageframework.TestDriver = &baremetalDriver{}
var _ storageframework.DynamicPVTestDriver = &baremetalDriver{}
var _ storageframework.PreprovisionedPVTestDriver = &baremetalDriver{}

// GetDriverInfo is implementation of TestDriver interface method
func (d *baremetalDriver) GetDriverInfo() *storageframework.DriverInfo {
	return &d.driverInfo
}

// SkipUnsupportedTest is implementation of TestDriver interface method
func (d *baremetalDriver) SkipUnsupportedTest(pattern storageframework.TestPattern) {
	if !common.BMDriverTestContext.NeedAllTests {
		// Block volume tests takes much time (20+ minutes). They should be skipped in short CI suite
		if pattern.VolMode == corev1.PersistentVolumeBlock {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}

		// Skip volume expand tests in short CI
		if pattern == storageframework.DefaultFsDynamicPVAllowExpansion {
			e2eskipper.Skipf("Should skip volume expand tests in short CI suite - skipping")
		}

		// We have volume and exec pvc test for default fs (equals to xfs) in short CI
		// Not need to perform them for other filesystems
		if pattern.FsType == xfsFs || pattern.FsType == ext4Fs || pattern.FsType == ext3Fs {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}

		// too long for short CI
		if pattern.Name == "Dynamic PV (filesystem volmode)" {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}

		// too long for short CI
		if pattern.Name == "Generic Ephemeral-volume (default fs) (late-binding)" {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}
	}

	// skip inline volume test since not supported anymore
	if pattern.VolType == storageframework.InlineVolume || pattern.VolType == storageframework.CSIInlineVolume {
		e2eskipper.Skipf("Baremetal Driver does not support Inline Volumes -- skipping")
	}

	if pattern.BindingMode == storagev1.VolumeBindingImmediate {
		e2eskipper.Skipf("Immediate volume binding mode is not supported -- skipping")
	}

	if pattern.AllowExpansion && pattern.VolMode == corev1.PersistentVolumeBlock {
		e2eskipper.Skipf("Baremetal Driver does not support block volume mode with volume expansion - skipping")
	}

	// TODO https://github.com/dell/csi-baremetal/issues/666 - add test coverage
	if pattern.VolType == storageframework.PreprovisionedPV {
		e2eskipper.Skipf("Baremetal Driver does not have PreprovisionedPV test suite implemented yet -- skipping")
	}
}

// PrepareCSI deploys CSI and enables logging for containers
func PrepareCSI(d *baremetalDriver, f *framework.Framework, deployConfig bool) (*storageframework.PerTestConfig, func()) {
	ginkgo.By("Deploying CSI Baremetal")

	installArgs := ""
	if deployConfig {
		installArgs += "--set driver.drivemgr.deployConfig=true"
	}
	cleanup, err := common.DeployCSIComponents(f, installArgs)
	framework.ExpectNoError(err)

	testConf := &storageframework.PerTestConfig{
		Driver:    d,
		Prefix:    "baremetal",
		Framework: f,
	}

	return testConf, func() {
		// wait until ephemeral volume will be deleted
		time.Sleep(time.Second * 20)
		ginkgo.By("Uninstalling CSI Baremetal")
		cleanup()
		// This condition delete custom config for loopbackmanager after every suite
		if testConf.Framework.BaseName == volumeExpandTag {
			cm := d.constructDefaultLoopbackConfig(testConf.Framework.Namespace.Name)
			err := testConf.Framework.ClientSet.CoreV1().ConfigMaps(testConf.Framework.Namespace.Name).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}
	}
}

// PrepareTest is implementation of TestDriver interface method
func (d *baremetalDriver) PrepareTest(f *framework.Framework) (*storageframework.PerTestConfig, func()) {
	deployConfig := true
	// This condition create custom config for loopbackmanager
	if f.BaseName == volumeExpandTag {
		utils.StartPodLogs(f, f.Namespace)
		cm := d.constructDefaultLoopbackConfig(f.Namespace.Name)
		_, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(context.TODO(), cm, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		deployConfig = false
	}
	return PrepareCSI(d, f, deployConfig)
}

// GetDynamicProvisionStorageClass is implementation of DynamicPVTestDriver interface method
func (d *baremetalDriver) GetDynamicProvisionStorageClass(config *storageframework.PerTestConfig,
	fsType string) *storagev1.StorageClass {
	var scFsType string
	switch strings.ToLower(fsType) {
	case "", xfsFs:
		scFsType = xfsFs
	default:
		scFsType = fsType
	}
	storageType := hddStorageType
	if config.Framework.BaseName == volumeExpandTag {
		storageType = "HDDLVG"
	}
	ns := config.Framework.Namespace.Name
	provisioner := d.driverInfo.Name
	//suffix := fmt.Sprintf("%s-sc", d.driverInfo.Name)
	delayedBinding := storagev1.VolumeBindingWaitForFirstConsumer
	scParams := map[string]string{
		"storageType": storageType,
		"fsType":      scFsType,
	}

	return storageframework.GetStorageClass(provisioner, scParams, &delayedBinding, ns)
}

// GetStorageClassWithStorageType allows to create SC with different storageType
func (d *baremetalDriver) GetStorageClassWithStorageType(config *storageframework.PerTestConfig,
	storageType string) *storagev1.StorageClass {
	ns := config.Framework.Namespace.Name
	provisioner := d.driverInfo.Name
	//suffix := fmt.Sprintf("%s-sc", d.driverInfo.Name)
	delayedBinding := storagev1.VolumeBindingWaitForFirstConsumer
	scParams := map[string]string{
		"storageType": storageType,
		"fsType":      xfsFs,
	}
	return storageframework.GetStorageClass(provisioner, scParams, &delayedBinding, ns)
}

// GetClaimSize is implementation of DynamicPVTestDriver interface method
func (d *baremetalDriver) GetClaimSize() string {
	return persistentVolumeClaimSize
}

var _ storageframework.EphemeralTestDriver = &baremetalDriver{}

// GetVolume is implementation of EphemeralTestDriver interface method
func (d *baremetalDriver) GetVolume(config *storageframework.PerTestConfig,
	volumeNumber int) (attributes map[string]string, shared bool, readOnly bool) {
	attributes = make(map[string]string)
	attributes["size"] = d.GetClaimSize()
	attributes["storageType"] = hddStorageType
	return attributes, false, false
}

// GetCSIDriverName is implementation of EphemeralTestDriver interface method
func (d *baremetalDriver) GetCSIDriverName(config *storageframework.PerTestConfig) string {
	return d.GetDriverInfo().Name
}

// CreateVolume is implementation of PreprovisionedPVTestDriver interface method
func (d *baremetalDriver) CreateVolume(config *storageframework.PerTestConfig, volumeType storageframework.TestVolType) storageframework.TestVolume {
	panic("implement me")
}

// GetPersistentVolumeSource is implementation of PreprovisionedPVTestDriver interface method
func (d *baremetalDriver) GetPersistentVolumeSource(readOnly bool, fsType string, testVolume storageframework.TestVolume) (*corev1.PersistentVolumeSource, *corev1.VolumeNodeAffinity) {
	panic("implement me")
}

// constructDefaultLoopbackConfig constructs default ConfigMap for LoopBackManager
// Receives namespace where cm should be deployed
func (d *baremetalDriver) constructDefaultLoopbackConfig(namespace string) *corev1.ConfigMap {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: namespace,
			Labels:    map[string]string{"app": "csi-baremetal-node"},
		},
		Data: map[string]string{
			"config.yaml": "\n" +
				fmt.Sprintf("defaultDriveSize: %s\n", maxDriveSize) +
				"defaultDrivePerNodeCount: 1\n" +
				"nodes:\n",
		},
	}

	return &cm
}
