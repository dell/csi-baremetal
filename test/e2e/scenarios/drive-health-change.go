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
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	akey "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/test/e2e/common"
)

const (
	// maximum time to wait before drive state will change after LM config change
	driveStateChangeTimeout = time.Minute * 3
)

var (
	pvcName = "baremetal-csi-pvc"
)

// DefineDriveHealthChangeTestSuite defines custom csi-baremetal e2e tests
func DefineDriveHealthChangeTestSuite(driver *baremetalDriver) {
	ginkgo.Context("Baremetal-csi drive health change tests", func() {
		// It consists of two steps.
		// 1) Set random drive to Failed state and see that amount of ACs reduced by 1.
		// 2) Install pod with pvc. Set drive which is used by pvc to Failed state. See that appropriate VolumeCR
		// changes its status too
		driveHealthChangeTest(driver)
	})
}

// driveHealthChangeTest test checks behavior of driver when drives change health from GOOD to BAD
func driveHealthChangeTest(driver *baremetalDriver) {
	ginkgo.BeforeEach(skipIfNotAllTests)

	var (
		testPODs      []*corev1.Pod
		testPVCs      []*corev1.PersistentVolumeClaim
		k8sSC         *storagev1.StorageClass
		driverCleanup func()
		ns            string
		eventManager  = &eventing.EventManager{}
		f             = framework.NewDefaultFramework("health")
	)

	init := func(lmConf *common.LoopBackManagerConfig) {
		var (
			perTestConf *testsuites.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name

		perTestConf, driverCleanup = driver.PrepareTest(f)

		k8sSC = driver.GetDynamicProvisionStorageClass(perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(context.TODO(), k8sSC, metav1.CreateOptions{})
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test DriveHealthChange")
		common.CleanupAfterCustomTest(f, driverCleanup, testPODs, testPVCs)
	}

	ginkgo.It("AC for unhealthy drive should be removed", func() {
		defaultDriveCount := 3
		conf := &common.LoopBackManagerConfig{DefaultDriveCount: &defaultDriveCount}

		init(conf)
		defer cleanup()

		acUnstructuredList := getUObjList(f, common.ACGVR)
		// Save amount of ACs before drive's health changing
		amountOfACBeforeDiskFailure := len(acUnstructuredList.Items)
		e2elog.Logf("found %d ac", amountOfACBeforeDiskFailure)

		targetAC := acUnstructuredList.Items[0]
		acLocation, _, err := unstructured.NestedString(targetAC.Object, "spec", "Location")
		framework.ExpectNoError(err)
		nodeUidOfAC, _, err := unstructured.NestedString(targetAC.Object, "spec", "NodeId")
		framework.ExpectNoError(err)
		nodeNameOfAC, err := findNodeNameByUID(f, nodeUidOfAC)
		framework.ExpectNoError(err)
		targetDrive, found := getUObj(f, common.DriveGVR, acLocation)
		Expect(found).To(BeTrue())
		targetDriveName, _, err := unstructured.NestedString(targetDrive.Object, "metadata", "name")
		framework.ExpectNoError(err)
		targetDriveSN, _, err := unstructured.NestedString(targetDrive.Object, "spec", "SerialNumber")
		framework.ExpectNoError(err)

		targetDriveNewHealth := apiV1.HealthBad
		// Append bad-health drive to this config and update config on the cluster side
		conf.Nodes = []common.LoopBackManagerConfigNode{{
			NodeID: &nodeNameOfAC,
			Drives: []common.LoopBackManagerConfigDevice{{
				SerialNumber: &targetDriveSN,
				Health:       &targetDriveNewHealth},
			}}}
		applyLMConfig(f, conf)

		// wait for drive health change
		waitForObjStateChange(f, common.DriveGVR, targetDriveName, driveStateChangeTimeout,
			targetDriveNewHealth, "spec", "Health")

		// Read ACs one more time with retry
		deadline := time.Now().Add(time.Second * 30)
		for {
			acUnstructuredList = getUObjList(f, common.ACGVR)
			targetAC := acUnstructuredList.Items[0]
			size, _, err := unstructured.NestedInt64(targetAC.Object, "spec", "Size")
			framework.ExpectNoError(err)
			if size == 0 {
				e2elog.Logf("AC size is 0")
				return
			}
			if time.Now().After(deadline) {
				framework.Failf("AC size is %d, should be 0", size)
			}
			time.Sleep(time.Second * 3)
		}
	})

	ginkgo.It("volume health should change after drive health changed", func() {
		defaultDriveCount := 3
		conf := &common.LoopBackManagerConfig{DefaultDriveCount: &defaultDriveCount}
		init(conf)

		defer cleanup()
		// Create test pvc on the cluster
		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(),
			constructPVC(ns, driver.GetClaimSize(), k8sSC.Name, pvcName),
			metav1.CreateOptions{})
		framework.ExpectNoError(err)

		// Create test pod that consumes the pvc
		pod, err := common.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)
		testPVCs = append(testPVCs, pvc)
		testPODs = append(testPODs, pod)

		// Get Volume CRs and save variables to identify on which drive the pod's Volume based on
		volumesUnstructuredList, _ := f.DynamicClient.Resource(common.VolumeGVR).List(context.TODO(), metav1.ListOptions{})
		targetVolume := volumesUnstructuredList.Items[0]
		location, _, err := unstructured.NestedString(targetVolume.Object, "spec", "Location")
		framework.ExpectNoError(err)
		volumeName, _, err := unstructured.NestedString(targetVolume.Object, "metadata", "name")
		framework.ExpectNoError(err)
		nodeUidOfVolume, _, err := unstructured.NestedString(targetVolume.Object, "spec", "NodeId")
		framework.ExpectNoError(err)
		nodeNameOfVolume, err := findNodeNameByUID(f, nodeUidOfVolume)
		framework.ExpectNoError(err)
		targetDrive, found := getUObj(f, common.DriveGVR, location)
		Expect(found).To(BeTrue())
		targetDriveSN, _, err := unstructured.NestedString(targetDrive.Object, "spec", "SerialNumber")
		framework.ExpectNoError(err)

		targetDriveNewHealth := apiV1.HealthBad
		// Append bad-health drive to this config and update config on the cluster side
		conf.Nodes = []common.LoopBackManagerConfigNode{{
			NodeID: &nodeNameOfVolume,
			Drives: []common.LoopBackManagerConfigDevice{{
				SerialNumber: &targetDriveSN,
				Health:       &targetDriveNewHealth},
			}}}
		applyLMConfig(f, conf)

		// wait for volume health change
		waitForObjStateChange(f, common.VolumeGVR, volumeName, driveStateChangeTimeout,
			apiV1.HealthBad, "spec", "Health")

		// check events for volume
		eventsWaitTimeout := time.Second * 30
		checkExpectedEventsExistWithRetry(f, &targetVolume, []string{
			eventManager.GetReason(eventing.VolumeBadHealth),
		}, eventsWaitTimeout)
	})

	ginkgo.It("Check drive events", func() {
		defaultDriveCount := 3
		conf := &common.LoopBackManagerConfig{DefaultDriveCount: &defaultDriveCount}

		init(conf)
		defer cleanup()

		allDrives := getUObjList(f, common.DriveGVR)
		Expect(len(allDrives.Items) > 2).To(BeTrue())
		targetNodeID, _, err := unstructured.NestedString(
			allDrives.Items[0].UnstructuredContent(), "spec", "NodeId")
		framework.ExpectNoError(err)
		targetNodeName, err := findNodeNameByUID(f, targetNodeID)
		framework.ExpectNoError(err)

		driveCRsForNode := filterDrivesCRsForNode(targetNodeID, allDrives)
		Expect(len(driveCRsForNode) > 2).To(BeTrue())

		driveUnderTest1 := driveCRsForNode[0]
		driveUnderTest1SN, _, _ := unstructured.NestedString(
			driveUnderTest1.Object, "spec", "SerialNumber")
		driveUnderTest1Name, _, _ := unstructured.NestedString(
			driveUnderTest1.Object, "metadata", "name")

		driveUnderTest2 := driveCRsForNode[1]
		driveUnderTest2SN, _, _ := unstructured.NestedString(
			driveUnderTest2.Object, "spec", "SerialNumber")
		driveUnderTest2Name, _, _ := unstructured.NestedString(
			driveUnderTest2.Object, "metadata", "name")

		// switch driveUnderTest1 health to "BAD"
		// switch driveUnderTest2 status to "OFFLINE"
		badHealth := apiV1.HealthBad
		driveRemoved := true
		conf.Nodes = []common.LoopBackManagerConfigNode{{
			NodeID: &targetNodeName,
			Drives: []common.LoopBackManagerConfigDevice{{
				SerialNumber: &driveUnderTest1SN,
				Health:       &badHealth,
			}, {
				SerialNumber: &driveUnderTest2SN,
				Removed:      &driveRemoved,
			}},
		}}
		applyLMConfig(f, conf)
		waitForObjStateChange(f, common.DriveGVR, driveUnderTest1Name, driveStateChangeTimeout,
			apiV1.HealthBad, "spec", "Health")
		waitForObjStateChange(f, common.DriveGVR, driveUnderTest2Name, driveStateChangeTimeout,
			apiV1.DriveStatusOffline, "spec", "Status")

		// switch driveUnderTest1 health to "GOOD"
		// switch driveUnderTest2 status to "ONLINE"
		goodHealth := apiV1.HealthGood
		// driveUnderTest1
		conf.Nodes[0].Drives[0].Health = &goodHealth
		// driveUnderTest2
		conf.Nodes[0].Drives[1].Removed = nil
		applyLMConfig(f, conf)
		waitForObjStateChange(f, common.DriveGVR, driveUnderTest1Name, driveStateChangeTimeout,
			apiV1.HealthGood, "spec", "Health")
		waitForObjStateChange(f, common.DriveGVR, driveUnderTest2Name, driveStateChangeTimeout,
			apiV1.DriveStatusOnline, "spec", "Status")

		// check events
		eventsWaitTimeout := time.Second * 30
		checkExpectedEventsExistWithRetry(f, &driveUnderTest1, []string{
			eventManager.GetReason(eventing.DriveDiscovered),
			eventManager.GetReason(eventing.DriveHealthGood),
			eventManager.GetReason(eventing.DriveHealthFailure),
			eventManager.GetReason(eventing.DriveHealthGood),
		}, eventsWaitTimeout)
		checkExpectedEventsExistWithRetry(f, &driveUnderTest2, []string{
			eventManager.GetReason(eventing.DriveDiscovered),
			eventManager.GetReason(eventing.DriveHealthGood),
			eventManager.GetReason(eventing.DriveStatusOffline),
			eventManager.GetReason(eventing.DriveStatusOnline),
		}, eventsWaitTimeout)
	})
}

