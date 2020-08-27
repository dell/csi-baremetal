package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	pode2e "k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/dell/csi-baremetal/pkg/base/command"
)

var utilExecutor command.CmdExecutor

// init initializes utilExecutor
func init() {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	utilExecutor = &command.Executor{}
	utilExecutor.SetLogger(logger)
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
		err = pode2e.DeletePodWithWait(f.ClientSet, p)
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
