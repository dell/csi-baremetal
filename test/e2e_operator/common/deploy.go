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
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/dell/csi-baremetal/pkg/base/command"
)

const (
	operatorVersionEnv = "OPERATOR_VERSION"
	csiVersionEnv      = "CSI_VERSION"

	operatorNamespace = "e2e-test-operator"
)

// DeployOperatorWithClient deploys csi-baremetal-operator with CmdHelmExecutor
// After install - waiting before all pods ready
// Cleanup - deleting operator-chart and csi crds
func DeployOperatorWithClient(c clientset.Interface) (func(), error) {
	var (
		executor        = CmdHelmExecutor{kubeconfig: framework.TestContext.KubeConfig, executor: GetExecutor()}
		operatorVersion = os.Getenv(operatorVersionEnv)
		chart           = HelmChart{
			name:      "csi-baremetal-operator",
			path:      path.Join(BMDriverTestContext.ChartsDir, "csi-baremetal-operator"),
			namespace: operatorNamespace,
		}
		installArgs = fmt.Sprintf("--set image.tag=%s", operatorVersion)
		waitTime    = 1 * time.Minute
	)

	cleanup := func() {
		if err := executor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Operator helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}

		if err := c.CoreV1().Namespaces().Delete(operatorNamespace, nil); err != nil {
			e2elog.Logf("Namespace %s deletion failed.", chart.namespace)
		}

		//crdPath := path.Join(chart.path, "crds")
		//if err := execCmdObj(framework.KubectlCmd("delete", "-f", crdPath)); err != nil {
		//	e2elog.Logf("CRD deletion failed")
		//}
	}

	if _, err := c.CoreV1().Namespaces().Create(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: operatorNamespace}}); err != nil && !k8sError.IsAlreadyExists(err) {
		return nil, err
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

// DeployCSIWithArgs deploys csi-baremetal-deployment with CmdHelmExecutor
// After install - waiting all pods ready, checking kubernetes-scheduler restart
// Cleanup - deleting csi-chart, cleaning all csi custom resources
func DeployCSI(f *framework.Framework, additionalInstallArgs string) (func(), error) {
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
	)

	if additionalInstallArgs != "" {
		installArgs += " " + additionalInstallArgs
	}

	cleanup := func() {
		// delete resources with finalizers
		if BMDriverTestContext.CompleteUninstall {
			deleteCSIResources(cmdExecutor, []string{"lvgs", "csibmnodes"})
		}

		if err := helmExecutor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Deployment helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}

		// delete resources without finalizers
		if BMDriverTestContext.CompleteUninstall {
			deleteCSIResources(cmdExecutor, []string{"acr", "ac", "drives"})
		}

		nodeList, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			e2elog.Logf("CRD deletion failed")
		}

		for _, node := range nodeList.Items {
			cmd := fmt.Sprintf("docker exec %s find /home -type f -name \"*.img\" -delete -print", node.Name)
			if _, _, err = cmdExecutor.RunCmd(cmd); err != nil {
				e2elog.Logf("Failed to clean loopback devices: %s", err)
			}
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
