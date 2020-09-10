package common

import (
	"fmt"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"sigs.k8s.io/yaml"
	"time"
)

const (
	extenderManifestsFolder = "scheduler-extender/templates/"
	schedulerLabel          = "component=kube-scheduler"
	restartWaitTimeout      = time.Minute * 2
)

func DeploySchedulerExtender(f *framework.Framework) func() {
	manifestsCleanupFunc := waitForRestart(f,
		func() func() {
			return deployManifests(f)
		})

	return func() {
		waitForRestart(f,
			func() func() {
				manifestsCleanupFunc()
				return func() {}
			})
	}
}

func waitForRestart(f *framework.Framework, fu func() func()) func() {
	wait := BMDriverTestContext.BMWaitSchedulerRestart

	rc := newSchedulerRestartChecker(f.ClientSet)
	if wait {
		framework.ExpectNoError(rc.ReadInitialState())
	}
	result := fu()
	if wait {
		e2elog.Logf("Wait for scheduler restart")
		deadline := time.Now().Add(restartWaitTimeout)
		for {
			ready, err := rc.CheckRestarted()
			framework.ExpectNoError(err)
			if ready {
				e2elog.Logf("Scheduler restarted")
				break
			}
			msg := "Scheduler restart NOT detected yet"
			e2elog.Logf(msg)
			if time.Now().After(deadline) {
				e2elog.Logf("Scheduler restart NOT detected after %d minutes. Continue.",
					restartWaitTimeout.Minutes())
				break
			}
			time.Sleep(time.Second * 5)
		}
	}
	return result
}

func deployManifests(f *framework.Framework) func() {
	manifests := []string{
		extenderManifestsFolder + "rbac.yaml",
	}

	daemonSetCleanup := buildDaemonSet(f)
	configMapCleanup := buildConfigMap(f)
	manifestsCleanupFunc, err := f.CreateFromManifests(nil, manifests...)
	framework.ExpectNoError(err)

	cleanupFunc := func() {
		configMapCleanup()
		daemonSetCleanup()
		manifestsCleanupFunc()
	}

	return cleanupFunc
}

func buildConfigMap(f *framework.Framework) func() {
	file, err := ioutil.ReadFile("/tmp/" + extenderManifestsFolder + "/patcher-configmap.yaml")
	framework.ExpectNoError(err)

	cm := &corev1.ConfigMap{}
	err = yaml.Unmarshal(file, cm)
	framework.ExpectNoError(err)

	ns := f.Namespace.Name
	f.PatchNamespace(&cm.ObjectMeta.Namespace)
	cm, err = f.ClientSet.CoreV1().ConfigMaps(ns).Create(cm)
	framework.ExpectNoError(err)
	return func() {
		if err := f.ClientSet.CoreV1().ConfigMaps(ns).Delete(cm.Name, &metav1.DeleteOptions{}); err != nil {
			e2elog.Logf("Failed to delete SE configmap %s: %v", cm.Name, err)
		}
	}
}

func buildDaemonSet(f *framework.Framework) func() {
	file, err := ioutil.ReadFile("/tmp/" + extenderManifestsFolder + "/scheduler-extender.yaml")
	framework.ExpectNoError(err)

	ds := &appsv1.DaemonSet{}
	err = yaml.Unmarshal(file, ds)
	framework.ExpectNoError(err)

	ns := f.Namespace.Name
	f.PatchNamespace(&ds.ObjectMeta.Namespace)
	ds, err = f.ClientSet.AppsV1().DaemonSets(ns).Create(ds)
	framework.ExpectNoError(err)
	return func() {
		if err := f.ClientSet.AppsV1().DaemonSets(ns).Delete(ds.Name, &metav1.DeleteOptions{}); err != nil {
			e2elog.Logf("Failed to delete SE daemonset %s: %v", ds.Name, err)
		}
	}
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
