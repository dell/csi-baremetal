package hwmgr

import (
	"context"

	"github.com/sirupsen/logrus"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
)

// HWServiceServerImpl is the implementation of gRPC server that gives possibility to invoke HWManager's methods
// remotely
type HWServiceServerImpl struct {
	mgr HWManager
	log *logrus.Entry
}

// NewHWServer is the constructor for HWServiceServerImpl struct
// Receives logrus logger and implementation of HWManager as parameters
// Returns an instance of HWServiceServerImpl
func NewHWServer(logger *logrus.Logger, manager HWManager) HWServiceServerImpl {
	hwService := HWServiceServerImpl{
		log: logger.WithField("component", "HWServiceServerImpl"),
		mgr: manager,
	}
	return hwService
}

// GetDrivesList invokes HWManager's GetDrivesList() and sends the response over gRPC
// Receives go context and DrivesRequest which contains node id
// Returns DrivesResponse with slice of api.Drives structs
func (svc *HWServiceServerImpl) GetDrivesList(ctx context.Context, req *api.DrivesRequest) (*api.DrivesResponse, error) {
	drives, err := svc.mgr.GetDrivesList()
	if err != nil {
		svc.log.Errorf("HWManager failed with error: %s", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	// All drives are ONLINE by default
	for _, drive := range drives {
		drive.NodeId = req.NodeId
		drive.Status = apiV1.DriveStatusOnline
	}
	return &api.DrivesResponse{
		Disks: drives,
	}, nil
}
