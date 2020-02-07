package main

import (
	"flag"
	"fmt"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr"
)

var (
	endpoint    = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "HWManager Endpoint")
	logPath     = flag.String("logpath", "", "Log path for HWManager")
	verboseLogs = flag.Bool("verbose", false, "Debug mode in logs")
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
		fmt.Printf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Start HWManager")
	// Server is insecure for now because credentials are nil
	serverRunner := base.NewServerRunner(nil, *endpoint, logger)

	hwServiceServer := &hwmgr.HWServiceServerImpl{}
	hwServiceServer.SetLogger(logger)

	api.RegisterHWServiceServer(serverRunner.GRPCServer, hwServiceServer)

	if err := serverRunner.RunServer(); err != nil {
		logger.Fatalf("Failed to serve on %s. Error: %s", *endpoint, err.Error())
	}
}
