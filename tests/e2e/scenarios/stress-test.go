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
	"sync"
	"time"

	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"

	"github.com/dell/csi-baremetal-e2e-tests/e2e/common"
)

var stsName = "stress-test-sts"

// DefineStressTestSuite defines custom csi-baremetal stress test
func DefineStressTestSuite(driver *baremetalDriver) {
	ginkgo.Context("Baremetal-csi stress test", func() {
		// Test scenario:
		// Create StatefulSet with replicas count which is equal to kind node count
		// Each replica should consume all loop devices from the node in HDD SC
		driveStressTest(driver)
	})
}

// driveStressTest test checks behavior of driver under horizontal scale load (increase amount of nodes)
func driveStressTest(driver *baremetalDriver) {
	ginkgo.BeforeEach(skipIfNotAllTests)

	var (
		k8sSC            *storagev1.StorageClass
		driverCleanup    func()
		ns               string
		f                = framework.NewDefaultFramework("stress")
		amountOfCSINodes int
	)

	init := func() {
		var (
			perTestConf *storageframework.PerTestConfig
			err         error
		)
		ns = f.Namespace.Name

		perTestConf, driverCleanup = driver.PrepareTest(f)

		nodes, err := e2enode.GetReadySchedulableNodes(f.ClientSet)
		framework.ExpectNoError(err)
		amountOfCSINodes = len(nodes.Items)

		k8sSC = driver.GetDynamicProvisionStorageClass(perTestConf, "xfs")
		k8sSC, err = f.ClientSet.StorageV1().StorageClasses().Create(context.TODO(), k8sSC, metav1.CreateOptions{})
		framework.ExpectNoError(err)
	}

	cleanup := func() {
		e2elog.Logf("Starting cleanup for test StressTest")

		ssPods, err := f.PodClientNS(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("sts=%s", stsName)})

		err = f.ClientSet.AppsV1().StatefulSets(ns).Delete(context.TODO(), stsName, metav1.DeleteOptions{})
		framework.ExpectNoError(err)

		var wg sync.WaitGroup

		// Kubernetes e2e test framework doesn't have native methods to wait for StatefulSet deletion
		// So it's needed to wait for each pod to be deleted manually after SS deletion
		for _, pod := range ssPods.Items {
			podCopy := pod
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = e2epod.WaitForPodNotFoundInNamespace(f.ClientSet, podCopy.Name, f.Namespace.Name, 2*time.Minute)
				framework.ExpectNoError(err)
			}()
		}

		wg.Wait()

		pvcList, err := f.ClientSet.CoreV1().PersistentVolumeClaims(ns).List(context.TODO(), metav1.ListOptions{})
		framework.ExpectNoError(err)
		pvcPointersList := make([]*corev1.PersistentVolumeClaim, len(pvcList.Items))
		for i, _ := range pvcList.Items {
			pvcPointersList[i] = &pvcList.Items[i]
		}

		common.CleanupAfterCustomTest(f, driverCleanup, nil, pvcPointersList)
	}

	ginkgo.It("should serve StatefulSet on multi node cluster", func() {
		init()
		defer cleanup()

		ss := CreateStressTestStatefulSet(ns, int32(amountOfCSINodes), 3, k8sSC.Name,
			driver.GetClaimSize())
		ss, err := f.ClientSet.AppsV1().StatefulSets(ns).Create(context.TODO(), ss, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		err = common.WaitForStatefulSetReplicasReady(context.TODO(), ss.Name, ns, f.ClientSet, 20*time.Second, 10*time.Minute)
		framework.ExpectNoError(err)
	})
}

// CreateStressTestStatefulSet creates StatefulSet manifest for test purposes
// Receives namespace as ns, amount of SS replicas, amount of PVCs per replica, name of StorageClass that serves PVCs,
// size of PVCs.
// Returns instance of appsv1.StatefulSet
func CreateStressTestStatefulSet(ns string, amountOfReplicas int32, volumesPerReplica int, scName string,
	claimSize string) *appsv1.StatefulSet {
	pvcs := make([]corev1.PersistentVolumeClaim, volumesPerReplica)
	vms := make([]corev1.VolumeMount, volumesPerReplica)
	pvcPrefix := "volume%d"
	dataPath := "/data/%s"

	for i, _ := range pvcs {
		pvcs[i] = corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(pvcPrefix, i),
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
				StorageClassName: &scName,
			},
		}
	}

	for i := 0; i < volumesPerReplica; i++ {
		volumeName := pvcs[i].Name
		vms[i] = corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf(dataPath, volumeName),
		}
	}

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"sts": stsName},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "busybox",
					Image:           "busybox:1.29",
					Command:         []string{"/bin/sh", "-c", "sleep 36000"},
					VolumeMounts:    vms,
					ImagePullPolicy: "Never",
				},
			},
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "sts",
										Operator: "In",
										Values:   []string{stsName},
									},
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		},
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stsName,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &amountOfReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"sts": stsName},
			},
			Template:             podTemplate,
			VolumeClaimTemplates: pvcs,
			ServiceName:          stsName,
			PodManagementPolicy:  "Parallel",
		},
	}
}
