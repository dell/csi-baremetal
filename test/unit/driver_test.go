package unit

import (
	"os/exec"
	"testing"

	"github.com/kubernetes-csi/csi-test/pkg/sanity"
)

func TestDriver(t *testing.T) {
	//TODO-8110 Investigate how to run driver properly
	go exec.Command("./../../build/_output/baremetal_csi", "--startrest=false", "--nodeid=test").Run()

	config := &sanity.Config{
		TargetPath:  "/tmp/target_path",
		StagingPath: "/tmp/staging_path",
		Address:     "unix:///tmp/csi.sock",
	}
	//TODO-8110 Tests fail. Because of REST server that monitors disks though k8s API. Driver doesn't work without k8s cluster
	sanity.Test(t, config)
}
