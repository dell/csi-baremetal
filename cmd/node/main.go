package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"
)

var (
	hwMgrEndpoint = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "Hardware Manager endpoint")
	volumeMgrIP   = flag.String("volumemgrip", base.DefaultVolumeManagerEndpoint, "Node Volume Manager endpoint")
	csiEndpoint   = flag.String("csiendpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	nodeID        = flag.String("nodeid", "", "node identification by k8s")
	// common logger for each component
	logger = logrus.New()
)

func main() {
	flag.Parse()

	logger.Out = os.Stdout
	logger.Info("Starting Node Service")

	// gRPC client for communication with HWMgr via TCP socket
	gRPCClient, err := base.NewClient(nil, *hwMgrEndpoint)
	if err != nil {
		logrus.Fatalf("fail to create grpc client for endpoint %s, error: %v", *hwMgrEndpoint, err)
	}
	clientToHwMgr := api.NewHWServiceClient(gRPCClient.GRPCClient)

	// gRPC server that will serve requests (node CSI) from k8s via unix socket
	csiUDSServer := base.NewServerRunner(nil, *csiEndpoint)

	csiNodeService := node.NewCSINodeService(clientToHwMgr, *nodeID)
	csiIdentityService := controller.NewIdentityServer("baremetal-csi", "0.1.0", true)
	csiNodeService.SetLogger(logger)
	// register CSI calls handler
	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiIdentityService)

	logger.Info("Starting Discovering go routine ...")
	go Discovering(csiNodeService)

	logger.Info("Starting VolumeManager server in go routine ...")
	go func() {
		if err := StartVolumeManagerServer(csiNodeService); err != nil {
			logger.Infof("VolumeManager server failed with error: %v", err)
		}
	}()

	logger.Info("Starting handle CSI calls in main thread ...")
	// handle CSI calls
	if err := csiUDSServer.RunServer(); err != nil {
		logrus.Fatalf("fail to serve: %v", err)
	}
}

func Discovering(c *node.CSINodeService) {
	for {
		time.Sleep(time.Second * 60)
		err := c.Discover()
		if err != nil {
			logger.Infof("Discover finished with error: %v", err)
		} else {
			logger.Info("Discover finished successful")
		}
	}
}

// StartVolumeManagerServer starts gRPC server to handle request from Controller Service
func StartVolumeManagerServer(c *node.CSINodeService) error {
	// gRPC server that will serve requests from controller service via tcp socket
	volumeMgrEndpoint := fmt.Sprintf("tcp://%s:%d", *volumeMgrIP, base.DefaultVolumeManagerPort)
	volumeMgrTCPServer := base.NewServerRunner(nil, volumeMgrEndpoint)
	api.RegisterVolumeManagerServer(volumeMgrTCPServer.GRPCServer, c)
	return volumeMgrTCPServer.RunServer()
}
