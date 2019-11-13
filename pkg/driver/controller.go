package driver

import (
	"context"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	kib int64 = 1024
	mib int64 = kib * 1024
	gib int64 = mib * 1024
	tib int64 = gib * 1024
)

const (
	deviceID           = "deviceID"
	provisionRoot      = "/tmp/"
	snapshotRoot       = "/tmp/"
	maxStorageCapacity = tib
)

// ControllerGetCapabilities is a function for returning plugin capabilities
func (d *ECSCSIDriver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	var caps []*csi.ControllerServiceCapability
	for _, cap := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	} {
		caps = append(caps, newCap(cap))
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	logrus.WithFields(logrus.Fields{
		"response": resp,
		"method":   "controller_get_capabilities",
	}).Info("controller get capabilities called")

	return resp, nil
}

// CreateVolume is a function for creating volumes
func (d *ECSCSIDriver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	logrus.WithField("request", req).Info("ControllerServer: CreateVolume() call")

	// Check arguments
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	var volumeID string
	var nodeID string

	Mutex.Lock()
	if !NodeAllocatedDisksInitialized {
		GetNodeAllocatedDisks()
		logrus.Info("Firstly initialized NodeAllocatedDisks map")
		NodeAllocatedDisksInitialized = true
	}

	var isDiskFound = false
	if req.GetAccessibilityRequirements() == nil {
		//If external-provisioner didn't send AR then use all nodes no find disk
		for node := range NodeAllocatedDisks {
			for disk, allocated := range NodeAllocatedDisks[node] {
				isDiskFound = checkDiskCanBeUsed(&disk, allocated, req.GetCapacityRange().GetRequiredBytes())
				if isDiskFound {
					volumeID = node + "_" + disk.Path
					nodeID = node
					NodeAllocatedDisks[node][disk] = true
					break
				}
			}
			if isDiskFound {
				logrus.Info("Disk found on the node - ", node)
				break
			} else {
				logrus.Info("All disks are allocated on node - ", node)
				logrus.Info(NodeAllocatedDisks[node])
			}
		}
	} else {
		/*If external-provisioner sent AR then use the first node from preferred ones (set by WaitForFirstConsumer
		SC mode) to find a disk. Other nodes cannot be used because the pod that uses volumes has been scheduled to
		the first node from preferred ones.*/
		node := req.GetAccessibilityRequirements().Preferred[0].Segments["baremetal-csi/nodeid"]
		logrus.Info("Preferred node: ", node)
		for disk, allocated := range NodeAllocatedDisks[node] {
			isDiskFound = checkDiskCanBeUsed(&disk, allocated, req.GetCapacityRange().GetRequiredBytes())
			if isDiskFound {
				volumeID = node + "_" + disk.Path
				nodeID = node
				NodeAllocatedDisks[node][disk] = true
				break
			}
		}
		if isDiskFound {
			logrus.Info("Disk found on the node - ", node)
		} else {
			logrus.Info("All disks are allocated on node - ", node)
			logrus.Info(NodeAllocatedDisks[node])
		}
	}
	Mutex.Unlock()

	if isDiskFound {
		//d.nodeIad -> node id
		topology := csi.Topology{
			Segments: map[string]string{
				"baremetal-csi/nodeid": nodeID,
			},
		}
		topologyList := []*csi.Topology{&topology}

		resp := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:           volumeID,
				CapacityBytes:      req.GetCapacityRange().GetRequiredBytes(),
				VolumeContext:      req.GetParameters(),
				AccessibleTopology: topologyList,
			},
		}

		logrus.WithField("response", resp).Info("volume created with ID: ", volumeID)
		return resp, nil
	}

	return nil, status.Error(codes.ResourceExhausted, "cannot allocate locale volume on any node")
}

// DeleteVolume is a function for deleting volume
func (d *ECSCSIDriver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	logrus.WithField("request", req).Info("ControllerServer: DeleteVolume() call")
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume is a function for publishing volume
func (d *ECSCSIDriver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	ll := logrus.WithFields(logrus.Fields{
		"volume_id": req.VolumeId,
		"node_id":   req.NodeId,
	})
	ll.Info("controller publish volume called")

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"baremetal-csi/test-volume": req.VolumeId,
		},
	}, nil
}

// ControllerUnpublishVolume is a function for unpublishing volume
func (d *ECSCSIDriver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	logrus.WithField("request", req).Info("ControllerServer: ControllerUnpublishVolume() call")
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnPublishVolume Volume ID must be provided")
	}
	unpublishDiskPath := strings.Split(req.VolumeId, "_")[1]
	node := req.GetNodeId()
	for disk, _ := range NodeAllocatedDisks[node] {
		if disk.Path == unpublishDiskPath {
			NodeAllocatedDisks[node][disk] = false
			break;
		}
	}
	logrus.Info("Disks state after unpublish on node: ", node)
	logrus.Info(NodeAllocatedDisks[node])
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities is a function
func (d *ECSCSIDriver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	logrus.Info("ControllerServer: ValidateVolumeCapabilities()")
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes is a function
func (d *ECSCSIDriver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	logrus.Info("ControllerServer: ListVolumes()")
	return nil, status.Error(codes.Unimplemented, "")
}

// GetCapacity is a function
func (d *ECSCSIDriver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	logrus.Info("ControllerServer: GetCapacity()")
	return nil, status.Error(codes.Unimplemented, "")
}

// CreateSnapshot is a function
func (d *ECSCSIDriver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	logrus.Info("ControllerServer: CreateSnapshot()")
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot is a function
func (d *ECSCSIDriver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	logrus.Info("ControllerServer: DeleteSnapshot()")
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots is a function
func (d *ECSCSIDriver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	logrus.Info("ControllerServer: ListSnapshots()")
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerExpandVolume is a function
func (d *ECSCSIDriver) ControllerExpandVolume(context.Context, *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
