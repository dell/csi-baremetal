package common

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	pode2e "k8s.io/kubernetes/test/e2e/framework/pod"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/command"
)

var (
	utilExecutor command.CmdExecutor

	lvgGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "lvgs",
	}
)

// init initializes utilExecutor
func init() {
	logger := logrus.New()
	utilExecutor = &command.Executor{}
	utilExecutor.SetLogger(logger)
}

// CleanupAfterCustomTest cleanups all resources related to CSI plugin and plugin as well
// This function deletes pod if it was installed during test. And waits for its correct deletion to perform
// NodeUnpublish and NodeUnstage properly. Next it deletes PVC and waits for correctly deletion of bounded PV
// to clear device for next tests (CSI performs wipefs during PV deletion). The last step is the deletion of driver.
func CleanupAfterCustomTest(f *framework.Framework, driverCleanupFn func(), pod *corev1.Pod, pvc []*corev1.PersistentVolumeClaim) {
	e2elog.Logf("Starting cleanup")
	var err error

	if pod != nil {
		e2elog.Logf("Deleting Pod %s", pod.Name)
		err = pode2e.DeletePodWithWait(f.ClientSet, pod)
		if err != nil {
			e2elog.Logf("Failed to delete pod %s: %v", pod.Name, err)
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

	//// need to clean-up logical volume group CRs until https://jira.cec.lab.emc.com:8443/browse/AK8S-1178 is resolved
	//if err := f.DynamicClient.Resource(lvgGVR).Namespace(f.Namespace.Name).DeleteCollection(
	//	metav1.NewDeleteOptions(0), metav1.ListOptions{}); err != nil {
	//	e2elog.Logf("failed to delete lvg CRs, error: %v", err)
	//}

	// Removes all driver's manifests installed during init(). (Driver, its RBACs, SC)
	if driverCleanupFn != nil {
		driverCleanupFn()
		driverCleanupFn = nil
	}
	e2elog.Logf("Cleanup finished.")
}

// GetDockerContainers returns slice of string each of which is represented
// particular docker container and has next format:
// CONTAINER_NAME:CONTAINER_ID:CONTAINER_STATUS
func GetDockerContainers() ([]string, error) {
	cmd := fmt.Sprintf("docker ps --format={{.Names}}:{{.ID}}:{{.Status}}")

	stdout, _, err := utilExecutor.RunCmd(cmd)
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSpace(stdout), "\n"), nil
}

// DeleteResource will delete some resource and waits until it all be deleted.
func DeleteResource(dc dynamic.Interface, resource schema.GroupVersionResource, ns string, poll, timeout time.Duration) error {
	// need to clean-up logical volume group CRs until https://jira.cec.lab.emc.com:8443/browse/AK8S-1178 is resolved
	err := dc.Resource(lvgGVR).Namespace(ns).DeleteCollection(metav1.NewDeleteOptions(0), metav1.ListOptions{})
	if err != nil {
		return err
	}
	// wait until it will all be deleted
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		lvgs, err := dc.Resource(resource).Namespace(ns).List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(lvgs.Items) == 0 {
			return nil
		}
	}
	return fmt.Errorf("apparently resource %v is still exists after deletion", resource)
}
