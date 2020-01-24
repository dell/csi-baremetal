package main

import (
	"flag"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"

	v1api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"github.com/sirupsen/logrus"
)

var (
	port    = flag.Int("port", 50051, "Node Volume Manager server port")
	address = flag.String("address", "", "Node Volume Manager server ip address")
)

func main() {
	flag.Parse()
	logrus.Info("Start Node Volume Manager")
	//Temporary run insecure server
	s := base.NewServerRunner(nil, *address, int32(*port))
	v1api.RegisterVolumeManagerServer(s.GRPCServer, &node.VolumeManager{})
	csi.RegisterNodeServer(s.GRPCServer, &node.CSINodeService{})
	if err := s.RunServer(); err != nil {
		logrus.Fatalf("fail to serve: %v", err)
	}
}
