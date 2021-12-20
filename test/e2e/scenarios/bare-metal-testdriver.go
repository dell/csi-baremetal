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

	"github.com/dell/csi-baremetal/test/e2e/common"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/framework/volume"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

type baremetalDriver struct {
	driverInfo testsuites.DriverInfo
}

var (
	volumeExpandTag           = "volume-expand"
	cmName                    = "loopback-config"
	persistentVolumeClaimSize = "100Mi"
	xfsFs                     = "xfs"
	ext4Fs                    = "ext4"
	ext3Fs                    = "ext3"
	hddStorageType            = "HDD"
	maxDriveSize              = "3Gi"
)

func initBaremetalDriverInfo(name string) testsuites.DriverInfo {
	return testsuites.DriverInfo{
		Name:               name,
		SupportedSizeRange: volume.SizeRange{Min: persistentVolumeClaimSize, Max: maxDriveSize},
		MaxFileSize:        testpatterns.FileSizeSmall,
		Capabilities: map[testsuites.Capability]bool{
			testsuites.CapPersistence:         true,
			testsuites.CapExec:                true,
			testsuites.CapMultiPODs:           true,
			testsuites.CapFsGroup:             true,
			testsuites.CapSingleNodeVolume:    true,
			testsuites.CapBlock:               true,
			testsuites.CapControllerExpansion: true,
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

var _ testsuites.TestDriver = &baremetalDriver{}
var _ testsuites.DynamicPVTestDriver = &baremetalDriver{}
var _ testsuites.EphemeralTestDriver = &baremetalDriver{}

// GetDriverInfo is implementation of TestDriver interface method
func (d *baremetalDriver) GetDriverInfo() *testsuites.DriverInfo {
	return &d.driverInfo
}

// SkipUnsupportedTest is implementation of TestDriver interface method
func (d *baremetalDriver) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
	if !common.BMDriverTestContext.NeedAllTests {
		// Block volume tests takes much time (20+ minutes). They should be skipped in short CI suite
		if pattern.VolMode == corev1.PersistentVolumeBlock {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}

		// Skip volume expand tests in short CI
		if pattern == testpatterns.DefaultFsDynamicPVAllowExpansion {
			e2eskipper.Skipf("Should skip volume expand tests in short CI suite - skipping")
		}

		// We have volume and exec pvc test for default fs (equals to xfs) in short CI
		// Not need to perform them for other filesystems
		if pattern.FsType == xfsFs || pattern.FsType == ext4Fs || pattern.FsType == ext3Fs {
			e2eskipper.Skipf("Should skip tests in short CI suite -- skipping")
		}
	}

	if pattern.AllowExpansion && pattern.VolMode == corev1.PersistentVolumeBlock {
		e2eskipper.Skipf("Baremetal Driver does not support block volume mode with volume expansion - skipping")
	}

	if pattern.VolType == testpatterns.PreprovisionedPV {
		e2eskipper.Skipf("Baremetal Driver does not support PreprovisionedPV -- skipping")
	}
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
		// This condition delete custom config for loopbackmanager after every suite
		if testConf.Framework.BaseName == volumeExpandTag {
			cm := d.constructDefaultLoopbackConfig(testConf.Framework.Namespace.Name)
			err := testConf.Framework.ClientSet.CoreV1().ConfigMaps(testConf.Framework.Namespace.Name).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})
			framework.ExpectNoError(err)
		}
	}
}

// PrepareTest is implementation of TestDriver interface method
func (d *baremetalDriver) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	deployConfig := true
	// This condition create custom config for loopbackmanager
	if f.BaseName == volumeExpandTag {
		testsuites.StartPodLogs(f)
		cm := d.constructDefaultLoopbackConfig(f.Namespace.Name)
		_, err := f.ClientSet.CoreV1().ConfigMaps(f.Namespace.Name).Create(context.TODO(), cm, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		deployConfig = false
	}
	return PrepareCSI(d, f, deployConfig)
}

// GetDynamicProvisionStorageClass is implementation of DynamicPVTestDriver interface method
func (d *baremetalDriver) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig,
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
	suffix := fmt.Sprintf("%s-sc", d.driverInfo.Name)
	delayedBinding := storagev1.VolumeBindingWaitForFirstConsumer
	scParams := map[string]string{
		"storageType": storageType,
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
		"fsType":      xfsFs,
	}
	return testsuites.GetStorageClass(provisioner, scParams, &delayedBinding, ns, suffix)
}

// GetClaimSize is implementation of DynamicPVTestDriver interface method
func (d *baremetalDriver) GetClaimSize() string {
	return persistentVolumeClaimSize
}

// GetVolume is implementation of EphemeralTestDriver interface method
func (d *baremetalDriver) GetVolume(config *testsuites.PerTestConfig,
	volumeNumber int) (attributes map[string]string, shared bool, readOnly bool) {
	attributes = make(map[string]string)
	attributes["size"] = d.GetClaimSize()
	attributes["storageType"] = hddStorageType
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
