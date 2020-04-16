package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr/halmgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr/idracmgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr/loopbackmgr"
)

var (
	endpoint    = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "HWManager Endpoint")
	logPath     = flag.String("logpath", "", "log path for HWManager")
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

	hwManager, err := chooseHWManager(logger)
	if err != nil {
		logger.Fatalf("Failed to create HW manager: %s", err.Error())
	}

	hwServiceServer := hwmgr.NewHWServer(logger, hwManager)

	api.RegisterHWServiceServer(serverRunner.GRPCServer, &hwServiceServer)

	if err := serverRunner.RunServer(); err != nil {
		logger.Fatalf("Failed to serve on %s. Error: %s", *endpoint, err.Error())
	}
}

/*
Pick HW manager implementation based on environment variable.
*/
func chooseHWManager(logger *logrus.Logger) (hwmgr.HWManager, error) {
	e := &base.Executor{}
	e.SetLogger(logger)

	switch os.Getenv("HW_MANAGER") {
	case hwmgr.REDFISH:
		linuxUtils := base.NewLinuxUtils(e, logger)
		ip := linuxUtils.GetBmcIP()
		if ip == "" {
			return nil, status.Error(codes.Internal, "IDRAC IP is not found")
		}
		return idracmgr.NewIDRACManager(logger, 10*time.Second, "root", "passwd", ip), nil
	case hwmgr.TEST:
		hwManager := loopbackmgr.NewLoopBackManager(e, logger)
		// initialize
		err := hwManager.Init()
		if err != nil {
			logger.Errorf("Failed to initialize HW manager: %s", err.Error())
			return nil, err
		}
		return hwManager, nil
	default:
		// use HAL manager by default
		return halmgr.NewHALManager(logger), nil
	}
}