// constructPVC constructs pvc for test purposes
// Receives PVC size and namespace
func constructPVC(ns string, claimSize string, storageClass string, pvcName string) *corev1.PersistentVolumeClaim {
	claim := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: ns,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(claimSize),
				},
			},
			StorageClassName: &storageClass,
		},
	}

	return &claim
}

// findNodeNameByUID finds node name according to its k8s uid
// Receives k8s test framework and node uid
// Returns node name or error if something went wrong
func findNodeNameByUID(f *framework.Framework, nodeUID string) (string, error) {
	nodeList, err := f.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	var nodeName string
	for _, node := range nodeList.Items {
		currID, _ := node.GetAnnotations()[akey.DeafultNodeIDAnnotationKey]

		if currID == nodeUID {
			nodeName = node.Name
			break
		}
	}
	return nodeName, nil
}

func applyLMConfig(f *framework.Framework, lmConf *common.LoopBackManagerConfig) {
	ns := f.Namespace.Name
	lmConfigMap, err := common.BuildLoopBackManagerConfigMap(ns, cmName, *lmConf)
	framework.ExpectNoError(err)
	_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(context.TODO(), lmConfigMap, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		_, err = f.ClientSet.CoreV1().ConfigMaps(ns).Update(context.TODO(), lmConfigMap, metav1.UpdateOptions{})
	}
	framework.ExpectNoError(err)
}

