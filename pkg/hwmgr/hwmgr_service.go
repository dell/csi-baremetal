package hwmgr

import (
	"context"

	"github.com/sirupsen/logrus"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/hwmgr/halmgr"
)

type HWServiceServerImpl struct {
	log *logrus.Entry
}

func (svc *HWServiceServerImpl) SetLogger(logger *logrus.Logger) {
	svc.log = logger.WithField("component", "HWServiceServerImpl")
}

func (svc *HWServiceServerImpl) GetDrivesList(context.Context, *api.DrivesRequest) (*api.DrivesResponse, error) {
	// Use HALManager as HWManager because it is the only implementation for now
	var mgr HWManager = &halmgr.HALManager{
		Log: svc.log.WithField("component", "HALManager"),
	}
	// HAL doesn't return DriveType (SSD, HDD, NVMe). So we need to discuss how to set this field
	drives, err := mgr.GetDrivesList()
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
