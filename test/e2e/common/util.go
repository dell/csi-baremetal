package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	pode2e "k8s.io/kubernetes/test/e2e/framework/pod"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
)

var (
	utilExecutor command.CmdExecutor
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
func CleanupAfterCustomTest(f *framework.Framework,
	driverCleanupFn func(),
	pod *corev1.Pod,
	pvc *corev1.PersistentVolumeClaim) {
	e2elog.Logf("Starting cleanup")
	var err error

	if pod != nil {
		e2elog.Logf("Deleting Pod %s", pod.Name)
		err = pode2e.DeletePodWithWait(f.ClientSet, pod)
		if err != nil {
			e2elog.Logf("Failed to delete pod %s: %v", pod.Name, err)
		}
	}

	if pvc != nil {
		e2elog.Logf("Deleting PVC %s", pvc.Name)
		pv, _ := framework.GetBoundPV(f.ClientSet, pvc)
		err := framework.DeletePersistentVolumeClaim(f.ClientSet, pvc.Name, f.Namespace.Name)
		if err != nil {
			e2elog.Logf("failed to delete pvc %v", err)
		}
		if pv != nil {
			// wait for pv deletion to clear devices for future tests
			err = framework.WaitForPersistentVolumeDeleted(f.ClientSet, pv.Name, 5*time.Second, 2*time.Minute)
			if err != nil {
				e2elog.Logf("unable to delete PV %s, ignore that error", pv.Name)
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
