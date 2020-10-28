package scenarios

import (
	"fmt"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"time"

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
		executor      = &command.Executor{}
		logger        = logrus.New()
		driverCleanup func()
		ns            string
		kindNodeContainer  string // represents kind node
		started       = false
		f             = framework.NewDefaultFramework("node-reboot")
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

		// save config of Drives on that node
		// change Loopback mgr config
		// restart baremetal-csi-plugin-node
		// ensure it ready

		// delete node and add it again
		cmd := fmt.Sprintf("/tmp/delete_add_node.sh %s %s", kindNodeContainer, "kind-control-plane")
		stdOut, stdErr, err := executor.RunCmd(cmd)
		e2elog.Logf("Results of delete_add_node.sh script. STDOUT: %v. STDERR: %v", stdOut, stdErr)
		framework.ExpectNoError(err)

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
