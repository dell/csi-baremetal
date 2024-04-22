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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"

	"github.com/dell/csi-baremetal-e2e-tests/e2e/common"
)

// DefineNodeRebootTestSuite defines custom csi-baremetal node reboot test
func DefineNodeRebootTestSuite(driver *baremetalDriver) {
	ginkgo.Context("Baremetal-csi node reboot test", func() {
		defineNodeRebootTest(driver)
	})
}

func defineNodeRebootTest(driver *baremetalDriver) {
	ginkgo.BeforeEach(skipIfNotAllTests)

	var (
		pod             *corev1.Pod
		pvc             *corev1.PersistentVolumeClaim
		k8sSC           *storagev1.StorageClass
		executor        = common.GetExecutor()
		ctx             context.Context
		ns              string
		containerToStop string
		started         = false
		f               = framework.NewDefaultFramework("node-reboot")
	)

	init := func() {
		var (
			perTestConf *storageframework.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name
		ctx = context.Background()

		perTestConf = driver.PrepareTest(ctx, f)
		k8sSC = driver.GetDynamicProvisionStorageClass(ctx, perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, k8sSC, metav1.CreateOptions{})
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		framework.Logf("Starting cleanup for test NodeReboot")

		if containerToStop != "" && !started {
			_, _, err := executor.RunCmd(fmt.Sprintf("docker start %s", containerToStop))
			framework.ExpectNoError(err)
		}

		common.CleanupAfterCustomTest(ctx, f, nil, []*corev1.Pod{pod}, []*corev1.PersistentVolumeClaim{pvc})
	}

	ginkgo.It("Pod should consume same PVC after node with it was rebooted", func(ctx context.Context) {
		init()
		ginkgo.DeferCleanup(cleanup)

		var err error
		// create pvc
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(ns).Create(ctx,
			constructPVC(ns, driver.GetClaimSize(), k8sSC.Name, pvcName),
			metav1.CreateOptions{})
		framework.ExpectNoError(err)

		// create pod with pvc
		pod, err = common.CreatePod(ctx, f.ClientSet, ns, nil, []*corev1.PersistentVolumeClaim{pvc},
			false, "sleep 3600")
		framework.ExpectNoError(err)

		framework.Logf("Pod %s with PVC %s created.", pod.Name, pvc.Name)

		// since test is run in Kind k8s cluster, each node is represented by docker container
		// node' name is the same as a docker container name by which this node is represented.
		containerToStop = pod.Spec.NodeName

		// stop container
		cmd := fmt.Sprintf("docker stop %s", containerToStop)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)

		// wait up to 5 minutes until node that is located on containerToStop become NotReady
		nodeNotReady := e2enode.WaitForNodeToBeNotReady(ctx, f.ClientSet, containerToStop, time.Minute*5)
		if !nodeNotReady {
			framework.Failf("Node %s still ready", containerToStop)
		}

		// start container
		cmd = fmt.Sprintf("docker start %s", containerToStop)
		_, _, err = executor.RunCmd(cmd)
		framework.ExpectNoError(err)
		started = true

		// wait up to 5 minutes until node that is located on containerToStop become Ready
		nodeReady := e2enode.WaitForNodeToBeReady(ctx, f.ClientSet, containerToStop, time.Minute*5)
		if !nodeReady {
			framework.Failf("Node %s still NotReady", containerToStop)
		}

		// wait 5 minutes until all pods become ready
		err = e2epod.WaitForPodsRunningReady(ctx, f.ClientSet, ns, 0, 0, 5*time.Minute)
		framework.ExpectNoError(err)
		framework.Logf("All pods are ready after node restart")

		// check that pod consume same pvc
		var boundAgain = false
		pods, err := e2epod.GetPodsInNamespace(ctx, f.ClientSet, f.Namespace.Name, map[string]string{})
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
		framework.Logf("Pod has same PVC: %v", boundAgain)
		gomega.Expect(boundAgain).To(gomega.BeTrue(), "expected boundAgain == true")
	})
}
