package hwmgr

import (
	"context"

	"github.com/sirupsen/logrus"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

type HWServiceServerImpl struct {
	mgr HWManager
	log *logrus.Entry
}

func NewHWServer(logger *logrus.Logger, manager HWManager) HWServiceServerImpl {
	hwService := HWServiceServerImpl{
		log: logger.WithField("component", "HWServiceServerImpl"),
		mgr: manager,
	}
	return hwService
}

func (svc *HWServiceServerImpl) GetDrivesList(context.Context, *api.DrivesRequest) (*api.DrivesResponse, error) {
	drives, err := svc.mgr.GetDrivesList()
	if err != nil {
		svc.log.Errorf("HWManager failed with error: %s", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	// All drives are ONLINE by default
	for _, drive := range drives {
		drive.Status = api.Status_ONLINE
	}
	return &api.DrivesResponse{
		Disks: drives,
	}, nil
}
