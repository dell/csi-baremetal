package drivemgr

import (
	"context"

	"github.com/sirupsen/logrus"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

// DriveServiceServerImpl is the implementation of gRPC server that gives possibility to invoke DriveManager's methods
// remotely
type DriveServiceServerImpl struct {
	mgr DriveManager
	log *logrus.Entry
}

// NewDriveServer is the constructor for DriveServiceServerImpl struct
// Receives logrus logger and implementation of DriveManager as parameters
// Returns an instance of DriveServiceServerImpl
func NewDriveServer(logger *logrus.Logger, manager DriveManager) DriveServiceServerImpl {
	driveService := DriveServiceServerImpl{
		log: logger.WithField("component", "DriveServiceServerImpl"),
		mgr: manager,
	}
	return driveService
}

// GetDrivesList invokes DriveManager's GetDrivesList() and sends the response over gRPC
// Receives go context and DrivesRequest which contains node id
// Returns DrivesResponse with slice of api.Drives structs
func (svc *DriveServiceServerImpl) GetDrivesList(ctx context.Context, req *api.DrivesRequest) (*api.DrivesResponse, error) {
	drives, err := svc.mgr.GetDrivesList()
	if err != nil {
		svc.log.Errorf("DriveManager failed with error: %s", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	// All drives are ONLINE by default
	for _, drive := range drives {
		drive.NodeId = req.NodeId
		if drive.Status == "" {
			drive.Status = apiV1.DriveStatusOnline
		}
	}
	return &api.DrivesResponse{
		Disks: drives,
	}, nil
}
