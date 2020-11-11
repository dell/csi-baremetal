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
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	pode2e "k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
)

var utilExecutor command.CmdExecutor

// GetExecutor initialize or just return utilExecutor
func GetExecutor() command.CmdExecutor {
	if utilExecutor == nil {
		// TODO: workaround until https://github.com/dell/csi-baremetal/issues/83 is open
		_ = os.Setenv("LOG_FORMAT", "text")
		logger, _ := base.InitLogger("", "debug")
		utilExecutor = &command.Executor{}
		utilExecutor.SetLogger(logger)
	}

	return utilExecutor
}

// CleanupAfterCustomTest cleanups all resources related to CSI plugin and plugin as well
// This function deletes pods if were created during test. And waits for its correct deletion to perform
// NodeUnpublish and NodeUnstage properly. Next it deletes PVC and waits for correctly deletion of bounded PV
// to clear device for next tests (CSI performs wipefs during PV deletion). The last step is the deletion of driver.
func CleanupAfterCustomTest(f *framework.Framework, driverCleanupFn func(), pod []*corev1.Pod, pvc []*corev1.PersistentVolumeClaim) {
	e2elog.Logf("Starting cleanup")
	var err error

	for _, p := range pod {
		e2elog.Logf("Deleting Pod %s", p.Name)
		err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(p.Name, nil)
		if err != nil {
			if apierrs.IsNotFound(err) {
				continue
			}
			e2elog.Logf("Failed to delete pod %s: %v", p.Name, err)
		}
	}
	for _, p := range pod {
		e2elog.Logf("Wait up to %v for pod %q to be fully deleted", pode2e.PodDeleteTimeout, p.Name)
		err = pode2e.WaitForPodNotFoundInNamespace(f.ClientSet, p.Name, f.Namespace.Name, time.Minute*2)
		if err != nil {
			e2elog.Logf("Failed to delete pod %s: %v", p.Name, err)
		}
	}

	// to speed up we need to delete PVC in parallel
	pvs := []*corev1.PersistentVolume{}
	for _, claim := range pvc {
		e2elog.Logf("Deleting PVC %s", claim.Name)
		pv, _ := framework.GetBoundPV(f.ClientSet, claim)
		err := framework.DeletePersistentVolumeClaim(f.ClientSet, claim.Name, f.Namespace.Name)
		if err != nil {
			e2elog.Logf("failed to delete pvc, error: %v", err)
		}
		// add pv to the list
		if pv != nil {
			pvs = append(pvs, pv)
		}
	}

	// wait for pv deletion to clear devices for future tests
	for _, pv := range pvs {
		err = framework.WaitForPersistentVolumeDeleted(f.ClientSet, pv.Name, 5*time.Second, 2*time.Minute)
		if err != nil {
			e2elog.Logf("unable to delete PV %s, ignore that error", pv.Name)
		}
	}
	// wait for SC deletion
	storageClasses, err := f.ClientSet.StorageV1().StorageClasses().List(metav1.ListOptions{})
	if err != nil {
		e2elog.Logf("failed to read SC list, error: %v", err)
	} else {
		for _, sc := range storageClasses.Items {
			if !strings.HasPrefix(sc.Name, f.Namespace.Name) {
				continue
			}
			err = f.ClientSet.StorageV1().StorageClasses().Delete(sc.Name, &metav1.DeleteOptions{})
			if err != nil {
				e2elog.Logf("failed to remove SC, error: %v", err)
			}
		}
	}

	// Removes all driver's manifests installed during init(). (Driver, its RBACs, SC)
	if driverCleanupFn != nil {
		driverCleanupFn()
		driverCleanupFn = nil
	}
	e2elog.Logf("Cleanup finished.")
}

func GetGlobalClientSet() (clientset.Interface, error) {
	conf, err := framework.LoadConfig()
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(conf)
}
