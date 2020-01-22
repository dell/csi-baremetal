package main

import (
	"flag"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr"
)

var (
	port = flag.Int("port", 50052, "HWManager server port")
	host = flag.String("host", "", "HWManager server IP address")
)

func main() {
	flag.Parse()
	// Server is insecure for now because credentials are nil
	serverRunner := base.NewServerRunner(nil, *host, int32(*port))
	api.RegisterHWServiceServer(serverRunner.GRPCServer, &hwmgr.HWServiceServerImpl{})
	if err := serverRunner.RunServer(); err != nil {
		logrus.Fatalf("Failed to serve on %s:%d. Error: %s", *host, *port, err.Error())
	}
}
