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

	"github.com/dell/csi-baremetal/test/e2e/common"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

type baremetalDriver struct {
	driverInfo testsuites.DriverInfo
	scManifest map[string]string
}

var (
	BaremetalDriver = InitBaremetalDriver
	cmName          = "loopback-config"
	manifestsFolder = "csi-baremetal-driver/templates/"
)

func initBaremetalDriver(name string) *baremetalDriver {
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

func InitBaremetalDriver() *baremetalDriver {
	return initBaremetalDriver("csi-baremetal")
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

func PrepareCSI(d *baremetalDriver, f *framework.Framework, installArgs string) (*testsuites.PerTestConfig, func()) {
	ginkgo.By("deploying baremetal driver")

	cancelLogging := testsuites.StartPodLogs(f)

	cleanupCSI, err := common.DeployCSI(f, installArgs)
	framework.ExpectNoError(err)

	testConf := &testsuites.PerTestConfig{
		Driver:    d,
		Prefix:    "baremetal",
		Framework: f,
	}

	cleanup := func() {
		framework.Logf("Delete loopback devices")
		cleanupCSI()
	}

	return testConf, func() {
		// wait until ephemeral volume will be deleted
		time.Sleep(time.Second * 20)
		ginkgo.By("uninstalling baremetal driver")
		cleanup()
		cancelLogging()
	}
}

// PrepareTest is implementation of TestDriver interface method
func (d *baremetalDriver) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	return PrepareCSI(d, f, "")
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

// removeAllCRs removes all CRs that were created during plugin installation except
// CSIBMNodes CRs because CSIBMNodes CRs creates once at common BeforeSuite step
func (d *baremetalDriver) removeAllCRs(f *framework.Framework) error {
	var savedErr error
	for _, gvr := range common.AllGVRs {
		err := f.DynamicClient.Resource(gvr).Namespace("").DeleteCollection(
			&metav1.DeleteOptions{}, metav1.ListOptions{})
		if err != nil {
			e2elog.Logf("Failed to clean CR %s: %s", gvr.String(), err.Error())
			savedErr = err
		}
	}
	return savedErr
}

// CleanupLoopbackDevices executes in node pods drive managers containers kill -SIGHUP 1
// Returns error if it's failed to get node pods
func CleanupLoopbackDevices(f *framework.Framework) error {
	pods, err := getNodePodsNames(f)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		f.ExecShellInContainer(pod, "drivemgr", "/bin/kill -SIGHUP 1")
	}
	return nil
}

// getNodePodsNames tries to get slice of node pods names
// Receives framework.Framewor
// Returns slice of pods name, error if it's failed to get node pods
func getNodePodsNames(f *framework.Framework) ([]string, error) {
	pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	podsNames := make([]string, 0)
	for _, pod := range pods.Items {
		if len(pod.OwnerReferences) == 1 &&
			pod.OwnerReferences[0].Name == "csi-baremetal-node" &&
			pod.OwnerReferences[0].Kind == "DaemonSet" {
			podsNames = append(podsNames, pod.Name)
		}
	}
	framework.Logf("Find node pods: ", podsNames)
	return podsNames, nil
}
