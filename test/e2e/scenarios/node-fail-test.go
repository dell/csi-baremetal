package scenarios

import (
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edep "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	pode2e "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

const ControllerName = "baremetal-csi-controller"

func DefineNodeFailTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi node fail tests", func() {
		nodeFailTest(driver)
	})
}

func nodeFailTest(driver testsuites.TestDriver) {
	var (
		pod           *corev1.Pod
		pvc           *corev1.PersistentVolumeClaim
		driverCleanup func()
		nodeName      string
		perTestConf   *testsuites.PerTestConfig
		k8sSC         *v12.StorageClass
	)

	f := framework.NewDefaultFramework("node-fail")

	init := func() {
		perTestConf, driverCleanup = driver.PrepareTest(f)
		k8sSC = driver.(*baremetalDriver).GetDynamicProvisionStorageClass(perTestConf, "xfs")
		var err error
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(k8sSC)
		framework.ExpectNoError(err)
	}

	// This function deletes pod if it was installed during test. And waits for its correct deletion to perform
	// NodeUnpublish and NodeUnstage properly. Next it deletes PVC and waits for correctly deletion of bounded PV
	// to clear device for next tests (CSI performs wipefs during PV deletion). The last step is the deletion of driver.
	cleanup := func() {
		ns := f.Namespace.Name
		//try to make node ready again
		err := execDockerCmd("exec", nodeName, "systemctl", "start", "kubelet.service")
		framework.ExpectNoError(err)

		if pod != nil {
			_ = pode2e.DeletePodWithWait(f.ClientSet, pod)
		}

		if pvc != nil {
			pv, _ := framework.GetBoundPV(f.ClientSet, pvc)
			err := framework.DeletePersistentVolumeClaim(f.ClientSet, pvcName, ns)
			if err != nil {
				e2elog.Logf("failed to delete pvc %v", err)
			}
			if pv != nil {
				//Wait for pv deletion to clear devices for future tests
				_ = framework.WaitForPersistentVolumeDeleted(f.ClientSet, pv.Name, 5*time.Second, 2*time.Minute)
			}
		}

		// Removes all driver's manifests installed during init(). (Driver, its RBACs, SC)
		if driverCleanup != nil {
			driverCleanup()
			driverCleanup = nil
		}
	}

	ginkgo.It("controller should keep handle request after node fails", func() {
		init()
		defer cleanup()

		ns := f.Namespace.Name
		//waiting for baremetal csi pods become ready
		err := pode2e.WaitForPodsRunningReady(f.ClientSet, ns, 2, 0, 90*time.Second, nil)
		framework.ExpectNoError(err)

		deployment, err := f.ClientSet.AppsV1().Deployments(ns).Get(ControllerName, metav1.GetOptions{})
		Expect(deployment).ToNot(BeNil())

		//try to find baremetal-csi-controller pod, expect 1 controller pod
		podList, err := e2edep.GetPodsForDeployment(f.ClientSet, deployment)
		framework.ExpectNoError(err)
		Expect(podList).ToNot(BeNil())
		Expect(len(podList.Items)).To(Equal(1))

		controller := &podList.Items[0]
		nodeName = controller.Spec.NodeName

		//try to make node NotReady by kubelet stop on docker node, where controller pod is running
		err = execDockerCmd("exec", nodeName, "systemctl", "stop", "kubelet.service")
		framework.ExpectNoError(err)

		//wait 5 minutes until node with controller become NotReady
		nodeNotReady := e2enode.WaitForNodeToBeNotReady(f.ClientSet, nodeName, time.Minute*5)
		if !nodeNotReady {
			framework.Failf("Node %s still ready", nodeName)
		}

		//waiting for the new controller pod to appear in cluster and become ready for 15 minute
		var found bool
		for start := time.Now(); time.Since(start) < time.Minute*15; time.Sleep(time.Second * 30) {
			podList, err := e2edep.GetPodsForDeployment(f.ClientSet, deployment)
			framework.ExpectNoError(err)
			for _, item := range podList.Items {
				e2elog.Logf("Pod %s with status %s", item.Name, string(item.Status.Phase))
				if item.Status.Phase == corev1.PodRunning && item.Name != controller.Name {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			framework.Failf("Controller is not ready")
		}

		//check if CSI controller keep handle requests
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).
			Create(constructPVC(ns, driver.(testsuites.DynamicPVTestDriver).GetClaimSize(), k8sSC.Name))
		framework.ExpectNoError(err)

		pod, err = pode2e.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		err = f.WaitForPodRunning(pod.Name)
		framework.ExpectNoError(err)

	})
}

//execDockerCmd run docker command on host
func execDockerCmd(args ...string) error {
	cmd := exec.Command("docker", args...)
	_, err := cmd.Output()
	return err
}
