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

package common

import (
	"fmt"
	"io/ioutil"
	"path"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	"sigs.k8s.io/yaml"

	akey "github.com/dell/csi-baremetal/pkg/crcontrollers/operator/common"
)

const (
	extenderManifestsFolder = "csi-baremetal-scheduler-extender/templates/"
	schedulerLabel          = "component=kube-scheduler"
	restartWaitTimeout      = time.Minute * 2
)

func DeploySchedulerExtender(f *framework.Framework) (func(), error) {
	return deployExtenderManifests(f)
}

func deployExtenderManifests(f *framework.Framework) (func(), error) {
	manifests := []string{
		extenderManifestsFolder + "rbac.yaml",
	}

	daemonSetCleanup, err := buildDaemonSet(
		f.ClientSet,
		f.Namespace.Name,
		path.Join("/tmp", extenderManifestsFolder, "scheduler-extender.yaml"))
	if err != nil {
		return nil, err
	}

	manifestsCleanupFunc, err := f.CreateFromManifests(nil, manifests...)
	if err != nil {
		return nil, err
	}

	cleanupFunc := func() {
		daemonSetCleanup()
		manifestsCleanupFunc()
	}

	return cleanupFunc, nil
}

func DeployPatcher(c clientset.Interface, namespace string) (func(), error) {
	manifestsCleanupFunc, err := waitForRestart(c,
		func() (func(), error) {
			return deployPatcherManifests(c, namespace)
		})
	if err != nil {
		return nil, err
	}
	return func() {
		_, err := waitForRestart(c,
			func() (func(), error) {
				manifestsCleanupFunc()
				return func() {}, nil
			})
		if err != nil {
			e2elog.Logf("failed to cleanup patcher, err: %s", err.Error())
		}
	}, nil
}

func deployPatcherManifests(c clientset.Interface, namespace string) (func(), error) {
	daemonSetCleanup, err := buildDaemonSet(
		c,
		namespace,
		path.Join("/tmp", extenderManifestsFolder, "patcher.yaml"))
	if err != nil {
		return nil, err
	}
	configMapCleanup, err := buildConfigMap(c, namespace)
	if err != nil {
		return nil, err
	}
	return func() {
		daemonSetCleanup()
		configMapCleanup()
	}, nil
}

func waitForRestart(c clientset.Interface, fu func() (func(), error)) (func(), error) {
	wait := BMDriverTestContext.BMWaitSchedulerRestart

	rc := newSchedulerRestartChecker(c)
	if wait {
		err := rc.ReadInitialState()
		if err != nil {
			return nil, err
		}
	}
	result, err := fu()
	if err != nil {
		return nil, err
	}
	if wait {
		e2elog.Logf("Wait for scheduler restart")
		deadline := time.Now().Add(restartWaitTimeout)
		for {
			ready, err := rc.CheckRestarted()
			if err != nil {
				return nil, err
			}
			if ready {
				e2elog.Logf("Scheduler restarted")
				break
			}
			msg := "Scheduler restart NOT detected yet"
			e2elog.Logf(msg)
			if time.Now().After(deadline) {
				e2elog.Logf("Scheduler didn't receive extender configuration after %f minutes. Continue...",
					restartWaitTimeout.Minutes())
				break
			}
			time.Sleep(time.Second * 5)
		}
	}
	return result, nil
}

func buildConfigMap(c clientset.Interface, namespace string) (func(), error) {
	file, err := ioutil.ReadFile("/tmp/" + extenderManifestsFolder + "/patcher-configmap.yaml")
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{}
	err = yaml.Unmarshal(file, cm)
	if err != nil {
		return nil, err
	}
	cm.ObjectMeta.Namespace = namespace
	cm, err = c.CoreV1().ConfigMaps(namespace).Create(cm)
	if err != nil {
		return nil, err
	}
	return func() {
		if err := c.CoreV1().ConfigMaps(namespace).Delete(cm.Name, &metav1.DeleteOptions{}); err != nil {
			e2elog.Logf("Failed to delete SE configmap %s: %v", cm.Name, err)
		}
	}, nil
}

func buildDaemonSet(c clientset.Interface, namespace, manifestFilePath string) (func(), error) {
	file, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	ds := &appsv1.DaemonSet{}
	err = yaml.Unmarshal(file, ds)
	if err != nil {
		return nil, err
	}

	ds.ObjectMeta.Namespace = namespace
	ds, err = c.AppsV1().DaemonSets(namespace).Create(ds)
	if err != nil {
		return nil, err
	}
	return func() {
		if err := c.AppsV1().DaemonSets(namespace).Delete(ds.Name, &metav1.DeleteOptions{}); err != nil {
			e2elog.Logf("Failed to delete daemonset %s: %v", ds.Name, err)
		}
	}, nil
}

func newSchedulerRestartChecker(client clientset.Interface) *schedulerRestartChecker {
	return &schedulerRestartChecker{
		c: client,
	}
}

type schedulerRestartChecker struct {
	c            clientset.Interface
	initialState map[string]metav1.Time
}

func (rc *schedulerRestartChecker) ReadInitialState() error {
	var err error
	rc.initialState, err = getPODStartTimeMap(rc.c)
	if err != nil {
		return err
	}
	if len(rc.initialState) == 0 {
		return fmt.Errorf("can't find schedulers PODs during reading initial state")
	}
	return nil
}

