package sanity_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/coreos/rkt/tests/testutils/logger"
	"github.com/kubernetes-csi/csi-test/v3/pkg/sanity"
	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
)

var (
	controllerEndpoint = "unix:///tmp/controller.sock"
	nodeEndpoint       = "unix:///tmp/node.sock"
	targetSanityPath   = os.TempDir() + "/csi-mount/target"
	stagingSanityPath  = os.TempDir() + "/csi-staging"
	driverName         = "baremetal-csi-plugin"
	version            = "test"
	testNs             = "default"
	nodeId             = "localhost"

	testDrives = []*api.Drive{
		{
			UUID:         "uuid-1",
			SerialNumber: "hdd1",
			Size:         1024 * 1024 * 1024 * 500,
			Health:       apiV1.HealthGood,
			Status:       apiV1.DriveStatusOnline,
			Path:         "/dev/sda",
			Type:         apiV1.DriveTypeHDD,
		},
		{
			UUID:         "uuid-2",
			SerialNumber: "hdd2",
			Size:         1024 * 1024 * 1024 * 200,
			Health:       apiV1.HealthGood,
			Status:       apiV1.DriveStatusOnline,
			Path:         "/dev/sdb",
			Type:         apiV1.DriveTypeHDD,
		},
	}
)

func skipIfNotSanity(t *testing.T) {
	if os.Getenv("SANITY") == "" {
		t.Skip("Skipping Sanity testing")
	}
}

func TestDriverWithSanity(t *testing.T) {
	skipIfNotSanity(t)

	// Node and Controller must share Fake k8s client because sanity tests don't run under k8s env.
	kubeClient, err := k8s.GetFakeKubeClient(testNs, logrus.New())
	if err != nil {
		panic(err)
	}

	nodeReady := make(chan bool)
	defer close(nodeReady)

	go newNodeSvc(kubeClient, nodeReady)

	// wait until Node become initialized
	<-nodeReady

	go newControllerSvc(kubeClient)

	config := sanity.NewTestConfig()
	config.Address = nodeEndpoint
	config.ControllerAddress = controllerEndpoint
	config.JUnitFile = "report.xml"

	// Call sanity test suite
	sanity.Test(t, config)
}

func newControllerSvc(kubeClient *k8s.KubeClient) {
	ll, _ := base.InitLogger("", logrus.InfoLevel)

	controllerService := controller.NewControllerService(kubeClient, ll)

	csiControllerServer := rpc.NewServerRunner(nil, controllerEndpoint, ll)

	csi.RegisterIdentityServer(csiControllerServer.GRPCServer, controller.NewIdentityServer(driverName, version, true))
	csi.RegisterControllerServer(csiControllerServer.GRPCServer, controllerService)

	ll.Info("Starting CSIControllerService")
	if err := csiControllerServer.RunServer(); err != nil {
		ll.Fatalf("fail to serve, error: %v", err)
		os.Exit(1)
	}
}

func newNodeSvc(kubeClient *k8s.KubeClient, nodeReady chan<- bool) {
	ll, _ := base.InitLogger("", logrus.InfoLevel)

	csiNodeService := prepareNodeMock(kubeClient, ll)
	csiIdentityService := controller.NewIdentityServer(driverName, version, true)

	csiUDSServer := rpc.NewServerRunner(nil, nodeEndpoint, ll)

	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiIdentityService)

	go func() {
		var doOnce sync.Once
		for range time.Tick(10 * time.Second) {
			err := csiNodeService.Discover()
			if err != nil {
				ll.Fatalf("Discover failed: %v", err)
			}
			doOnce.Do(func() {
				nodeReady <- true
			})
		}
	}()

	go imitateVolumeManagerReconcile(kubeClient)

	ll.Info("Starting CSINodeService")
	if err := csiUDSServer.RunServer(); err != nil {
		logger.Fatalf("fail to serve: %v", err)
	}
}

// prepareNodeMock prepares instance of Node service with mocked hwmgr and mocked executor
func prepareNodeMock(kubeClient *k8s.KubeClient, log *logrus.Logger) *node.CSINodeService {
	c := mocks.NewMockHWMgrClient(testDrives)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{fmt.Sprintf(lsblk.CmdTmpl, ""): {Stdout: mocks.LsblkTwoDevicesStr}})
	e.SetSuccessIfNotFound(true)

	nodeService := node.NewCSINodeService(nil, nodeId, log, kubeClient)

	nodeService.VolumeManager = *node.NewVolumeManager(c, e, log, kubeClient, nodeId)

	scImplMock := &sc.ImplementerMock{}
	scImplMock.On("DeleteFileSystem", "/dev/sda").Return(nil)
	scImplMock.On("DeleteFileSystem", "/dev/sdb").Return(nil)
	scImplMock.On("PrepareVolume", "/dev/sda1", targetSanityPath).Return(false, nil)
	scImplMock.On("PrepareVolume", "/dev/sdb1", targetSanityPath).Return(false, nil)
	scImplMock.On("PrepareVolume", "/dev/sda1", stagingSanityPath).Return(false, nil)
	scImplMock.On("PrepareVolume", "/dev/sdb1", stagingSanityPath).Return(false, nil)
	scImplMock.On("IsMountPoint", targetSanityPath).Return(true, nil)
	scImplMock.On("Unmount", stagingSanityPath).Return(nil)
	scImplMock.On("Unmount", targetSanityPath).Return(nil)
	nodeService.SetSCImplementer("hdd", scImplMock)

	return nodeService
}

// imitateVolumeManagerReconcile imitates working of VolumeManager's Reconcile loop under not k8s env.
func imitateVolumeManagerReconcile(kubeClient *k8s.KubeClient) {
	for range time.Tick(10 * time.Second) {
		volumes := &vcrd.VolumeList{}
		_ = kubeClient.ReadList(context.Background(), volumes)
		for _, v := range volumes.Items {
			if v.Spec.CSIStatus == apiV1.Creating {
				v.Spec.CSIStatus = apiV1.Created
				_ = kubeClient.UpdateCRWithAttempts(context.Background(), &v, 5)
			}
			if v.Spec.CSIStatus == apiV1.Removing {
				v.Spec.CSIStatus = apiV1.Removed
				_ = kubeClient.UpdateCRWithAttempts(context.Background(), &v, 5)
			}
		}
	}
}
