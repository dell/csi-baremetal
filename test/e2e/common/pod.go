/*
Copyright 2019 The Kubernetes Authors.

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

package common

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

// CreatePod with given claims based on node selector
// Modified version of CreatePod function from k8s.io/kubernetes/test/e2e/framework/pod
func CreatePod(client clientset.Interface, namespace string, nodeSelector map[string]string, pvclaims []*v1.PersistentVolumeClaim, isPrivileged bool, command string) (*v1.Pod, error) {
	pod := MakePod(namespace, nodeSelector, pvclaims, isPrivileged, command)

	pod, err := client.CoreV1().Pods(namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("pod Create API error: %v", err)
	}
	// Waiting for pod to be running
	err = e2epod.WaitTimeoutForPodReadyInNamespace(client, pod.Name, namespace, 5*time.Minute)
	if err != nil {
		return pod, fmt.Errorf("pod %q is not Running: %v", pod.Name, err)
	}

	// get fresh pod info
	pod, err = client.CoreV1().Pods(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return pod, fmt.Errorf("pod Get API error: %v", err)
	}
	return pod, nil
}

// MakePod returns a pod definition based on the namespace. The pod references the PVC's
// name.  A slice of BASH commands can be supplied as args to be run by the pod
// Modified version of MakePod function from k8s.io/kubernetes/test/e2e/framework/pod
// which support Block volumes
func MakePod(ns string, nodeSelector map[string]string, pvclaims []*v1.PersistentVolumeClaim, isPrivileged bool, command string) *v1.Pod {
	if len(command) == 0 {
		command = "trap exit TERM; while true; do sleep 1; done"
	}
	podSpec := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvc-tester-",
			Namespace:    ns,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "write-pod",
					Image:           "centos",
					ImagePullPolicy: v1.PullIfNotPresent,
					//Image:   framework.BusyBoxImage,
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", command},
					SecurityContext: &v1.SecurityContext{
						Privileged: &isPrivileged,
					},
				},
			},
			RestartPolicy: v1.RestartPolicyOnFailure,
		},
	}
	var volumeMounts []v1.VolumeMount
	var volumeDevices []v1.VolumeDevice
	var volumes = make([]v1.Volume, len(pvclaims))
	for index, pvclaim := range pvclaims {
		volumename := fmt.Sprintf("volume%v", index+1)
		if pvcIsBlock(pvclaim) {
			volumeDevices = append(volumeDevices, v1.VolumeDevice{Name: volumename, DevicePath: "/dev/" + volumename})
		} else {
			volumeMounts = append(volumeMounts, v1.VolumeMount{Name: volumename, MountPath: "/mnt/" + volumename})
		}
		volumes[index] = v1.Volume{Name: volumename, VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvclaim.Name, ReadOnly: false}}}
	}
	podSpec.Spec.Containers[0].VolumeMounts = volumeMounts
	podSpec.Spec.Containers[0].VolumeDevices = volumeDevices
	podSpec.Spec.Volumes = volumes
	if nodeSelector != nil {
		podSpec.Spec.NodeSelector = nodeSelector
	}
	return podSpec
}

func pvcIsBlock(pvc *v1.PersistentVolumeClaim) bool {
	if pvc.Spec.VolumeMode == nil {
		return false
	}
	return *pvc.Spec.VolumeMode == v1.PersistentVolumeBlock
}
