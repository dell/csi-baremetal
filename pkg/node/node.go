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
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
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

	volumeID := req.VolumeId
	v, ok := s.getFromVolumeCache(volumeID)
	if !ok {
		message := fmt.Sprintf("No volume with ID %s found on node", volumeID)
		ll.Error(message)
		return nil, status.Error(codes.NotFound, message)
	}

	if v.CSIStatus == apiV1.Failed {
		return nil, fmt.Errorf("corresponding volume CR %s reached failed status", v.Id)
	}

	scImpl := s.getStorageClassImpl(v.StorageClass)
	ll.Infof("Chosen StorageClass is %s", v.StorageClass.String())

	targetPath := req.StagingTargetPath

	var partition string
	switch v.StorageClass {
	case api.StorageClass_HDDLVG, api.StorageClass_SSDLVG:
		vgName := v.Location
		var err error

		// for LVG based on system disk LVG CR name != VG name
		// need to read appropriate LVG CR and use LVG CR.Spec.Name as VG name
		if v.StorageClass == api.StorageClass_SSDLVG {
			vgName, err = s.k8sclient.GetVGNameByLVGCRName(ctx, v.Location)
			if err != nil {
				return nil, fmt.Errorf("unable to find LVG name by LVG CR name: %v", err)
			}
		}

		partition = fmt.Sprintf("/dev/%s/%s", vgName, v.Id)
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
		// todo get rid of code duplicates
		volumeUUID, _ := util.GetVolumeUUID(v.Id)
		// get partition name
		partition, err = s.linuxUtils.GetPartitionNameByUUID(bdev, volumeUUID)
		if err != nil {
			return nil, err
		}
	}

	if v.CSIStatus == apiV1.VolumeReady {
		ll.Info("Perform mount operation")
		if err := scImpl.Mount(partition, targetPath); err != nil {
			ll.Errorf("Failed to stage volume %s, error: %v", v.Id, err)
			return nil, fmt.Errorf("failed to stage volume %s", v.Id)
		}
		return &csi.NodeStageVolumeResponse{}, nil
	}
	ll.Infof("Work with partition %s", partition)

	if err := s.prepareAndPerformMount(partition, targetPath, scImpl); err != nil {
		s.setVolumeStatus(req.VolumeId, apiV1.Failed)
		return nil, fmt.Errorf("failed to stage volume")
	}
	s.setVolumeStatus(req.VolumeId, apiV1.VolumeReady)
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
	v, ok := s.getFromVolumeCache(req.GetVolumeId())
	if !ok {
		return nil, status.Error(codes.Internal, "Unable to find volume")
	}

	if v.CSIStatus == apiV1.Failed {
		//Should we return error or nil?
		return nil, status.Errorf(codes.Internal, "corresponding CR %s reached Failed status", v.Id)
	}

	// This is a temporary solution to clear all owners during NodeUnstage
	// because NodeUnpublishRequest doesn't contain info about pod
	// TODO AK8S-466 Remove owner from Owners slice during Unpublish properly
	// Not fail NodeUnstageRequest if can't clear owners
	if err := s.clearVolumeOwners(req.GetVolumeId()); err != nil {
		ll.Errorf("Failed to clear owners: %v", err)
	}

	if err := s.unmount(v.StorageClass, req.GetStagingTargetPath()); err != nil {
		s.setVolumeStatus(v.Id, apiV1.Failed)
		return nil, err
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

//We use unmount in Unstage/Unpublish requests to avoid duplicated code
func (s *CSINodeService) unmount(storageClass api.StorageClass, path string) error {
	ll := s.log.WithFields(logrus.Fields{
		"method": "unmount",
	})
	scImpl := s.getStorageClassImpl(storageClass)

	err := scImpl.Unmount(path)
	if err != nil {
		return status.Error(codes.Internal, "Unable to unmount")
	}

	ll.Infof("volume was successfully unmount from %s", path)
	return nil
}

// prepareAndPerformMount is used it in Stage/Publish requests to prepare and perform mount scrPath to targetPath,
// opts are used as an opts in mount command
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

	if v.CSIStatus == apiV1.Failed {
		return nil, fmt.Errorf("corresponding volume CR %s reached failed status", v.Id)
	}
	scImpl := s.getStorageClassImpl(v.StorageClass)

	if err := s.prepareAndPerformMount(srcPath, path, scImpl, "--bind"); err != nil {
		ll.Errorf("prepareAndPerformMount failed, set status to FailedToPublish")
		s.setVolumeStatus(v.Id, apiV1.Failed)
		return nil, fmt.Errorf("failed to publish volume")
	}
	ll.Infof("Set status to %s", apiV1.Published)
	s.setVolumeStatus(v.Id, apiV1.Published)
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

	v, ok := s.getFromVolumeCache(req.GetVolumeId())
	if !ok {
		return nil, status.Error(codes.NotFound, "Unable to find volume")
	}
	if err := s.unmount(v.StorageClass, req.GetTargetPath()); err != nil {
		s.setVolumeStatus(v.Id, apiV1.Failed)
		return nil, err
	}
	//If volume has more than 1 owner pods then keep its status as Published
	if len(v.Owners) > 1 {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}
	s.setVolumeStatus(v.Id, apiV1.VolumeReady)
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
		ll.Info("Node service is not ready yet")
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

// Watch is used by clients to receive updates when the service status changes.
// Watch only dummy implemented just to satisfy the interface.
func (s *CSINodeService) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}
