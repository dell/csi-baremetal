package main

import (
	"flag"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"
)

var (
	hwMgrEndpoint = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "Hardware Manager endpoint")
	volumeMgrIP   = flag.String("volumemgrip", base.DefaultVolumeManagerEndpoint, "Node Volume Manager endpoint")
	csiEndpoint   = flag.String("csiendpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	nodeID        = flag.String("nodeid", "", "node identification by k8s")
	logPath       = flag.String("logpath", "", "Log path for Node Volume Manager service")
	verboseLogs   = flag.Bool("verbose", false, "Debug mode in logs")
)

func main() {
	flag.Parse()

	var logLevel logrus.Level
	if *verboseLogs {
		logLevel = logrus.DebugLevel
	} else {
		logLevel = logrus.InfoLevel
	}

	logger, err := base.InitLogger(*logPath, logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Starting Node Service")

	// gRPC client for communication with HWMgr via TCP socket
	gRPCClient, err := base.NewClient(nil, *hwMgrEndpoint, logger)
	if err != nil {
		logger.Fatalf("fail to create grpc client for endpoint %s, error: %v", *hwMgrEndpoint, err)
	}
	clientToHwMgr := api.NewHWServiceClient(gRPCClient.GRPCClient)

	// gRPC server that will serve requests (node CSI) from k8s via unix socket
	csiUDSServer := base.NewServerRunner(nil, *csiEndpoint, logger)

	csiNodeService := node.NewCSINodeService(clientToHwMgr, *nodeID, logger)
	csiIdentityService := controller.NewIdentityServer("baremetal-csi", "0.0.2", true)

	// register CSI calls handler
	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiIdentityService)

	// TODO: implement logic for discover  AK8S-64
	// logger.Info("Starting Discovering go routine ...")
	// go Discovering(csiNodeService, logger)

	logger.Info("Starting VolumeManager server in go routine ...")
	go func() {
		if err := StartVolumeManagerServer(csiNodeService, logger); err != nil {
			logger.Infof("VolumeManager server failed with error: %v", err)
		}
	}()

	logger.Info("Starting handle CSI calls in main thread ...")
	// handle CSI calls
	if err := csiUDSServer.RunServer(); err != nil {
		logger.Fatalf("fail to serve: %v", err)
	}
}

// TODO: implement logic for discover  AK8S-64
//func Discovering(c *node.CSINodeService, logger *logrus.Logger)  {
//	for {
//		time.Sleep(time.Second * 30)
//		err := c.Discover()
//		if err != nil {
//			logger.Infof("Discover finished with error: %v", err)
//		} else {
//			logger.Info("Discover finished successful")
//		}
//	}
//}

// StartVolumeManagerServer starts gRPC server to handle request from Controller Service
func StartVolumeManagerServer(c *node.CSINodeService, logger *logrus.Logger) error {
	// gRPC server that will serve requests from controller service via tcp socket
	volumeMgrEndpoint := fmt.Sprintf("tcp://%s:%d", *volumeMgrIP, base.DefaultVolumeManagerPort)
	volumeMgrTCPServer := base.NewServerRunner(nil, volumeMgrEndpoint, logger)
	api.RegisterVolumeManagerServer(volumeMgrTCPServer.GRPCServer, c)
	// register Health checks
	logger.Info("Registering Node service health check")
	health.RegisterHealthServer(volumeMgrTCPServer.GRPCServer, c)
	return volumeMgrTCPServer.RunServer()
}
