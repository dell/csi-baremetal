package scenarios

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/test/e2e/common"
)

// DefineNodeRebootTestSuite defines custom baremetal-csi node reboot test
func DefineNodeRebootTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi node reboot test", func() {
		defineNodeRebootTest(driver)
	})
}

func defineNodeRebootTest(driver testsuites.TestDriver) {
	var (
		pod             *corev1.Pod
		pvc             *corev1.PersistentVolumeClaim
		k8sSC           *storagev1.StorageClass
		executor        = &command.Executor{}
		driverCleanup   func()
		ns              string
		containerToStop string
		started         = false
		f               = framework.NewDefaultFramework("node-reboot")
	)
	executor.SetLogger(logrus.New())

	init := func() {
		var (
			perTestConf *testsuites.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name

		perTestConf, driverCleanup = driver.PrepareTest(f)
		k8sSC = driver.(*baremetalDriver).GetDynamicProvisionStorageClass(perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(k8sSC)
		framework.ExpectNoError(err)

		// waiting for baremetal csi pods become ready
		err = e2epod.WaitForPodsRunningReady(f.ClientSet, ns, 2, 0, 90*time.Second, nil)
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test NodeReboot")

		if containerToStop != "" && !started {
			_, _, err := executor.RunCmd(fmt.Sprintf("docker start %s", containerToStop))
			framework.ExpectNoError(err)
		}

		common.CleanupAfterCustomTest(f, driverCleanup, []*corev1.Pod{pod}, []*corev1.PersistentVolumeClaim{pvc})
	}

	ginkgo.It("Pod should consume same PVC after node with it was rebooted", func() {
		framework.Skipf("Skip node reboot test, see ATLDEF-93")
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

		// since test is run in Kind k8s cluster, each node is represented by docker container
		// node' name is the same as a docker container name by which this node is represented.
		containerToStop = pod.Spec.NodeName

		// stop container
		cmd := fmt.Sprintf("docker stop %s", containerToStop)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		// wait up to 5 minutes until node that is located on containerToStop become NotReady
		nodeNotReady := e2enode.WaitForNodeToBeNotReady(f.ClientSet, containerToStop, time.Minute*5)
		if !nodeNotReady {
			framework.Failf("Node %s still ready", containerToStop)
		}

		// start container
		cmd = fmt.Sprintf("docker start %s", containerToStop)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)
		started = true

		// wait up to 5 minutes until node that is located on containerToStop become Ready
		nodeReady := e2enode.WaitForNodeToBeReady(f.ClientSet, containerToStop, time.Minute*5)
		if !nodeReady {
			framework.Failf("Node %s still NotReady", containerToStop)
		}

		// wait until pod become ready
		podReadyErr := e2epod.WaitForPodsReady(f.ClientSet, ns, pod.Name, 2)
		framework.ExpectNoError(podReadyErr)
		e2elog.Logf("Pod %s became ready again", pod.Name)

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
