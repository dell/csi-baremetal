package node

import (
	"context"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
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

const PodNameKey = "csi.storage.k8s.io/pod.name"

func NewCSINodeService(client api.HWServiceClient, nodeID string, logger *logrus.Logger, k8sclient *base.KubeClient) *CSINodeService {
	s := &CSINodeService{
		VolumeManager: *NewVolumeManager(client, &base.Executor{}, logger, k8sclient, nodeID),
		NodeID:        nodeID,
	}
	s.log = logger.WithField("component", "CSINodeService")
	return s
}

func (s *CSINodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeStageVolume",
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
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Stage Path missing in request")
	}

	v, ok := s.getFromVolumeCache(req.VolumeId)
	if !ok {
		return nil, status.Error(codes.NotFound, "There is no volume with appropriate VolumeID")
	}

	scImpl := s.getStorageClassImpl(v.StorageClass)
	ll.Infof("Chosen StorageClass is %s", v.StorageClass.String())

	targetPath := req.StagingTargetPath

	var partition string
	switch v.StorageClass {
	case api.StorageClass_HDDLVG, api.StorageClass_SSDLVG:
		partition = fmt.Sprintf("/dev/%s/%s", v.Location, v.Id)
	default:
		//TODO AK8S-380 Make drives cache thread safe
		s.dCacheMu.Lock()
		drive := s.drivesCache[v.Location]
		s.dCacheMu.Unlock()
		if drive == nil {
			return nil, fmt.Errorf("drive with uuid %s wasn't found ", v.Location)
		}

		// get device path
		bdev, err := s.linuxUtils.SearchDrivePath(drive)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "unable to find device for drive with S/N %s", v.Location)
		}
		partition = fmt.Sprintf("%s1", bdev)
	}

	ll.Infof("Work with partition %s", partition)

	//TODO AK8S-421 Provide operational status for Unstage request
	if v.Status == api.OperationalStatus_ReadyToRemove {
		ll.Info("File system already exists. Perform mount operation")
		if err := scImpl.BindMount(partition, targetPath, true); err != nil {
			ll.Errorf("Failed to stage volume %s, error: %v", v.Id, err)
			return nil, fmt.Errorf("failed to stage volume %s", v.Id)
		}
		return &csi.NodeStageVolumeResponse{}, nil
	}

	ll.Info("Set status to Staging")
	s.setVolumeStatus(req.VolumeId, api.OperationalStatus_Staging)
	ll.Info("Create file system and mount")
	rollBacked, err := scImpl.PrepareVolume(partition, targetPath)
	if err != nil {
		ll.Infof("PrepareVolume failed: %v, set status to FailedToStage", err)
		if !rollBacked {
			ll.Error("Try to rollBack again")
		}
		ll.Errorf("Failed to stage volume %s", v.Id)
		return nil, fmt.Errorf("failed to stage volume %s", v.Id)
	}
	ll.Info("PreparedVolume finished successfully, set status to Staged")
	return &csi.NodeStageVolumeResponse{}, nil
}

