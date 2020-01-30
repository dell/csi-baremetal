package main

import (
	"flag"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"
)

var (
	//hwMgrEndpoint     = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "Hardware Manager endpoint")
	//volumeMgrEndpoint = flag.String("volumetmgrendpoint", base.DefaultVolumeManagerEndpoint, "Node Volume Manager endpoint")
	csiEndpoint = flag.String("csiendpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	//discoverTimeout   = flag.Uint("dt", base.DefaultDiscoverTimeout, "Timeout between volume discovering")
	nodeID = flag.String("nodeid", "", "node identification by k8s")
)

func main() {
	flag.Parse()

	logrus.Info("Start Node Volume Manager")
	//// Temporary run insecure server
	//s := base.NewServerRunner(nil, *volumeMgrEndpoint)
	//
	//// create grpc client that will be communicated with HWManager
	//c, err := base.NewClient(nil, *hwMgrEndpoint)
	//if err != nil {
	//	logrus.Fatalf("fail to create grpc client: %v", err)
	//}
	//hwC := api.NewHWServiceClient(c.GRPCClient)
	//
	//// create VolumeManager instance based on grpc client
	//vm := node.NewVolumeManager(hwC, &base.Executor{})
	//
	//api.RegisterVolumeManagerServer(s.GRPCServer, vm)

	//logrus.Info("Starting Discover from main")
	//go func(timeout uint) {
	//	for {
	//		time.Sleep(time.Duration(timeout) * time.Second) // TODO: wait until hwmgl will be up
	//		err := vm.Discover()
	//		if err != nil {
	//			logrus.WithField("method", "VolumeManager.Discover()").Errorf("Failed with: %v", err)
	//		}
	//	}
	//}(*discoverTimeout)
	//
	//logrus.Info("Starting VolumeManager server from main")
	//go func() {
	//	logrus.Info("Starting VolumeManager server")
	//	if err := s.RunServer(); err != nil {
	//		logrus.Fatalf("fail to serve: %v", err)
	//	}
	//}()

	logrus.Info("Create CSI services")

	csiS := base.NewServerRunner(nil, *csiEndpoint)

	csi.RegisterNodeServer(csiS.GRPCServer, &node.CSINodeService{NodeID: *nodeID})

	csiIdentityService := controller.NewIdentityServer("baremetal-csi", "0.1.0", true)
	csi.RegisterIdentityServer(csiS.GRPCServer, csiIdentityService)

	logrus.Info("Starting CSI UDS server from main")
	//go func() {
	//	logrus.Info("Starting CSI UDS server for sidecar")
	//	if err := csiS.RunServer(); err != nil {
	//		logrus.Fatalf("csi sidecar server: fail to serve: %v", err)
	//	}
	//}()
	if err := csiS.RunServer(); err != nil {
		logrus.Fatalf("fail to serve: %v", err)
	}
}
