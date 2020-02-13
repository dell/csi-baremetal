package node

import (
	"context"
	"fmt"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// depending on SC and parameters in CreateVolumeRequest()
// here we should use different SC implementations for creating required volumes
// the same principle we can use in Controller Server or read from a CRD instance
// store storage class name
type SCName string

type CSINodeService struct {
	VolumeManager
	NodeID string
	log    *logrus.Entry
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

	s.vCacheMu.Lock()
	defer s.vCacheMu.Unlock()

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

	v := s.volumesCache[req.VolumeId]
	if v == nil {
		return nil, status.Error(codes.NotFound, "There is no volume with appropriate VolumeID")
	}

	scImpl := s.scMap[SCName("hdd")]
	targetPath := req.TargetPath

	bdev, err := s.searchDrivePathBySN(v.Location)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to find device for drive with S/N %s", v.Location)
	}
	partition := fmt.Sprintf("%s1", bdev)

	rollBacked, err := scImpl.PrepareVolume(partition, targetPath)
	if err != nil {
		if !rollBacked {
			v.Status = api.OperationalStatus_Inoperative
			// TODO: figure out device status
			return nil, status.Error(codes.Internal, fmt.Sprintf("unable to publish volume to %s on %s", targetPath, bdev))
		}
		return nil, err
	}
	v.Status = api.OperationalStatus_Operative
	ll.Infof("Successfully mount %s to path %s", partition, targetPath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *CSINodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeUnpublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing request: %v", req)

	s.vCacheMu.Lock()
	defer s.vCacheMu.Unlock()

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path missing in request")
	}

	v := s.volumesCache[req.VolumeId]
	err := s.scMap["hdd"].Unmount(req.TargetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, "Unable to unmount")
	}

	v.Status = api.OperationalStatus_ReadyToRemove
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
