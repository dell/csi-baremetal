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
	"context"
	"fmt"
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
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/logger"
)

var utilExecutor command.CmdExecutor

// GetExecutor initialize or just return utilExecutor
func GetExecutor() command.CmdExecutor {
	if utilExecutor == nil {
		// TODO: workaround until https://github.com/dell/csi-baremetal/issues/83 is open
		_ = os.Setenv("LOG_FORMAT", "text")
		logger, _ := logger.InitLogger("", "debug")
		utilExecutor = command.NewExecutor(logger)
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
		err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.TODO(), p.Name, metav1.DeleteOptions{})
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
		pv, _ := GetBoundPV(context.TODO(), f.ClientSet, claim)
		// Get the bound PV
		err := e2epv.DeletePersistentVolumeClaim(f.ClientSet, claim.Name, f.Namespace.Name)
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
		err = e2epv.WaitForPersistentVolumeDeleted(f.ClientSet, pv.Name, 5*time.Second, 2*time.Minute)
		if err != nil {
			e2elog.Logf("unable to delete PV %s, ignore that error", pv.Name)
		}
	}
	// wait for SC deletion
	storageClasses, err := f.ClientSet.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		e2elog.Logf("failed to read SC list, error: %v", err)
	} else {
		for _, sc := range storageClasses.Items {
			if !strings.HasPrefix(sc.Name, f.Namespace.Name) {
				continue
			}
			err = f.ClientSet.StorageV1().StorageClasses().Delete(context.TODO(), sc.Name, metav1.DeleteOptions{})
			if err != nil {
				e2elog.Logf("failed to remove SC, error: %v", err)
			}
		}
	}

	// Removes CSI Baremetal
	if driverCleanupFn != nil {
		driverCleanupFn()
		driverCleanupFn = nil
	}
	e2elog.Logf("Cleanup finished.")
}

// GetBoundPV returns a PV details.
func GetBoundPV(ctx context.Context, client clientset.Interface, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
	// Get new copy of the claim
	claim, err := client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Get the bound PV
	pv, err := client.CoreV1().PersistentVolumes().Get(ctx, claim.Spec.VolumeName, metav1.GetOptions{})
	return pv, err
}

// WaitForStatefulSetReplicasReady waits for all replicas of a StatefulSet to become ready or until timeout occurs, whichever comes first.
func WaitForStatefulSetReplicasReady(ctx context.Context, statefulSetName, ns string, c clientset.Interface, Poll, timeout time.Duration) error {
	e2elog.Logf("Waiting up to %v for StatefulSet %s to have all replicas ready", timeout, statefulSetName)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(Poll) {
		sts, err := c.AppsV1().StatefulSets(ns).Get(ctx, statefulSetName, metav1.GetOptions{})
		if err != nil {
			e2elog.Logf("Get StatefulSet %s failed, ignoring for %v: %v", statefulSetName, Poll, err)
			continue
		}
		if sts.Status.ReadyReplicas == *sts.Spec.Replicas {
			e2elog.Logf("All %d replicas of StatefulSet %s are ready. (%v)", sts.Status.ReadyReplicas, statefulSetName, time.Since(start))
			return nil
		}
		e2elog.Logf("StatefulSet %s found but there are %d ready replicas and %d total replicas.", statefulSetName, sts.Status.ReadyReplicas, *sts.Spec.Replicas)
	}
	return fmt.Errorf("StatefulSet %s still has unready pods within %v", statefulSetName, timeout)
}
