package common

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"sigs.k8s.io/yaml"
)

const (
	extenderManifestsFolder = "scheduler-extender/templates/"
)

func DeploySchedulerExtender(f *framework.Framework) func() {
	return deployManifests(f)
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

// selector - component=kube-scheduler
func isSchedulerRunsWithNewConfig() (bool, error) {
	kubeSystemNS := "kube-system"
	podName, err := getPodNameBySelector(kubeSystemNS, "component=kube-scheduler")
	if podName == "" || err != nil {
		return false, fmt.Errorf("unable to determine kube-scheduler pod name. Error: %v", err)
	}

	/**
		output will be a list of command section for first container from scheduler pod manifest and looks like:
		~$ kubectl get pod kube-scheduler-qwe -n kube-system -o jsonpath='{.spec.containers[0].command}'
		[kube-scheduler --kubeconfig=/etc/kubernetes/scheduler.conf --leader-elect=true]
	**/
	cmd := fmt.Sprintf("kubectl get pod %s -n %s -o jsonpath='{.spec.containers[0].command}'",
		podName, kubeSystemNS)

	strOut, _, err := utilExecutor.RunCmd(cmd)
	if err != nil {
		return false, fmt.Errorf("unable to read scheduler pod (%s) config: %v", podName, err)
	}

	return strings.Contains(strOut, "kubeconfig=/etc/kubernetes/scheduler.conf"), nil
}

func getPodNameBySelector(namespace, selector string) (string, error) {
	cmd := fmt.Sprintf("kubectl get pods -n %s --no-headers %s", namespace, selector)
	strOut, _, err := utilExecutor.RunCmd(cmd)
	if err != nil {
		return "", err
	}

	pods := strings.Split(strOut, "\n")
	if len(pods) == 0 {
		return "", nil
	}

	return strings.Fields(pods[0])[0], nil
}

func WaitUntilSchedulerRestartsWithConfig(attempts int, timeout time.Duration) error {
	for i := 0; i < attempts; i++ {
		useCfg, err := isSchedulerRunsWithNewConfig()
		if useCfg {
			return nil
		}
		if err != nil {
			e2elog.Logf("unable to determine whether kube-scheduler runs with new config or not: %v", err)
		}
	}

	return errors.New("kube-scheduler isn't running with new config")
}