func (s *CSINodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeUnstageVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing request: %v", req)

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Stage Path missing in request")
	}

	// This is a temporary solution to clear all owners during NodeUnstage
	// because NodeUnpublishRequest doesn't contain info about pod
	// TODO AK8S-466 Remove owner from Owners slice during Unpublish properly
	// Not fail NodeUnstageRequest if can't clear owners
	if err := s.clearVolumeOwners(req.GetVolumeId()); err != nil {
		ll.Errorf("Failed to clear owners: %v", err)
	}

	if err := s.unmount(req.VolumeId, req.StagingTargetPath); err != nil {
		return nil, err
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (s *CSINodeService) unmount(volumeID string, path string) error {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "unmount",
		"volumeID": volumeID,
	})
	v, ok := s.getFromVolumeCache(volumeID)
	if !ok {
		return status.Error(codes.Internal, "Unable to find volume")
	}

	scImpl := s.getStorageClassImpl(v.StorageClass)
	ll.Infof("Chosen StorageClass is %s", v.StorageClass.String())

	err := scImpl.Unmount(path)
	if err != nil {
		return status.Error(codes.Internal, "Unable to unmount")
	}

	s.setVolumeStatus(volumeID, api.OperationalStatus_ReadyToRemove)
	ll.Infof("volume was successfully unmount from %s", path)
	return nil
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
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Stage path missing in request")
	}
	v, ok := s.getFromVolumeCache(req.VolumeId)
	if !ok {
		return nil, status.Error(codes.NotFound, "There is no volume with appropriate VolumeID")
	}

	// Not fail NodePublishRequest if can't set owner
	podName, ok := req.VolumeContext[PodNameKey]
	if !ok {
		ll.Infof("podInfoOnMound flag is not provided")
	} else if err := s.addVolumeOwner(req.GetVolumeId(), podName); err != nil {
		ll.Errorf("Failed to set owner %s: %v", podName, err)
	}

	srcPath := req.GetStagingTargetPath()
	path := req.GetTargetPath()

	if v.Status == api.OperationalStatus_Published ||
		v.Status == api.OperationalStatus_Publishing {
		return s.pullPublishStatus(ctx, v.Id)
	}
	ll.Info("Set status to Publishing")
	s.setVolumeStatus(v.Id, api.OperationalStatus_Publishing)

	scImpl := s.getStorageClassImpl(v.StorageClass)

	mounted, err := scImpl.IsMountPoint(path)
	if err != nil {
		ll.Infof("IsMountPoint failed: %v, set status to FailedToPublish", err)
		s.setVolumeStatus(req.VolumeId, api.OperationalStatus_FailedToPublish)
		return s.pullPublishStatus(ctx, v.Id)
	}
	go func() {
		newStatus := api.OperationalStatus_Published
		if mounted {
			ll.Infof("Mountpoint already exist, set status to Published")
		} else if err := scImpl.CreateTargetPath(path); err != nil {
			newStatus = api.OperationalStatus_FailedToPublish
		} else if err := scImpl.BindMount(srcPath, path, false); err != nil {
			_ = scImpl.DeleteTargetPath(path)
			newStatus = api.OperationalStatus_FailedToPublish
		}
		ll.Infof("Set status to %s", newStatus)
		s.setVolumeStatus(v.Id, newStatus)
	}()
	return s.pullPublishStatus(ctx, v.Id)
}

func (s *CSINodeService) pullPublishStatus(ctx context.Context, volumeID string) (*csi.NodePublishVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "pullPublishStatus",
		"volumeID": volumeID,
	})

	var vol, _ = s.getFromVolumeCache(volumeID)
	ll.Infof("Current status: %s", api.OperationalStatus_name[int32(s.getVolumeStatus(volumeID))])

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done and volume still not become Published, current status %s",
				api.OperationalStatus_name[int32(vol.Status)])
			return nil, status.Error(codes.Internal, "volume is still publishing")
		case <-ticker.C:
			vol, _ = s.getFromVolumeCache(volumeID)
			switch vol.Status {
			case api.OperationalStatus_Publishing:
				<-ticker.C
			case api.OperationalStatus_Published:
				ll.Info("Volume was published, return it")
				return &csi.NodePublishVolumeResponse{}, nil
			case api.OperationalStatus_FailedToPublish:
				ll.Errorf("Failed to publish volume %s", volumeID)
				return nil, fmt.Errorf("failed to publish volume %s", volumeID)
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

	if err := s.unmount(req.GetVolumeId(), req.GetTargetPath()); err != nil {
		return nil, err
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *CSINodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil
}

func (s *CSINodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (s *CSINodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{Capabilities: []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		}},
	}, nil
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
//TODO AK8S-379 Investigate readiness and liveness probes conditions
func (s *CSINodeService) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method": "Check",
	})

	switch len(s.drivesCache) {
	case 0:
		ll.Info("no drives in cache - Node service is not ready yet")
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	default:
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	}
}

// Watch is used by clients to receive updates when the service status changes.
// Watch only dummy implemented just to satisfy the interface.
func (s *CSINodeService) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}
