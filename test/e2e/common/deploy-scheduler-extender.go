package common

import (
	"errors"
	"io/ioutil"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"sigs.k8s.io/yaml"

	"github.com/dell/csi-baremetal/pkg/base/util"
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

// WaitUntilSchedulerRestartsWithConfig checks whether kube-scheduler pod is running with new config or not
// within attempts and interval between them
func WaitUntilSchedulerRestartsWithConfig(attempts int, interval time.Duration, f *framework.Framework) error {
	for i := 0; i < attempts; i++ {
		pods, err := f.ClientSet.CoreV1().Pods("kube-system").
			List(metav1.ListOptions{
				LabelSelector: "component=kube-scheduler",
			})

		if err != nil {
			e2elog.Logf("Unable to get pods list: %v", err)
			return err
		}

		if len(pods.Items) != 0 {
			pod := pods.Items[0] // expect only one scheduler pod
			if util.ContainsString(pod.Spec.Containers[0].Command, "--config=/etc/kubernetes/scheduler/config.yaml") {
				return nil
			}
		}

		time.Sleep(interval)
	}

	return errors.New("kube-scheduler isn't running with new config")
}
