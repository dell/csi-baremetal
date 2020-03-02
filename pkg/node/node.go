package node

import (
	"context"
	"fmt"
	"time"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// depending on SC and parameters in CreateVolumeRequest()
// here we should use different SC implementations for creating required volumes
// the same principle we can use in Controller Server or read from a CRD instance
// store storage class name
type SCName string

type CSINodeService struct {
	NodeID string
	log    *logrus.Entry
	VolumeManager
	grpc_health_v1.HealthServer
}

func NewCSINodeService(client api.HWServiceClient, nodeID string, logger *logrus.Logger) *CSINodeService {
	s := &CSINodeService{
		VolumeManager: *NewVolumeManager(client, &base.Executor{}, logger),
		NodeID:        nodeID,
	}
	s.log = logger.WithField("component", "CSINodeService")
	return s
}

func (s *CSINodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *CSINodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *CSINodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodePublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing request: %v", req)

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path missing in request")
	}

	v, ok := s.getFromVolumeCache(req.VolumeId)
	if !ok {
		return nil, status.Error(codes.NotFound, "There is no volume with appropriate VolumeID")
	}

	// OperationalStatus_Created means that volumes was created (setPartitionUUID) but it is a first NodePublish reuest
	if v.Status == api.OperationalStatus_Publishing ||
		v.Status == api.OperationalStatus_Published ||
		v.Status == api.OperationalStatus_FailedToCreate {
		return s.pullPublishStatus(ctx, v.Id)
	}

	ll.Info("Set status to Publishing")
	s.setVolumeStatus(req.VolumeId, api.OperationalStatus_Publishing)

	scImpl := s.scMap[SCName("hdd")]
	targetPath := req.TargetPath

	bdev, err := s.searchDrivePathBySN(v.Location)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to find device for drive with S/N %s", v.Location)
	}
	partition := fmt.Sprintf("%s1", bdev)

	ll.Info("Create file system and mount in background")
	go func() {
		rollBacked, err := scImpl.PrepareVolume(partition, targetPath)
		if err != nil {
			ll.Infof("PrepareVolume failed: %v, set status to FailedToPublish", err)
			if !rollBacked {
				ll.Error("Try to rollBack again")
			}
			s.setVolumeStatus(req.VolumeId, api.OperationalStatus_FailedToPublish)
		} else {
			ll.Info("PreparedVolume finished successfully, set status to Published")
			s.setVolumeStatus(req.VolumeId, api.OperationalStatus_Published)
		}
	}()

	return s.pullPublishStatus(ctx, v.Id)
}

func (s *CSINodeService) pullPublishStatus(ctx context.Context, volumeID string) (*csi.NodePublishVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "pullPublishStatus",
		"volumeID": volumeID,
	})

	var vol, _ = s.getFromVolumeCache(volumeID)
	ll.Infof("Current status: %s", api.OperationalStatus_name[int32(vol.Status)])

	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done and volume still not become Published, current status %s",
				api.OperationalStatus_name[int32(vol.Status)])
			return nil, status.Error(codes.Internal, "volume is still publishing")
		case <-time.After(time.Second):
			vol, _ = s.getFromVolumeCache(volumeID)
			switch vol.Status {
			case api.OperationalStatus_Publishing:
				{
					time.Sleep(time.Second)
				}
			case api.OperationalStatus_Published:
				{
					ll.Info("Volume was published, return it")
					return &csi.NodePublishVolumeResponse{}, nil
				}
			case api.OperationalStatus_FailedToPublish:
				{
					ll.Errorf("Failed to publish volume %s", volumeID)
					return nil, fmt.Errorf("failed to publish volume %s", volumeID)
				}
			}
		}
	}
}

func (s *CSINodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeUnpublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing request: %v", req)

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path missing in request")
	}

	_, ok := s.getFromVolumeCache(req.VolumeId)
	if !ok {
		return nil, status.Error(codes.Internal, "Unable to find volume")
	}

	err := s.scMap["hdd"].Unmount(req.TargetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, "Unable to unmount")
	}

	s.setVolumeStatus(req.VolumeId, api.OperationalStatus_ReadyToRemove)
	ll.Infof("volume was successfully unmount from %s", req.TargetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *CSINodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil
}

func (s *CSINodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (s *CSINodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func (s *CSINodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method": "NodeGetInfo",
	})

	topology := csi.Topology{
		Segments: map[string]string{
			"baremetal-csi/nodeid": s.NodeID,
		},
	}

	ll.Infof("NodeGetInfo created topology: %v", topology)

	return &csi.NodeGetInfoResponse{
		NodeId:             s.NodeID,
		AccessibleTopology: &topology,
	}, nil
}

// Check does the health check and changes the status of the server based on drives cache size
func (s *CSINodeService) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method": "Check",
	})

	switch len(s.drivesCache) {
	case 0:
		ll.Info("no drives in cache - Node service is not ready yet")
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	default:
		ll.Info("drives in cache - Node service is ready")
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	}
}

// Watch is used by clients to receive updates when the service status changes.
// Watch only dummy implemented just to satisfy the interface.
func (s *CSINodeService) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}
