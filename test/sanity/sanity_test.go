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
	"github.com/google/uuid"
	"github.com/kubernetes-csi/csi-test/v3/pkg/sanity"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/controller"
	"github.com/dell/csi-baremetal/pkg/mocks"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
	"github.com/dell/csi-baremetal/pkg/mocks/provisioners"
	"github.com/dell/csi-baremetal/pkg/node"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
)

var (
	controllerEndpoint = "unix:///tmp/controller.sock"
	nodeEndpoint       = "unix:///tmp/node.sock"
	targetSanityPath   = os.TempDir() + "/csi-mount/target"
	stagingSanityPath  = os.TempDir() + "/csi-staging"
	driverName         = "csi-baremetal-driver"
	version            = "test"
	testNs             = "default"
	nodeId             = "localhost"
	nodeName           = "node-name"

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
	config.RemoveStagingPath = os.RemoveAll
	config.Address = nodeEndpoint
	config.ControllerAddress = controllerEndpoint
	config.JUnitFile = "report.xml"

	// Call sanity test suite
	sanity.Test(t, config)
}

func newControllerSvc(kubeClient *k8s.KubeClient) {
	ll, _ := base.InitLogger("", base.DebugLevel)

	controllerService := controller.NewControllerService(kubeClient, ll, featureconfig.NewFeatureConfig())

	csiControllerServer := rpc.NewServerRunner(nil, controllerEndpoint, false, ll)

	csi.RegisterIdentityServer(csiControllerServer.GRPCServer, controller.NewIdentityServer(driverName, version))
	csi.RegisterControllerServer(csiControllerServer.GRPCServer, controllerService)

	ll.Info("Starting CSIControllerService")
	if err := csiControllerServer.RunServer(); err != nil {
		ll.Fatalf("fail to serve, error: %v", err)
		os.Exit(1)
	}
}

func newNodeSvc(kubeClient *k8s.KubeClient, nodeReady chan<- bool) {
	ll, _ := base.InitLogger("", base.DebugLevel)

	csiNodeService := prepareNodeMock(kubeClient, ll)

	csiUDSServer := rpc.NewServerRunner(nil, nodeEndpoint, false, ll)

	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiNodeService)

	go func() {
		var doOnce sync.Once
		for range time.Tick(10 * time.Second) {
			err := csiNodeService.Discover()
			if err != nil {
				ll.Fatalf("Discover failed: %v", err)
			}
			doOnce.Do(func() {
				drives := &drivecrd.DriveList{}
				_ = kubeClient.ReadList(context.Background(), drives)
				for _, d := range drives.Items {
					name := uuid.New().String()
					location := d.Name
					var ac = accrd.AvailableCapacity{}
					err = kubeClient.ReadCR(context.Background(), location, "", &ac)
					if k8serrors.IsNotFound(err) {
						acCR := kubeClient.ConstructACCR(name, api.AvailableCapacity{
							Location:     d.Spec.UUID,
							NodeId:       d.Spec.NodeId,
							StorageClass: d.Spec.Type,
							Size:         d.Spec.Size,
						})
						if err := kubeClient.CreateCR(context.Background(), name, acCR); err != nil {
							logger.Errorf("unable to create AC, error: %v", err)
						}
					}
				}
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

// prepareNodeMock prepares instance of Node service with mocked drivemgr and mocked executor
func prepareNodeMock(kubeClient *k8s.KubeClient, log *logrus.Logger) *node.CSINodeService {
	c := mocks.NewMockDriveMgrClient(testDrives)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{fmt.Sprintf(lsblk.CmdTmpl, ""): {Stdout: mocks.LsblkTwoDevicesStr}})
	e.SetSuccessIfNotFound(true)

	nodeService := node.NewCSINodeService(nil, nodeId, nodeName, log, kubeClient, kubeClient,
		new(mocks.NoOpRecorder), featureconfig.NewFeatureConfig())

	nodeService.VolumeManager = *node.NewVolumeManager(c, e, log, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeId, nodeName)

	listBlk := mocklu.GetMockWrapLsblk("/some/path")
	nodeService.VolumeManager.SetListBlk(listBlk)

	pMock := provisioners.GetMockProvisionerSuccess("/some/path")
	nodeService.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

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
