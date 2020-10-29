package scenarios

import (
	"fmt"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/test/e2e/common"
)

func DefineNodeReplacementTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("CSI-Baremetal Node Replacement test suite", func() {
		nrTest(driver)
	})
}

func nrTest(driver testsuites.TestDriver) {
	var (
		pod               *corev1.Pod
		pvc               *corev1.PersistentVolumeClaim
		k8sSC             *storagev1.StorageClass
		executor          = &command.Executor{}
		logger            = logrus.New()
		driverCleanup     func()
		ns                string
		kindNodeContainer string // represents kind node
		f                 = framework.NewDefaultFramework("node-reboot")
	)
	logger.SetLevel(logrus.DebugLevel)
	executor.SetLogger(logger)

	init := func() {
		var (
			perTestConf *testsuites.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name

		perTestConf, driverCleanup = driver.PrepareTest(f)
		k8sSC = driver.(*baremetalDriver).GetStorageClassWithStorageType(perTestConf, storageClassHDD)
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(k8sSC)
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test NodeReplacement")

		// TODO: handle case when node wasn't added

		common.CleanupAfterCustomTest(f, driverCleanup, []*corev1.Pod{pod}, []*corev1.PersistentVolumeClaim{pvc})
	}

	ginkgo.Context("Pod should consume same PV after node had being replaced", func() {
		init()
		defer cleanup()

		var err error
		// create pvc
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).
			Create(constructPVC(ns, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), k8sSC.Name, pvcName))
		framework.ExpectNoError(err)

		// create pod with pvc
		pod, err = e2epod.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		e2elog.Logf("Pod %s with PVC %s created.", pod.Name, pvc.Name)

		// delete pod
		err = e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, pod.Name, f.Namespace.Name, e2epod.PodDeleteTimeout)
		framework.ExpectNoError(err)

		// since test is run in Kind k8s cluster, each node is represented by docker container
		// node' name is the same as a docker container name by which this node is represented.
		kindNodeContainer = pod.Spec.NodeName

		// find node UID that is used as a part of Drive.Spec.NodeID
		nodes, err := e2enode.GetReadySchedulableNodesOrDie(f.ClientSet)
		framework.ExpectNoError(err)
		var nodeID string
		for _, node := range nodes.Items {
			if node.Name == pod.Spec.NodeName {
				nodeID = string(node.UID)
				break
			}
		}
		if nodeID == "" {
			framework.Failf("Unable to find UID for node %s", pod.Spec.NodeName)
		}

		// save config of Drives on that node
		allDrivesUnstr := getUObjList(f, common.DriveGVR)
		volumeUnstr, found := getUObj(f, common.VolumeGVR, pvc.Name)
		if !found {
			framework.Failf("Unable to found PVC with name %s. Pod spec is: %v", pvc.Name, pod.Spec)
		}
		volumeLocation, _, err := unstructured.NestedString(volumeUnstr.Object, "spec", "Location")
		framework.ExpectNoError(err)

		// found drive that corresponds to volumeCR
		driveOnNode := make([]common.LoopBackManagerConfigDevice, 0)
		bdev := ""
		provisionedDriveSN := ""
		for _, drive := range allDrivesUnstr.Items {
			n, _, err := unstructured.NestedString(drive.Object, "spec", "NodeId")
			framework.ExpectNoError(err)
			if n == nodeID {
				sn, _, err := unstructured.NestedString(drive.Object, "spec", "SerialNumber")
				framework.ExpectNoError(err)
				driveOnNode = append(driveOnNode, common.LoopBackManagerConfigDevice{SerialNumber: &sn})
				path, _, err := unstructured.NestedString(drive.Object, "spec", "Path")
				framework.ExpectNoError(err)
				e2elog.Logf("Append drive with SN %s and path %s", sn, path)
				if sn == volumeLocation {
					e2elog.Logf("PVC %s location is drive with SN %s and path %s", sn, path)
					bdev = path
					provisionedDriveSN = sn
				}
			}
		}
		if len(driveOnNode) == 0 {
			framework.Failf("Unable to detect which DriveCRs correspond to node %s (%s)", pod.Spec.NodeName, nodeID)
		}

		// change Loopback mgr config
		lmConfig := &common.LoopBackManagerConfig{
			Nodes: []common.LoopBackManagerConfigNode{
				{
					NodeID: &pod.Spec.NodeName,
					Drives: driveOnNode,
				},
			},
		}
		applyLMConfig(f, lmConfig)

		// delete node and add it again
		cmd := fmt.Sprintf("/tmp/delete_add_node.sh %s %s", kindNodeContainer, "kind-control-plane")
		stdOut, stdErr, err := executor.RunCmd(cmd)
		e2elog.Logf("Results of delete_add_node.sh script. STDOUT: %v. STDERR: %v", stdOut, stdErr)
		framework.ExpectNoError(err)

		// by drive.spec.serialNumber find drive.spec.path -> newBDev
		// read config from bdev and apply it for newBDev
		allDrivesUnstr = getUObjList(f, common.DriveGVR)
		for _, drive := range allDrivesUnstr.Items {
			sn, _, err := unstructured.NestedString(drive.Object, "spec", "SerialNumber")
			framework.ExpectNoError(err)
			if sn == provisionedDriveSN {
				newBDev, _, err := unstructured.NestedString(drive.Object, "spec", "Path")
				framework.ExpectNoError(err)
				framework.Logf("Found drive with SN %s. bdev - %s, newBDev - %s", bdev, newBDev)
				if bdev == newBDev {
					break
				}
				err = common.CopyPartitionConfig(
					bdev,
					strings.Replace(pvc.Name, "pvc-", "", 1),
					newBDev,
					logger)
				if err != nil {
					e2elog.Failf("CopyPartitionConfig finished with error: %v", err)
				}
				e2elog.Logf("Partition was restored successfully from %s to %s", bdev, newBDev)
				break
			}
		}

		// create pod again
		pod, err = e2epod.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		// check that pod consume same pvc
		var boundAgain = false
		pods, err := e2epod.GetPodsInNamespace(f.ClientSet, f.Namespace.Name, map[string]string{})
		framework.ExpectNoError(err)

		// search pod again
		for _, p := range pods {
			if p.Name == pod.Name {
				// search volumes
				volumes := p.Spec.Volumes
				for _, v := range volumes {
					if v.PersistentVolumeClaim.ClaimName == pvc.Name {
						boundAgain = true
						break
					}
				}
				break
			}
		}
		e2elog.Logf("Pod has same PVC: %v", boundAgain)
		framework.ExpectEqual(boundAgain, true)
	})
}
