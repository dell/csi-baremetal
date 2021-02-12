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
	"io/ioutil"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"sigs.k8s.io/yaml"

	"github.com/dell/csi-baremetal/test/e2e/common"
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

	testConf := &testsuites.PerTestConfig{
		Driver:    d,
		Prefix:    "baremetal",
		Framework: f,
	}

	extenderCleanup := func() {}
	if common.BMDriverTestContext.BMDeploySchedulerExtender {
		extenderCleanup, err = common.DeploySchedulerExtender(f)
		framework.ExpectNoError(err)
	}

	// always create at least one SC, this required for Inline volumes testing
	// TODO remove after ISSUE-128 will be solved
	defaultSC, err := f.ClientSet.StorageV1().StorageClasses().Create(
		d.GetDynamicProvisionStorageClass(testConf, ""))
	if err != nil {
		framework.Failf("Failed to create default SC, error: %s", err.Error())
	}
	defaultSCCleanup := func() {
		err := f.ClientSet.StorageV1().StorageClasses().Delete(defaultSC.Name, &metav1.DeleteOptions{})
		if err != nil {
			framework.Logf("Failed to remove default SC, error: ", err)
		}
	}

	cleanup := func() {
		framework.Logf("Delete loopback devices")
		err := CleanupLoopbackDevices(f)
		if err != nil {
			framework.Logf("Failed to clean up devices, error: ", err)
		}
		driverCleanup()
		extenderCleanup()
		defaultSCCleanup()
		err = d.removeAllCRs(f)
		if err != nil {
			framework.Logf("Failed to clean up CRs, error: ", err)
		}
	}
	err = e2epod.WaitForPodsRunningReady(f.ClientSet, ns, 2, 0,
		90*time.Second, nil)
	if err != nil {
		cleanup()
		framework.Failf("Pods not ready, error: %s", err.Error())
	}
	return testConf, func() {
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
		if strings.Contains(pod.Name, "baremetal-csi-node") {
			podsNames = append(podsNames, pod.Name)
		}
	}
	framework.Logf("Find node pods: ", podsNames)
	return podsNames, nil
}
