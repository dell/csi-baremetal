package scenarios

import (
	"fmt"
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
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/test/e2e/common"
)

var (
	driveGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "drives",
	}

	acGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "availablecapacities",
	}

	volumeGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "volumes",
	}

	pvcName   = "baremetal-csi-pvc"
	configKey = "config.yaml"
)

// DefineDriveHealthChangeTestSuite defines custom baremetal-csi e2e tests
func DefineDriveHealthChangeTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi drive health change tests", func() {
		// It consists of two steps.
		// 1) Set random drive to Failed state and see that amount of ACs reduced by 1.
		// 2) Install pod with pvc. Set drive which is used by pvc to Failed state. See that appropriate VolumeCR
		// changes its status too
		driveHealthChangeTest(driver)
	})
}

// driveHealthChangeTest test checks behavior of driver when drives change health from GOOD to BAD
func driveHealthChangeTest(driver testsuites.TestDriver) {
	var (
		testPODs      []*corev1.Pod
		testPVCs      []*corev1.PersistentVolumeClaim
		k8sSC         *storagev1.StorageClass
		driverCleanup func()
		ns            string
		f             = framework.NewDefaultFramework("health")
	)

	init := func(lmConf *common.LoopBackManagerConfig) {
		var (
			perTestConf *testsuites.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name

		if lmConf != nil {
			applyLMConfig(f, ns, lmConf)
		}

		perTestConf, driverCleanup = driver.PrepareTest(f)

		k8sSC = driver.(*baremetalDriver).GetDynamicProvisionStorageClass(perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(k8sSC)
		framework.ExpectNoError(err)

		// wait for csi pods to be running and ready
		err = e2epod.WaitForPodsRunningReady(f.ClientSet, ns, 2, 0, 90*time.Second, nil)
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test DriveHealthChange")
		common.CleanupAfterCustomTest(f, driverCleanup, testPODs, testPVCs)
	}

	ginkgo.It("should discover drives' health changes and delete ac or change volume health", func() {
		init(nil)
		defer cleanup()

		// Get ACs from the cluster
		acUnstructuredList, err := f.DynamicClient.Resource(acGVR).Namespace(ns).List(metav1.ListOptions{})
		framework.ExpectNoError(err)

		// Save amount of ACs before drive's health changing
		amountOfACBeforeDiskFailure := len(acUnstructuredList.Items)
		e2elog.Logf("found %d ac", amountOfACBeforeDiskFailure)

		// Prepare variables to find serialNumber of drive which should be unhealthy
		drivesUnstructuredList, _ := f.DynamicClient.Resource(driveGVR).Namespace(ns).List(metav1.ListOptions{})
		acToDelete, _, err := unstructured.NestedString(acUnstructuredList.Items[0].Object, "spec", "Location")
		framework.ExpectNoError(err)
		nodeUidOfAC, _, err := unstructured.NestedString(acUnstructuredList.Items[0].Object, "spec", "NodeId")
		framework.ExpectNoError(err)
		nodeNameOfAC, err := findNodeNameByUID(f, nodeUidOfAC)
		framework.ExpectNoError(err)

		// Get current loopback-config from the cluster
		cm, err := f.ClientSet.CoreV1().ConfigMaps(ns).Get(cmName, metav1.GetOptions{})
		framework.ExpectNoError(err)
		// Append bad-health drive to this config and update config on the cluster side
		appendBadHealthDriveToConfig(cm, findSNByDriveLocation(drivesUnstructuredList.Items, acToDelete), nodeNameOfAC)
		cm, err = f.ClientSet.CoreV1().ConfigMaps(ns).Update(cm)
		framework.ExpectNoError(err)

		// k8s docs say that time from updating ConfigMap to receiving updated ConfigMap in the pod where it's mounted
		// could last 1 minute by default. Plus 30 seconds between Node's Discover in the worst case
		time.Sleep(90 * time.Second)

		// Read ACs one more time
		acUnstructuredList, err = f.DynamicClient.Resource(acGVR).Namespace(ns).List(metav1.ListOptions{})
		framework.ExpectNoError(err)

		// Check that amount of ACs reduced by one
		Expect(len(acUnstructuredList.Items)).To(Equal(amountOfACBeforeDiskFailure - 1))

		// Create test pvc on the cluster
		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).
			Create(constructPVC(ns, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), k8sSC.Name, pvcName))
		framework.ExpectNoError(err)

		// Create test pod that consumes the pvc
		pod, err := e2epod.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)
		testPVCs = append(testPVCs, pvc)
		testPODs = append(testPODs, pod)

		// Get Volume CRs and save variables to identify on which drive the pod's Volume based on
		volumesUnstructuredList, _ := f.DynamicClient.Resource(volumeGVR).List(metav1.ListOptions{})
		location, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "spec", "Location")
		framework.ExpectNoError(err)
		volumeName, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "metadata", "name")
		framework.ExpectNoError(err)
		nodeUidOfVolume, _, err := unstructured.NestedString(volumesUnstructuredList.Items[0].Object, "spec", "NodeId")
		framework.ExpectNoError(err)
		nodeNameOfVolume, err := findNodeNameByUID(f, nodeUidOfVolume)
		framework.ExpectNoError(err)

		// Make the drive of the Volume unhealthy
		appendBadHealthDriveToConfig(cm, findSNByDriveLocation(drivesUnstructuredList.Items, location), nodeNameOfVolume)
		cm, err = f.ClientSet.CoreV1().ConfigMaps(ns).Update(cm)
		framework.ExpectNoError(err)

		// k8s docs say that time from updating ConfigMap to receiving updated ConfigMap in the pod where it's mounted
		// could last 1 minute by default. Plus 30 seconds between Node's Discover in the worst case
		time.Sleep(90 * time.Second)

		// Read Volume one more time
		changedVolume, err := f.DynamicClient.Resource(volumeGVR).Namespace(ns).Get(volumeName, metav1.GetOptions{})
		framework.ExpectNoError(err)
		health, _, err := unstructured.NestedString(changedVolume.Object, "spec", "Health")
		//get events on volume
		evlist, err := f.ClientSet.CoreV1().Events(ns).Search(runtime.NewScheme(), changedVolume)
		framework.ExpectNoError(err)

		bhEvents := filterEventsByReason(evlist, eventing.VolumeBadHealth)
		e2elog.Logf("found bad health events %+v len %d\n", bhEvents, len(bhEvents))
		// Check that Volume is marked as unhealthy
		Expect(health).To(Equal(apiV1.HealthBad))
		// CHeck we have Bad Health Event
		Expect(len(bhEvents)).To(Equal(1))
	})
	ginkgo.It("Check drive events", func() {
		nodes, err := e2enode.GetReadySchedulableNodesOrDie(f.ClientSet)
		framework.ExpectNoError(err)

		node := nodes.Items[0]
		defaultDriveCount := 0
		nodeDriveCount := 3

		// initial LM config
		// one node has 3 drives
		conf := &common.LoopBackManagerConfig{
			DefaultDriveCount: &defaultDriveCount,
			Nodes: []common.LoopBackManagerConfigNode{{
				NodeID:     &node.ObjectMeta.Name,
				DriveCount: &nodeDriveCount},
			}}

		init(conf)
		defer cleanup()

		driveCRs := filterDrivesCRsForNode(string(node.ObjectMeta.GetUID()), getDrivesList(f))
		Expect(len(driveCRs) > 2).To(BeTrue())

		driveUnderTest1, driveUnderTest2 := driveCRs[0], driveCRs[1]

		driveStateChangeTimeout := time.Minute * 3

		// switch driveUnderTest1 health to "BAD"
		// switch driveUnderTest2 status to "OFFLINE"
		badHealth := apiV1.HealthBad
		driveRemoved := true
		conf.Nodes[0].Drives = append(conf.Nodes[0].Drives, common.LoopBackManagerConfigDevice{
			SerialNumber: &driveUnderTest1.Spec.SerialNumber,
			Health:       &badHealth,
		}, common.LoopBackManagerConfigDevice{
			SerialNumber: &driveUnderTest2.Spec.SerialNumber,
			Removed:      &driveRemoved,
		})
		applyLMConfig(f, ns, conf)
		waitForDriveHealthChange(f, driveUnderTest1.Name, apiV1.HealthBad, driveStateChangeTimeout)
		waitForDriveStatusChange(f, driveUnderTest2.Name, apiV1.DriveStatusOffline, driveStateChangeTimeout)

		// switch driveUnderTest1 health to "GOOD"
		// switch driveUnderTest2 status to "ONLINE"
		goodHealth := apiV1.HealthGood
		// driveUnderTest1
		conf.Nodes[0].Drives[0].Health = &goodHealth
		// driveUnderTest2
		conf.Nodes[0].Drives[1].Removed = nil
		applyLMConfig(f, ns, conf)
		waitForDriveHealthChange(f, driveUnderTest1.Name, apiV1.HealthGood, driveStateChangeTimeout)
		waitForDriveStatusChange(f, driveUnderTest2.Name, apiV1.DriveStatusOnline, driveStateChangeTimeout)

		// check events
		checkExpectedEventsExist(f, &driveUnderTest1, []string{
			eventing.DriveDiscovered,
			eventing.DriveHealthGood,
			eventing.DriveHealthFailure,
			eventing.DriveHealthGood,
		})
		checkExpectedEventsExist(f, &driveUnderTest2, []string{
			eventing.DriveDiscovered,
			eventing.DriveHealthGood,
			eventing.DriveStatusOffline,
			eventing.DriveStatusOnline,
		})
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

// appendBadHealthDriveToConfig appends spec of bad-health drive to LoopBackMgr's ConfigMap
// Receives current state of ConfigMap, serialNumber of drive to make it unhealthy, nodeID where the drive is placed
func appendBadHealthDriveToConfig(cm *corev1.ConfigMap, serialNumber string, nodeID string) {
	cm.Data[configKey] = cm.Data[configKey] + fmt.Sprintf("- nodeID: %s\n", nodeID) +
		"  drives:\n" +
		fmt.Sprintf("  - serialNumber: %s\n", serialNumber) +
		"    health: BAD\n"
}

// findSNByDriveLocation finds SerialNumber of the drive which is used by the volume using its location
// Receives unstructured list of drives and location
func findSNByDriveLocation(driveList []unstructured.Unstructured, driveLocation string) string {
	for _, unstrDrive := range driveList {
		name, _, _ := unstructured.NestedString(unstrDrive.Object, "metadata", "name")
		if name == driveLocation {
			sn, _, _ := unstructured.NestedString(unstrDrive.Object, "spec", "SerialNumber")
			return sn
		}
	}
	return ""
}

// findNodeNameByUID finds node name according to its k8s uid
// Receives k8s test framework and node uid
// Returns node name or error if something went wrong
func findNodeNameByUID(f *framework.Framework, nodeUID string) (string, error) {
	nodeList, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	var nodeName string
	for _, node := range nodeList.Items {
		if string(node.UID) == nodeUID {
			nodeName = node.Name
			break
		}
	}
	return nodeName, nil
}

func filterEventsByReason(eventlist *corev1.EventList, reason string) []corev1.Event {
	events := make([]corev1.Event, 0)
	for i := range eventlist.Items {
		if eventlist.Items[i].Reason == reason {
			events = append(events, eventlist.Items[i])
		}
	}
	return events
}

func applyLMConfig(f *framework.Framework, namespace string, lmConf *common.LoopBackManagerConfig) {
	lmConfigMap, err := common.BuildLoopBackManagerConfigMap(namespace, cmName, *lmConf)
	framework.ExpectNoError(err)
	_, err = f.ClientSet.CoreV1().ConfigMaps(namespace).Create(lmConfigMap)
	if errors.IsAlreadyExists(err) {
		_, err = f.ClientSet.CoreV1().ConfigMaps(namespace).Update(lmConfigMap)
	}
	framework.ExpectNoError(err)
}

func filterDrivesCRsForNode(nodeID string, driveList drivecrd.DriveList) []drivecrd.Drive {
	var filtered []drivecrd.Drive

	for _, d := range driveList.Items {
		if d.Spec.NodeId == nodeID {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

func getDrivesList(f *framework.Framework) drivecrd.DriveList {
	drivesU, err := f.DynamicClient.Resource(driveGVR).Namespace(f.Namespace.Name).List(metav1.ListOptions{})
	framework.ExpectNoError(err)
	driveList := drivecrd.DriveList{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(drivesU.UnstructuredContent(), &driveList)
	framework.ExpectNoError(err)
	return driveList
}

func getDrive(f *framework.Framework, name string) (drivecrd.Drive, bool) {
	driveU, err := f.DynamicClient.Resource(driveGVR).Namespace(f.Namespace.Name).Get(name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return drivecrd.Drive{}, false
		}
		framework.ExpectNoError(err)
	}
	drive := drivecrd.Drive{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(driveU.UnstructuredContent(), &drive)
	framework.ExpectNoError(err)
	return drive, true
}

func waitForDriveHealthChange(f *framework.Framework, name, expectedHealth string, timeout time.Duration) {
	waitForDriveStateChange(f, name, timeout, func(drive drivecrd.Drive) bool {
		return drive.Spec.Health == expectedHealth
	})
}

func waitForDriveStatusChange(f *framework.Framework, name, expectedStatus string, timeout time.Duration) {
	waitForDriveStateChange(f, name, timeout, func(drive drivecrd.Drive) bool {
		return drive.Spec.Status == expectedStatus
	})
}

func waitForDriveStateChange(f *framework.Framework, name string,
	timeout time.Duration, checkFunc func(drive drivecrd.Drive) bool) {

	deadline := time.Now().Add(timeout)
	for {
		drive, found := getDrive(f, name)
		if !found {
			continue
		}
		if checkFunc(drive) {
			return
		}
		if time.Now().After(deadline) {
			framework.Failf("drive doesn't change to expected state")
		}
		time.Sleep(time.Second * 5)
	}
}

func checkExpectedEventsExist(f *framework.Framework, object runtime.Object, eventsReasons []string) {
	evlist, err := f.ClientSet.CoreV1().Events(f.Namespace.Name).Search(runtime.NewScheme(), object)
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
			framework.Failf("expected event not found: %s", er)
		}

	}
}
