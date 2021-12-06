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
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/framework/podlogs"
)

const (
	operatorVersionEnv = "OPERATOR_VERSION"
	csiVersionEnv      = "CSI_VERSION"
)

// Create folder for every tests and save container logs and events
func collectPodLogs(f *framework.Framework) func() {
	ctx, cancel := context.WithCancel(context.Background())
	cs := f.ClientSet
	ns := f.Namespace

	testName := strings.ReplaceAll(ginkgo.CurrentGinkgoTestDescription().FullTestText, "/", "")
	dirname := fmt.Sprintf("reports/%v/", testName)
	if err := os.MkdirAll(dirname, os.ModePerm); err != nil {
		log.Fatalf("error creating folders: %v", err)
	}
	to := podlogs.LogOutput{
		LogPathPrefix: dirname,
	}
	eventsLogs, err := os.OpenFile(fmt.Sprintf("reports/%v/events.log", testName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	if err := podlogs.CopyAllLogs(ctx, cs, ns.Name, to); err != nil {
		e2elog.Logf("Cant copy all pod logs: %s", err)
	}
	if err := podlogs.WatchPods(ctx, cs, ns.Name, eventsLogs); err != nil {
		e2elog.Logf("Cant copy all pod events: %s", err)
	}
	return func() {
		_ = eventsLogs.Close()
		cancel()
	}
}

// DeployCSIComponents deploys csi-baremetal-operator and csi-baremetal-deployment with CmdHelmExecutor
// and start print containers logs from framework namespace
// returns cleanup function and error if failed
// See DeployOperator and DeployCSI descriptions for more details
func DeployCSIComponents(f *framework.Framework, additionalInstallArgs string) (func(), error) {
	cancelLogging := collectPodLogs(f)
	cleanupOperator, err := DeployOperator(f)
	if err != nil {
		cancelLogging()
		return nil, err
	}

	cleanupCSI, err := DeployCSI(f, additionalInstallArgs)
	if err != nil {
		cancelLogging()
		cleanupOperator()
		return nil, err
	}

	return func() {
		cleanupCSI()
		cleanupOperator()
		cancelLogging()
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

// DeployCSI deploys csi-baremetal-deployment with CmdHelmExecutor
// After install - waiting all pods ready, checking kubernetes-scheduler restart
// Cleanup - deleting csi-chart, cleaning all csi custom resources
// Helm command - helm install csi-baremetal <CHARTS_DIR>/csi-baremetal-deployment
// 			--set image.tag=<CSI_VERSION>
//			--set image.pullPolicy=IfNotPresent - due to kind
//			--set driver.drivemgr.type=loopbackmgr
//			--set scheduler.log.level=debug
//			--set nodeController.log.level=debug
//			--set driver.log.level=debug
//			--set scheduler.patcher.readinessTimeout=(3) - se readiness probe has a race - kube-scheduler restores for a long time after unpatching
//															override default value here to force patching repeating
//															if kube-scheduler is not restarted
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
		// CSI Operator repeats patching after <seReadinessTimeout> if extender pod is not ready
		seReadinessTimeout = 3 // Minutes
		installArgs        = fmt.Sprintf("--set image.tag=%s "+
			"--set image.pullPolicy=IfNotPresent "+
			"--set scheduler.patcher.enable=true "+
			"--set driver.drivemgr.type=loopbackmgr "+
			"--set scheduler.log.level=debug "+
			"--set nodeController.log.level=debug "+
			"--set driver.log.level=debug "+
			"--set scheduler.patcher.readinessTimeout=%d", csiVersion, seReadinessTimeout)
		podWait         = 6 * time.Minute
		sleepBeforeWait = 10 * time.Second
	)

	if additionalInstallArgs != "" {
		installArgs += " " + additionalInstallArgs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	cleanup := func() {
		defer cancel()
		if BMDriverTestContext.CompleteUninstall {
			CleanupLoopbackDevices(ctx, f)
			// delete resources with finalizers and wait until node- and lvgcontroller reconcile requests
			removeCRs(ctx, f, CsibmnodeGVR, LVGGVR)
			deadline := time.Now().Add(30 * time.Second)
			for {
				time.Sleep(2 * time.Second)
				if !isCRInstancesExists(ctx, f, CsibmnodeGVR) && !isCRInstancesExists(ctx, f, LVGGVR) {
					break
				}
				if time.Now().After(deadline) {
					e2elog.Logf("Some csibmnodes or lvgs have not been deleted yet")
					printCRs(ctx, f, CsibmnodeGVR, LVGGVR)
					break
				}
			}
		}

		if err := helmExecutor.DeleteRelease(&chart); err != nil {
			e2elog.Logf("CSI Deployment helm chart deletion failed. Name: %s, namespace: %s", chart.name, chart.namespace)
		}

		if BMDriverTestContext.CompleteUninstall {
			// delete resources without finalizers
			removeCRs(ctx, f, ACGVR, ACRGVR, DriveGVR)
		}

		printCRs(ctx, f, VolumeGVR, CsibmnodeGVR, ACGVR, ACRGVR, LVGGVR, DriveGVR)
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

	// print info about all custom resources into log messages
	printCRs(ctx, f, CsibmnodeGVR, DriveGVR, ACGVR)

	return cleanup, nil
}

// CleanupLoopbackDevices executes in node pods drive managers containers kill -SIGHUP 1
// Returns error if it's failed to get node pods
func CleanupLoopbackDevices(ctx context.Context, f *framework.Framework) error {
	pods, err := GetNodePodsNames(ctx, f)
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
func GetNodePodsNames(ctx context.Context, f *framework.Framework) ([]string, error) {
	pods, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).List(ctx, metav1.ListOptions{})
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

// printCRs prints all CRs that were passed by type into logs using e2elog
func printCRs(ctx context.Context, f *framework.Framework, GVRs ...schema.GroupVersionResource) {
	for _, gvr := range GVRs {
		recources, err := f.DynamicClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
		if err != nil {
			e2elog.Logf("Failed to get CR list %s: %s", gvr.String(), err.Error())
		}
		e2elog.Logf("CR Type: %s", gvr.String())
		printCRList(recources.Items)
	}
}

// printCRList prints into logs list of unstructured.Unstructured
// Format: <name>string - <spec>map\n
func printCRList(list []unstructured.Unstructured) {
	for _, item := range list {
		e2elog.Logf("%s - %v", item.Object["metadata"].(map[string]interface{})["name"], item.Object["spec"])
	}
}

// removeCRs removes all CRs that were passed by type
func removeCRs(ctx context.Context, f *framework.Framework, GVRs ...schema.GroupVersionResource) {
	for _, gvr := range GVRs {
		err := f.DynamicClient.Resource(gvr).Namespace("").DeleteCollection(ctx,
			metav1.DeleteOptions{}, metav1.ListOptions{})
		if err != nil {
			e2elog.Logf("Failed to clean CR %s: %s", gvr.String(), err.Error())
		}
	}
}

func isCRInstancesExists(ctx context.Context, f *framework.Framework, GVR schema.GroupVersionResource) bool {
	recources, err := f.DynamicClient.Resource(GVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return true
	}
	return len(recources.Items) != 0
}
