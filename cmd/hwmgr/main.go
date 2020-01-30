package main

import (
	"flag"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr"
)

var (
	endpoint = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "HWManager Endpoint")
)

func main() {
	flag.Parse()
	logrus.Info("Start HWManager")
	// Server is insecure for now because credentials are nil
	serverRunner := base.NewServerRunner(nil, *endpoint)
	api.RegisterHWServiceServer(serverRunner.GRPCServer, &hwmgr.HWServiceServerImpl{})
	if err := serverRunner.RunServer(); err != nil {
		logrus.Fatalf("Failed to serve on %s. Error: %s", *endpoint, err.Error())
	}
}
