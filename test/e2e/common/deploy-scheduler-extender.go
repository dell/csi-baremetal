package common

import (
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"sigs.k8s.io/yaml"
)

const (
	extenderManifestsFolder = "csi-baremetal-se/templates/"
)

func DeploySchedulerExtender(f *framework.Framework) func() {
	return deployManifests(f)
}

func deployManifests(f *framework.Framework) func() {
	manifests := []string{
		extenderManifestsFolder + "rbac.yaml",
	}
	file, err := ioutil.ReadFile("/tmp/" + extenderManifestsFolder + "/scheduler-extender.yaml")
	framework.ExpectNoError(err)

	deployment := &appsv1.Deployment{}
	err = yaml.Unmarshal(file, deployment)
	framework.ExpectNoError(err)

	ns := f.Namespace.Name
	f.PatchNamespace(&deployment.ObjectMeta.Namespace)
	deployment, err = f.ClientSet.AppsV1().Deployments(ns).Create(deployment)
	framework.ExpectNoError(err)

	manifestsCleanupFunc, err := f.CreateFromManifests(nil, manifests...)
	framework.ExpectNoError(err)

	cleanupFunc := func() {
		if err := f.ClientSet.AppsV1().Deployments(ns).Delete(deployment.Name, &metav1.DeleteOptions{}); err != nil {
			e2elog.Logf("Failed to delete SE deployment %s: %v", deployment.Name, err)
		}
		manifestsCleanupFunc()
	}

	return cleanupFunc
}