func (rc *schedulerRestartChecker) CheckRestarted() (bool, error) {
	currentState, err := getPODStartTimeMap(rc.c)
	if err != nil {
		return false, err
	}
	for podName, initialTime := range rc.initialState {
		currentTime, ok := currentState[podName]
		if !ok {
			// podName not found
			return false, nil
		}
		// check that POD start time changed
		if !currentTime.After(initialTime.Time) {
			// at lease on pod not restarted yet
			return false, nil
		}
		// check that POD uptime more than 10 seconds
		// we need to wait additional 10 seconds to protect from CrashLoopBackOff caused by frequently POD restarts
		if time.Since(currentTime.Time).Seconds() <= 10 {
			return false, nil
		}
	}
	return true, nil
}

func getPODStartTimeMap(client clientset.Interface) (map[string]metav1.Time, error) {
	pods, err := findSchedulerPods(client)
	if err != nil {
		return nil, err
	}
	return buildPODStartTimeMap(pods), nil
}

func buildPODStartTimeMap(pods *corev1.PodList) map[string]metav1.Time {
	data := map[string]metav1.Time{}
	for _, p := range pods.Items {
		if len(p.Status.ContainerStatuses) == 0 {
			continue
		}
		if p.Status.ContainerStatuses[0].State.Running == nil {
			data[p.Name] = metav1.Time{}
			continue
		}
		data[p.Name] = p.Status.ContainerStatuses[0].State.Running.StartedAt
	}
	return data
}

func findSchedulerPods(client clientset.Interface) (*corev1.PodList, error) {
	pods, err := client.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: schedulerLabel})
	if err != nil {
		return nil, err
	}
	e2elog.Logf("Find %d scheduler pods", len(pods.Items))
	return pods, nil
}

func DeployCSIBMOperator(c clientset.Interface) (func(), error) {
	var (
		chartsDir               = "/tmp"
		operatorManifestsFolder = "csi-baremetal-operator/templates"
	)

	setupRBACCMD := fmt.Sprintf("kubectl apply -f %s",
		path.Join(chartsDir, operatorManifestsFolder, "csibm-rbac.yaml"))
	cleanupRBACCMD := fmt.Sprintf("kubectl delete -f %s",
		path.Join(chartsDir, operatorManifestsFolder, "csibm-rbac.yaml"))

	if _, _, err := GetExecutor().RunCmd(setupRBACCMD); err != nil {
		return nil, err
	}

	file, err := ioutil.ReadFile(path.Join(chartsDir, operatorManifestsFolder, "csibm-controller.yaml"))
	if err != nil {
		return nil, err
	}

	deployment := &appsv1.Deployment{}
	err = yaml.Unmarshal(file, deployment)
	if err != nil {
		return nil, err
	}

	if err = waitUntilAllNodesWillBeTagged(c); err != nil {
		return nil, err
	}

	depl, err := c.AppsV1().Deployments("default").Create(deployment)
	if err != nil {
		return nil, err
	}

	return func() {

		if err := c.AppsV1().Deployments("default").Delete(depl.Name, &metav1.DeleteOptions{}); err != nil {
			e2elog.Logf("Failed to delete deployment %s: %v", depl.Name, err)
		}
		if _, _, err = GetExecutor().RunCmd(cleanupRBACCMD); err != nil {
			e2elog.Logf("Failed to delete RBAC for CSIBMNode operator: %v", err)
		}

		// remove annotations
		nodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			e2elog.Logf("Unable to get nodes list for cleaning annotations: %v", err)
			return
		}

		driverRegistarNodeAnnotation := "csi.volume.kubernetes.io/nodeid"
		for _, node := range nodes.Items {
			isFound := false
			// try to remove annotations set by driver registar
			if _, ok := node.GetAnnotations()[driverRegistarNodeAnnotation]; ok {
				delete(node.Annotations, driverRegistarNodeAnnotation)
				isFound = true
			}
			// try to remove annotations set by csi operator
			if _, ok := node.GetAnnotations()[akey.DeafultNodeIDAnnotationKey]; ok {
				delete(node.Annotations, akey.DeafultNodeIDAnnotationKey)
				isFound = true
			}
			// update node object if required
			if isFound {
				if _, err := c.CoreV1().Nodes().Update(&node); err != nil {
					e2elog.Logf("Unable to unset annotations from node %s: %v", node.Name, err)
				}
			}
		}
	}, nil
}

func waitUntilAllNodesWillBeTagged(c clientset.Interface) error {
	nodeAnnotationMap := make(map[string]string, 0)

	timeout := time.Minute * 2
	sleepTime := time.Second * 5
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(sleepTime) {
		nodes, err := e2enode.GetReadySchedulableNodesOrDie(c)
		allHas := true

		if err != nil {
			e2elog.Logf("Got error during waitUntilAllNodesWillBeTagged: %v. Sleep and retry", err)
			allHas = false
		}

		for _, node := range nodes.Items {
			if _, ok := node.GetAnnotations()[akey.DeafultNodeIDAnnotationKey]; !ok {
				e2elog.Logf("Not all nodes has annotation %s. Sleep and retry", akey.DeafultNodeIDAnnotationKey)
				allHas = false
				break
			}
			nodeAnnotationMap[node.Name] = node.GetAnnotations()[akey.DeafultNodeIDAnnotationKey]
		}
		if allHas {
			e2elog.Logf("Annotation %s was set for all nodes: %v", akey.DeafultNodeIDAnnotationKey, nodeAnnotationMap)
			return nil
		}
		time.Sleep(sleepTime)
	}

	return nil
}
