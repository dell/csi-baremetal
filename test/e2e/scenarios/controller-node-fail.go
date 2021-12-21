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
	"time"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2edep "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal/test/e2e/common"
)

const (
	ControllerName = "csi-baremetal-controller"
	ContextTimeout = 20 * time.Minute
)

func DefineControllerNodeFailTestSuite(driver testsuites.TestDriver) {
	ginkgo.Context("Baremetal-csi controller node fail tests", func() {
		controllerNodeFailTest(driver)
	})
}

func controllerNodeFailTest(driver testsuites.TestDriver) {
	ginkgo.BeforeEach(skipIfNotAllTests)

	var (
		pod           *corev1.Pod
		pvc           *corev1.PersistentVolumeClaim
		k8sSC         *storagev1.StorageClass
		executor      = common.GetExecutor()
		driverCleanup func()
		ctx           context.Context
		//cancel		  func()
		nodeName string
		ns       string
		f        = framework.NewDefaultFramework("controller-node-fail")
	)

	init := func() {
		var (
			perTestConf *testsuites.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name

		perTestConf, driverCleanup = driver.PrepareTest(f)

		// TODO get rid of TODO context https://github.com/dell/csi-baremetal/issues/556
		//ctx, cancel = context.WithTimeout(context.Background(), ContextTimeout)
		ctx = context.Background()
		k8sSC = driver.(*baremetalDriver).GetDynamicProvisionStorageClass(perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, k8sSC, metav1.CreateOptions{})
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test ControllerNodeFail")

		// try to make node ready again
		cmd := fmt.Sprintf("docker exec %s systemctl start kubelet.service", nodeName)
		_, _, err := executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		common.CleanupAfterCustomTest(f, driverCleanup, []*corev1.Pod{pod}, []*corev1.PersistentVolumeClaim{pvc})
	}

	ginkgo.It("controller should keep handle request after node fails", func() {
		init()
		//defer cancel()
		defer cleanup()

		deployment, err := f.ClientSet.AppsV1().Deployments(ns).Get(ctx, ControllerName, metav1.GetOptions{})
		Expect(deployment).ToNot(BeNil())

		// try to find csi-baremetal-controller pod, expect 1 controller pod
		podList, err := e2edep.GetPodsForDeployment(f.ClientSet, deployment)
		framework.ExpectNoError(err)
		Expect(podList).ToNot(BeNil())
		Expect(len(podList.Items)).To(Equal(1))

		controller := &podList.Items[0]
		nodeName = controller.Spec.NodeName
		controllerPodName := controller.Name

		// try to make node NotReady by kubelet stop on docker node, where controller pod is running
		cmd := fmt.Sprintf("docker exec %s systemctl stop kubelet.service", nodeName)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		// wait 5 minutes until node with controller become NotReady
		nodeNotReady := e2enode.WaitForNodeToBeNotReady(f.ClientSet, nodeName, time.Minute*5)
		if !nodeNotReady {
			framework.Failf("Node %s still ready", nodeName)
		}

		// to speed up failover delete pod
		e2elog.Logf("Deleting pod %s...", controllerPodName)
		err = f.ClientSet.CoreV1().Pods(ns).Delete(ctx, controllerPodName, metav1.DeleteOptions{})
		framework.ExpectNoError(err)

		// waiting for the new controller pod to appear in cluster and become ready for 15 minute
		var found bool
		e2elog.Logf("Waiting to controller pod to run on another node...")
		for start := time.Now(); time.Since(start) < time.Minute*15; time.Sleep(time.Second * 30) {
			podList, err := e2edep.GetPodsForDeployment(f.ClientSet, deployment)
			framework.ExpectNoError(err)
			for _, item := range podList.Items {
				e2elog.Logf("Pod %s with status %s", item.Name, string(item.Status.Phase))
				if item.Status.Phase == corev1.PodRunning && item.Name != controllerPodName {
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

		// check if CSI controller keep handle requests
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(ctx,
			constructPVC(ns, persistentVolumeClaimSize, k8sSC.Name, pvcName),
			metav1.CreateOptions{})
		framework.ExpectNoError(err)

		pod, err = common.CreatePod(f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		e2elog.Logf("Waiting for test pod %s to be in running state...", pod.Name)
		err = f.WaitForPodRunning(pod.Name)
		framework.ExpectNoError(err)
	})
}
