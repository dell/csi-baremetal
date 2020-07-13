// Package dmsetup has method for drivemgr initialization and startup
package dmsetup

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/drivemgr"
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
