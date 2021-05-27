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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"github.com/dell/csi-baremetal/pkg/base/command"
)

const (
	operatorVersionEnv = "OPERATOR_VERSION"
	csiVersionEnv      = "CSI_VERSION"
)

func DeployCSIComponents(f *framework.Framework, additionalInstallArgs string) (func(), error) {
	cancelLogging := testsuites.StartPodLogs(f)

	cleanupOperator, err := DeployOperator(f)
	if err != nil {
		cancelLogging()
		return nil, err
	}

	cleanupCSI, err := DeployCSIA(f, additionalInstallArgs)
	if err != nil {
		cancelLogging()
		cleanupOperator()
		return nil, err
	}

	return func() {
		cancelLogging()
		cleanupCSI()
		cleanupOperator()
	}, nil
}

// DeployOperator deploys csi-baremetal-operator with CmdHelmExecutor
// After install - waiting before all pods ready
// Cleanup - deleting operator-chart and csi crds
// Helm command - "helm install csi-baremetal-operator <CHARTS_DIR>/csi-baremetal-operator
// 			--set image.tag=<OPERATOR_VERSION>
//			--set image.pullPolicy=IfNotPresent"
func DeployOperator(f *framework.Framework) (func(), error) {
	var (
		executor        = CmdHelmExecutor{kubeconfig: framework.TestContext.KubeConfig, executor: GetExecutor()}
		operatorVersion = os.Getenv(operatorVersionEnv)
		chart           = HelmChart{
			name:      "csi-baremetal-operator",
			path:      path.Join(BMDriverTestContext.ChartsDir, "csi-baremetal-operator"),
			namespace: f.Namespace.Name,
		}
		installArgs = fmt.Sprintf("--set image.tag=%s "+
			"--set image.pullPolicy=IfNotPresent", operatorVersion)
		waitTime = 1 * time.Minute
	)

	cleanup := func() {
		if err := executor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Operator helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}
	}

	if err := executor.InstallRelease(&chart, installArgs); err != nil {
		return nil, err
	}

	if err := e2epod.WaitForPodsRunningReady(f.ClientSet, chart.namespace, 0, 0, waitTime, nil); err != nil {
		cleanup()
		return nil, err
	}

	return cleanup, nil
}

// DeployCSIWithArgs deploys csi-baremetal-deployment with CmdHelmExecutor
// After install - waiting all pods ready, checking kubernetes-scheduler restart
// Cleanup - deleting csi-chart, cleaning all csi custom resources
// Helm command - "helm install csi-baremetal <CHARTS_DIR>/csi-baremetal-deployment
// 			--set image.tag=<CSI_VERSION>
//			--set image.pullPolicy=IfNotPresent
//			--set driver.drivemgr.type=loopbackmgr
//			--set driver.drivemgr.deployConfig=true
//			--set scheduler.patcher.enable=true
//			--set scheduler.log.level=debug
//			--set nodeController.log.level=debug
//			--set driver.log.level=debug"
func DeployCSIA(f *framework.Framework, additionalInstallArgs string) (func(), error) {
	var (
		cmdExecutor  = GetExecutor()
		helmExecutor = CmdHelmExecutor{kubeconfig: framework.TestContext.KubeConfig, executor: cmdExecutor}
		csiVersion   = os.Getenv(csiVersionEnv)
		chart        = HelmChart{
			name:      "csi-baremetal",
			path:      path.Join(BMDriverTestContext.ChartsDir, "csi-baremetal-deployment"),
			namespace: f.Namespace.Name,
		}
		installArgs = fmt.Sprintf("--set image.tag=%s "+
			"--set image.pullPolicy=IfNotPresent "+
			"--set driver.drivemgr.type=loopbackmgr "+
			"--set driver.drivemgr.deployConfig=true "+
			"--set scheduler.patcher.enable=true "+
			"--set scheduler.log.level=debug "+
			"--set nodeController.log.level=debug "+
			"--set driver.log.level=debug", csiVersion)
		podWait         = 3 * time.Minute
		sleepBeforeWait = 10 * time.Second
		schedulerRC     = newSchedulerRestartChecker(f.ClientSet)
		isRestarted     = false
		err             error
	)

	if additionalInstallArgs != "" {
		installArgs += " " + additionalInstallArgs
	}

	cleanup := func() {
		if BMDriverTestContext.CompleteUninstall {
			CleanupLoopbackDevices(f)
			// delete resources with finalizers
			// pvcs and volumes are namespaced resources and deleting with it
			deleteCSIResources(cmdExecutor, []string{"lvgs", "csibmnodes"})
		}

		if err := helmExecutor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Deployment helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}

		if BMDriverTestContext.CompleteUninstall {
			// delete resources without finalizers
			deleteCSIResources(cmdExecutor, []string{"acr", "ac", "drives"})
		}
	}

	if err := schedulerRC.ReadInitialState(); err != nil {
		e2elog.Logf("SchedulerRestartChecker is not initialized. Err: %s", err)
	}

	if err := helmExecutor.InstallRelease(&chart, installArgs); err != nil {
		return nil, err
	}

	// wait until operator reconciling CR
	time.Sleep(sleepBeforeWait)

	if err := e2epod.WaitForPodsRunningReady(f.ClientSet, chart.namespace, 0, 0, podWait, nil); err != nil {
		cleanup()
		return nil, err
	}

	if schedulerRC.IsInitialized {
		if isRestarted, err = schedulerRC.WaitForRestart(); err != nil {
			e2elog.Logf("SchedulerRestartChecker has been failed while waiting. Err: %s", err)
		}
	}

	if isRestarted {
		e2elog.Logf("Scheduler is restarted")
	} else {
		cleanup()
		return nil, errors.New("scheduler is not restarted")
	}

	// print info about all custom resources into log messages
	getCSIResources(cmdExecutor)

	return cleanup, nil
}

func getCSIResources(e command.CmdExecutor) {
	resources := []string{"pvc", "volumes", "lvgs", "csibmnodes", "acr", "ac", "drives"}

	for _, name := range resources {
		cmd := framework.KubectlCmd("get", name)
		if _, _, err := e.RunCmd(cmd); err != nil {
			e2elog.Logf("Failed to get %s with kubectl", name)
		}
	}
}

func deleteCSIResources(e command.CmdExecutor, resources []string) {
	for _, name := range resources {
		cmd := framework.KubectlCmd("delete", name, "--all")
		if _, _, err := e.RunCmd(cmd); err != nil {
			e2elog.Logf("%s deletion failed", name)
		}
	}
}

// CleanupLoopbackDevices executes in node pods drive managers containers kill -SIGHUP 1
// Returns error if it's failed to get node pods
func CleanupLoopbackDevices(f *framework.Framework) error {
	pods, err := GetNodePodsNames(f)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		f.ExecShellInContainer(pod, "drivemgr", "/bin/kill -SIGHUP 1")
	}
	return nil
}

// GetNodePodsNames tries to get slice of node pods names
// Receives framework.Framewor
// Returns slice of pods name, error if it's failed to get node pods
func GetNodePodsNames(f *framework.Framework) ([]string, error) {
	pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	podsNames := make([]string, 0)
	for _, pod := range pods.Items {
		if len(pod.OwnerReferences) == 1 &&
			pod.OwnerReferences[0].Kind == "DaemonSet" &&
			strings.Contains(pod.OwnerReferences[0].Name, "csi-baremetal-node") {
			podsNames = append(podsNames, pod.Name)
		}
	}
	framework.Logf("Find node pods: ", podsNames)
	return podsNames, nil
}
