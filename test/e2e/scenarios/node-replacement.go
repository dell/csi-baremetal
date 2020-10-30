package scenarios

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal/pkg/base"
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
		pod           *corev1.Pod
		pvc           *corev1.PersistentVolumeClaim
		k8sSC         *storagev1.StorageClass
		driverCleanup func()
		ns            string
		nodeToReplace string // represents kind node
		f             = framework.NewDefaultFramework("node-replacement")
	)

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

	ginkgo.It("Pod should consume same PV after node had being replaced", func() {
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
		err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(pod.Name, nil)
		if err != nil {
			if !apierrs.IsNotFound(err) {
				framework.Failf("unable to delete pod %s: %v", pod.Name, err)
			}
		} else {
			err = e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, pod.Name, f.Namespace.Name, time.Minute*2)
			framework.ExpectNoError(err)
		}

		// since test is run in Kind k8s cluster, each node is represented by docker container
		// node' name is the same as a docker container name by which this node is represented.
		nodeToReplace = pod.Spec.NodeName

		// find node UID that is used as a part of Drive.Spec.NodeID and master node name
		nodes, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
		framework.ExpectNoError(err)
		var nodesCount = len(nodes.Items)
		e2elog.Logf("Got nodesCount: %d", nodesCount)
		var nodeID string
		var masterNodeName string
		for _, node := range nodes.Items {
			e2elog.Logf("Inspecting node %s with labels %v", node.Name, node.GetLabels())
			if node.Name == pod.Spec.NodeName {
				nodeID = string(node.UID)
			}
			if _, ok := node.GetLabels()["node-role.kubernetes.io/master"]; ok {
				masterNodeName = node.Name
				//nodesCount-- // we are interested only in worker nodes count
			}
		}
		if nodeID == "" {
			framework.Failf("Unable to find UID for node %s", pod.Spec.NodeName)
		}

		e2elog.Logf("Master host is %s", masterNodeName)

		// save config of Drives on that node
		allDrivesUnstr := getUObjList(f, common.DriveGVR)
		createdPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Get(pvc.Name, metav1.GetOptions{})
		framework.ExpectNoError(err)
		e2elog.Logf("Read created PVC: %v", createdPVC)
		volumeUnstr, found := getUObj(f, common.VolumeGVR, createdPVC.Spec.VolumeName)
		if !found {
			framework.Failf("Unable to found Volume CR that corresponds to the PVC %s by name %s", pvc.Name, pvc.Spec.VolumeName)
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
				name, _, err := unstructured.NestedString(drive.Object, "metadata", "Name")
				framework.ExpectNoError(err)
				if name == volumeLocation {
					e2elog.Logf("PVC %s location is drive with SN %s, path %s, name %s", sn, path, name)
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
		e := &command.Executor{}
		_ = os.Setenv("LOG_FORMAT", "text")
		logger, err := base.InitLogger("", "debug")
		framework.ExpectNoError(err)
		e.SetLogger(logger)

		_, _, err = e.RunCmd(fmt.Sprintf("kubectl drain %s --ignore-daemonsets", nodeToReplace))
		framework.ExpectNoError(err)
		_, _, err = e.RunCmd(fmt.Sprintf("kubectl delete node %s", nodeToReplace))
		framework.ExpectNoError(err)
		_, _, err = e.RunCmd(fmt.Sprintf("docker exec -i %s kubeadm reset --force", nodeToReplace))
		framework.ExpectNoError(err)
		_, _, err = e.RunCmd(fmt.Sprintf("docker exec -i %s rm -rf /etc/kubernetes", nodeToReplace))
		framework.ExpectNoError(err)
		_, _, err = e.RunCmd(fmt.Sprintf("docker exec -i %s systemctl restart kubelet", nodeToReplace))
		framework.ExpectNoError(err)

		// wait until node will be removed from cluster
		shouldBe := nodesCount - 1
		_, err = e2enode.CheckReady(f.ClientSet, shouldBe, time.Minute*3)
		framework.ExpectNoError(err)

		var joinCommand string
		joinCommand, _, err = e.RunCmd(fmt.Sprintf("docker exec -i %s kubeadm token create --print-join-command", masterNodeName))
		framework.ExpectNoError(err)
		_, _, err = e.RunCmd(fmt.Sprintf("docker exec -i %s %s --ignore-preflight-errors=all", nodeToReplace, joinCommand))
		framework.ExpectNoError(err)

		_, err = e2enode.CheckReady(f.ClientSet, nodesCount, time.Minute*3)
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
