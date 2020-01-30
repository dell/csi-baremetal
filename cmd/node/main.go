package main

import (
	"flag"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"
)

var (
	hwMgrEndpoint     = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "Hardware Manager endpoint")
	volumeMgrEndpoint = flag.String("volumetmgrendpoint", base.DefaultVolumeManagerEndpoint, "Node Volume Manager endpoint")
	discoverTimeout   = flag.Uint("dt", base.DefaultDiscoverTimeout, "Timeout between volume discovering")
)

func main() {
	flag.Parse()

	logrus.Info("Start Node Volume Manager")
	// Temporary run insecure server
	s := base.NewServerRunner(nil, *volumeMgrEndpoint)

	// create grpc client that will be communicated with HWManager
	c, err := base.NewClient(nil, *hwMgrEndpoint)
	if err != nil {
		logrus.Fatalf("fail to create grpc client: %v", err)
	}
	hwC := api.NewHWServiceClient(c.GRPCClient)

	// create VolumeManager instance based on grpc client
	vm := node.NewVolumeManager(hwC, &base.Executor{})

	api.RegisterVolumeManagerServer(s.GRPCServer, vm)

	go func(timeout uint) {
		for {
			time.Sleep(time.Duration(timeout) * time.Second) // TODO: wait until hwmgl will be up
			err := vm.Discover()
			if err != nil {
				logrus.WithField("method", "VolumeManager.Discover()").Errorf("Failed with: %v", err)
			}
		}
	}(*discoverTimeout)

	csi.RegisterNodeServer(s.GRPCServer, &node.CSINodeService{})

	if err := s.RunServer(); err != nil {
		logrus.Fatalf("fail to serve: %v", err)
	}
}
