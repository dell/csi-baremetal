/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"os"
	"path"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	operatorVersionEnv = "OPERATOR_VERSION"
	csiVersionEnv      = "CSI_VERSION"
)

// DeployOperator calls DeployOperatorWithClient with f.ClientSet
func DeployOperator(f *framework.Framework) (func(), error) {
	return DeployOperatorWithClient(f.ClientSet, f.Namespace.Name)
}

// DeployOperatorWithClient deploys csi-baremetal-operator with CmdHelmExecutor
// After install - waiting before all pods ready
// Cleanup - deleting operator-chart and csi crds
func DeployOperatorWithClient(c clientset.Interface, ns string) (func(), error) {
	var (
		executor        = CmdHelmExecutor{framework.TestContext.KubeConfig}
		operatorVersion = os.Getenv(operatorVersionEnv)
		chart           = HelmChart{
			name:      "csi-baremetal-operator",
			path:      path.Join(BMDriverTestContext.ChartsFolder, "csi-baremetal-operator"),
			namespace: ns,
		}
		installArgs = fmt.Sprintf("--set image.tag=%s", operatorVersion)
		waitTime    = 1 * time.Minute
	)

	cleanup := func() {
		if err := executor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Operator helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}

		//crdPath := path.Join(chart.path, "crds")
		//if err := execCmdObj(framework.KubectlCmd("delete", "-f", crdPath)); err != nil {
		//	e2elog.Logf("CRD deletion failed")
		//}
	}

	if err := executor.InstallRelease(&chart, installArgs); err != nil {
		return nil, err
	}

	if err := e2epod.WaitForPodsRunningReady(c, chart.namespace, 0, 0, waitTime, nil); err != nil {
		cleanup()
		return nil, err
	}

	return cleanup, nil
}

// DeployCSI deploys csi-baremetal-deployment with CmdHelmExecutor
// After install - waiting all pods ready, checking kubernetes-scheduler restart
// Cleanup - deleting csi-chart, cleaning all csi custom resources
func DeployCSI(f *framework.Framework) (func(), error) {
	var (
		executor   = CmdHelmExecutor{framework.TestContext.KubeConfig}
		csiVersion = os.Getenv(csiVersionEnv)
		chart      = HelmChart{
			name:      "csi-baremetal",
			path:      path.Join(BMDriverTestContext.ChartsFolder, "csi-baremetal-deployment"),
			namespace: f.Namespace.Name,
		}
		installArgs = fmt.Sprintf("--set image.tag=%s "+
			"--set image.pullPolicy=IfNotPresent "+
			"--set driver.drivemgr.type=loopbackmgr "+
			"--set driver.drivemgr.deployConfig=true "+
			"--set scheduler.patcher.enable=true", csiVersion)
		podWait         = 3 * time.Minute
		sleepBeforeWait = 10 * time.Second
		schedulerRC     = newSchedulerRestartChecker(f.ClientSet)
	)

	cleanup := func() {
		// delete resources with finalizers
		if BMDriverTestContext.CompleteUninstall {
			deleteCSIResources([]string{"pvc", "volumes", "lvgs"})
		}

		if err := executor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Deployment helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}

		// delete resources without finalizers
		if BMDriverTestContext.CompleteUninstall {
			deleteCSIResources([]string{"acr", "ac", "drives"})
		}
	}

	if err := schedulerRC.ReadInitialState(); err != nil {
		e2elog.Logf("SchedulerRestartChecker is not initialized. Err: %s", err)
	}

	if err := executor.InstallRelease(&chart, installArgs); err != nil {
		return nil, err
	}

	// wait until operator reconciling CR
	time.Sleep(sleepBeforeWait)

	if err := e2epod.WaitForPodsRunningReady(f.ClientSet, chart.namespace, 0, 0, podWait, nil); err != nil {
		cleanup()
		return nil, err
	}

	if schedulerRC.IsInitialized {
		if isRestarted, err := schedulerRC.WaitForRestart(); err != nil {
			e2elog.Logf("SchedulerRestartChecker has been failed while waiting. Err: %s", err)
		} else {
			if isRestarted {
				e2elog.Logf("Scheduler is restarted")
			} else {
				e2elog.Logf("Scheduler is not restarted")
			}
		}
	}

	// print info about all custom resources into log messages
	getCSIResources()

	return cleanup, nil
}

func getCSIResources() {
	resources := []string{"pvc", "volumes", "lvgs", "csibmnodes", "acr", "ac", "drives"}

	for _, name := range resources {
		if err := execCmdObj(framework.KubectlCmd("get", name)); err != nil {
			e2elog.Logf("Failed to get %s with kubectl", name)
		}
	}
}

func deleteCSIResources(resources []string) {
	for _, name := range resources {
		if err := execCmdObj(framework.KubectlCmd("delete", name, "--all")); err != nil {
			e2elog.Logf("%s deletion failed", name)
		}
	}
}

func newSchedulerRestartChecker(client clientset.Interface) *schedulerRestartChecker {
	return &schedulerRestartChecker{
		IsInitialized:      false,
		c:                  client,
		schedulerLabel:     "component=kube-scheduler",
		restartWaitTimeout: time.Minute * 2,
	}
}

type schedulerRestartChecker struct {
	c                  clientset.Interface
	initialState       map[string]metav1.Time
	schedulerLabel     string
	restartWaitTimeout time.Duration
	IsInitialized      bool
}

func (rc *schedulerRestartChecker) ReadInitialState() error {
	var err error
	rc.initialState, err = rc.getPODStartTimeMap()
	if err != nil {
		return err
	}
	if len(rc.initialState) == 0 {
		return fmt.Errorf("can't find schedulers PODs during reading initial state")
	}

	rc.IsInitialized = true
	return nil
}

func (rc *schedulerRestartChecker) WaitForRestart() (bool, error) {
	e2elog.Logf("Wait for scheduler restart")

	deadline := time.Now().Add(rc.restartWaitTimeout)
	for {
		ready, err := rc.CheckRestarted()
		if err != nil {
			return false, err
		}
		if ready {
			e2elog.Logf("Scheduler restarted")
			return true, nil
		}
		msg := "Scheduler restart NOT detected yet"
		e2elog.Logf(msg)
		if time.Now().After(deadline) {
			e2elog.Logf("Scheduler didn't receive extender configuration after %f minutes. Continue...",
				rc.restartWaitTimeout.Minutes())
			break
		}
		time.Sleep(time.Second * 5)
	}

	return false, nil
}

func (rc *schedulerRestartChecker) CheckRestarted() (bool, error) {
	currentState, err := rc.getPODStartTimeMap()
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

func (rc *schedulerRestartChecker) getPODStartTimeMap() (map[string]metav1.Time, error) {
	pods, err := rc.findSchedulerPods()
	if err != nil {
		return nil, err
	}
	return rc.buildPODStartTimeMap(pods), nil
}

func (rc *schedulerRestartChecker) buildPODStartTimeMap(pods *corev1.PodList) map[string]metav1.Time {
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

func (rc *schedulerRestartChecker) findSchedulerPods() (*corev1.PodList, error) {
	pods, err := rc.c.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: rc.schedulerLabel})
	if err != nil {
		return nil, err
	}
	e2elog.Logf("Find %d scheduler pods", len(pods.Items))
	return pods, nil
}
