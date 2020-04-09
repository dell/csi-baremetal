package node

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
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

	if err := s.prepareAndPerformMount(partition, targetPath, scImpl); err != nil {
		return nil, fmt.Errorf("failed to stage volume")
	}
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

// prepareAndPerformMount is used it in Stage/Publish requests to prepareAndPerformMount scrPath to targetPath, opts are used for prepareAndPerformMount commands
func (s *CSINodeService) prepareAndPerformMount(srcPath, targetPath string, scImpl sc.StorageClassImplementer, opts ...string) error {
	ll := s.log.WithFields(logrus.Fields{
		"method": "prepareAndPerformMount",
	})
	if err := scImpl.CreateTargetPath(targetPath); err != nil {
		return err
	}
	mounted, err := scImpl.IsMountPoint(targetPath)
	if err != nil {
		ll.Info("IsMountPoint failed: ", err)
		_ = scImpl.DeleteTargetPath(targetPath)
		return err
	}
	if mounted {
		ll.Infof("Mount point already exist")
		return nil
	}
	if err := scImpl.Mount(srcPath, targetPath, opts...); err != nil {
		_ = scImpl.DeleteTargetPath(targetPath)
		return err
	}
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

	ll.Info("Set status to Publishing")
	s.setVolumeStatus(v.Id, api.OperationalStatus_Publishing)

	scImpl := s.getStorageClassImpl(v.StorageClass)

	if err := s.prepareAndPerformMount(srcPath, path, scImpl, "--bind"); err != nil {
		ll.Errorf("prepareAndPerformMount failed, set status to FailedToPublish")
		s.setVolumeStatus(v.Id, api.OperationalStatus_FailedToPublish)
		return nil, fmt.Errorf("failed to publish volume")
	}
	ll.Infof("Set status to %s", api.OperationalStatus_Published)
	s.setVolumeStatus(v.Id, api.OperationalStatus_Published)
	return &csi.NodePublishVolumeResponse{}, nil
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

	if !s.initialized {
		ll.Info("no drives in cache - Node service is not ready yet")
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

// Watch is used by clients to receive updates when the service status changes.
// Watch only dummy implemented just to satisfy the interface.
func (s *CSINodeService) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}
