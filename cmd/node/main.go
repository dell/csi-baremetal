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
	port            = flag.Int("port", base.DefaultVolumeManagerPort, "Node Volume Manager server port")
	host            = flag.String("host", base.DefaultVolumeManagerHost, "Node Volume Manager server ip host")
	hwMgrServerPort = flag.Int("hw-port", base.DefaultHWMgrPort, "Port which HW manager is listening")
	hwMgrServerHost = flag.String("hw-host", base.DefaultHWMgrHost, "Address on which HW manager is running")
	discoverTimeout = flag.Uint("dt", base.DefaultDiscoverTimeout, "Timeout between volume discovering")
)

func main() {
	flag.Parse()

	logrus.Info("Start Node Volume Manager")
	// Temporary run insecure server
	s := base.NewServerRunner(nil, *host, int32(*port))

	// create grpc client that will be communicated with HWManager
	c, err := base.NewClient(nil, *hwMgrServerHost, string(*hwMgrServerPort))
	if err != nil {
		logrus.Fatalf("fail to create grpc client: %v", err)
	}
	hwC := api.NewHWServiceClient(c.GRPCClient)

	// create VolumeManager instance based on grpc client
	vm := node.NewVolumeManager(hwC)

	api.RegisterVolumeManagerServer(s.GRPCServer, vm)

	go func(timeout uint) {
		for {
			err := vm.Discover()
			if err != nil {
				logrus.WithField("method", "VolumeManager.Discover()").Errorf("Failed with: %v", err)
			}
			time.Sleep(time.Duration(timeout) * time.Second)
		}
	}(*discoverTimeout)

	csi.RegisterNodeServer(s.GRPCServer, &node.CSINodeService{})

	if err := s.RunServer(); err != nil {
		logrus.Fatalf("fail to serve: %v", err)
	}
}
