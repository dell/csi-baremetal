// Package dmsetup has method for drivemgr initialization and startup
package dmsetup

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/drivemgr"
)

// SetupAndRunDriveMgr setups and start/stop particular drive manager
func SetupAndRunDriveMgr(d drivemgr.DriveManager, sr *rpc.ServerRunner, cleanupFn func(), logger *logrus.Logger) {
	logger.Info("Start DriveManager")

	driveServiceServer := drivemgr.NewDriveServer(logger, d)

	api.RegisterDriveServiceServer(sr.GRPCServer, &driveServiceServer)

	go util.SetupSignalHandler(sr)

	if err := sr.RunServer(); err != nil && err != grpc.ErrServerStopped {
		logger.Fatalf("Failed to serve on %s. Error: %v", sr.Endpoint, err)
	}

	logger.Info("Got SIGTERM signal")
	// clean loop devices after drivemgr deletion
	// using defer is the bad practice because defer isn't invoking during SIGTERM or SIGINT
	// kubernetes sends SIGTERM signal to containers for pods terminating
	if cleanupFn != nil {
		cleanupFn()
	}
}