func filterDrivesCRsForNode(nodeID string, drives *unstructured.UnstructuredList) []unstructured.Unstructured {
	var filtered []unstructured.Unstructured

	for _, d := range drives.Items {
		v, _, err := unstructured.NestedString(d.UnstructuredContent(), "spec", "NodeId")
		framework.ExpectNoError(err)
		if v == nodeID {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

func getUObjList(f *framework.Framework, resource schema.GroupVersionResource) *unstructured.UnstructuredList {
	var namespace = ""
	if resource == common.VolumeGVR {
		namespace = f.Namespace.Name
	}
	drivesU, err := f.DynamicClient.Resource(resource).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	framework.ExpectNoError(err)
	return drivesU
}

func getUObj(f *framework.Framework, resource schema.GroupVersionResource, name string) (*unstructured.Unstructured, bool) {
	var namespace = ""
	if resource == common.VolumeGVR {
		namespace = f.Namespace.Name
	}
	driveU, err := f.DynamicClient.Resource(resource).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, false
		}
		framework.ExpectNoError(err)
	}
	return driveU, true
}

func waitForObjStateChange(f *framework.Framework, resource schema.GroupVersionResource, name string,
	timeout time.Duration, expectedValue string, fields ...string) {

	deadline := time.Now().Add(timeout)
	for {
		drive, found := getUObj(f, resource, name)
		var result string
		if found {
			result, _, err := unstructured.NestedString(drive.Object, fields...)
			framework.ExpectNoError(err)
			if result == expectedValue {
				e2elog.Logf("%s %s in expected state: %s", resource.Resource, name, expectedValue)
				return
			}
		}
		// check timeout
		if time.Now().After(deadline) {
			if found {
				framework.Failf("%s %s doesn't change to expected state: %s, current state: %s",
					resource.Resource, name, expectedValue, result)
			} else {
				framework.Failf("Resource %s name %s not found", resource, name)
			}
		}
		// sleep before next try
		time.Sleep(time.Second * 5)
	}
}

func checkExpectedEventsExistWithRetry(f *framework.Framework, object runtime.Object,
	eventsReasons []string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			framework.Failf("expected events not found")
		}
		if checkExpectedEventsExist(f, object, eventsReasons) {
			return
		}
		time.Sleep(time.Second * 5)
	}
}

func checkExpectedEventsExist(f *framework.Framework, object runtime.Object, eventsReasons []string) bool {
	evlist, err := f.ClientSet.CoreV1().Events("").Search(runtime.NewScheme(), object)
	framework.ExpectNoError(err)
	events := evlist.Items

	for _, er := range eventsReasons {
		var found bool
		for i := 0; i < len(events); i++ {
			if events[i].Reason == er {
				found = true
				// remove matched event
				events[i] = events[len(events)-1]
				events[len(events)-1] = corev1.Event{}
				events = events[:len(events)-1]
				break
			}
		}
		if !found {
			e2elog.Logf("expected event not found: %s", er)
			return false
		}
	}
	e2elog.Logf("all expected events found")
	return true
}
