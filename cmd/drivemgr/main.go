// Package for main function of DriveManager
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/drivemgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/drivemgr/halmgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/drivemgr/idracmgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/drivemgr/loopbackmgr"
)

var (
	endpoint    = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "DriveManager Endpoint")
	logPath     = flag.String("logpath", "", "log path for DriveManager")
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

	logger.Info("Start DriveManager")
	// Server is insecure for now because credentials are nil
	serverRunner := rpc.NewServerRunner(nil, *endpoint, logger)

	driveManager, cleanup, err := chooseDriveManager(logger)

	if err != nil {
		logger.Fatalf("Failed to create drive manager: %s", err.Error())
	}

	driveServiceServer := drivemgr.NewDriveServer(logger, driveManager)

	api.RegisterDriveServiceServer(serverRunner.GRPCServer, &driveServiceServer)

	go util.SetupSignalHandler(serverRunner)

	if err := serverRunner.RunServer(); err != nil && err != grpc.ErrServerStopped {
		logger.Fatalf("Failed to serve on %s. Error: %s", *endpoint, err.Error())
	}

	logger.Info("Got SIGTERM signal")
	// clean loop devices after drivemgr deletion
	// using defer is the bad practice because defer isn't invoking during SIGTERM or SIGINT
	// kubernetes sends SIGTERM signal to containers for pods terminating
	if cleanup != nil {
		cleanup()
	}
}

// chooseDriveManager picks Drive manager implementation based on environment variable.
func chooseDriveManager(logger *logrus.Logger) (drivemgr.DriveManager, func(), error) {
	e := &command.Executor{}
	e.SetLogger(logger)

	switch os.Getenv("DRIVEMGR_MANAGER") {
	case drivemgr.REDFISH:
		linuxUtils := linuxutils.NewLinuxUtils(e, logger)
		ip := linuxUtils.GetBmcIP()
		if ip == "" {
			return nil, nil, status.Error(codes.Internal, "IDRAC IP is not found")
		}
		return idracmgr.NewIDRACManager(logger, 10*time.Second, "root", "passwd", ip), nil, nil
	case drivemgr.TEST:
		driveManager := loopbackmgr.NewLoopBackManager(e, logger)
		// initialize
		err := driveManager.Init()
		if err != nil {
			logger.Errorf("Failed to initialize Drive manager: %s", err.Error())
			return nil, nil, err
		}
		cleanup := driveManager.CleanupLoopDevices
		return driveManager, cleanup, nil
	default:
		// use HAL manager by default
		return halmgr.NewHALManager(logger), nil, nil
	}
}
