package driver

import (
	"context"
	"math"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	caps := make([]*csi.ControllerServiceCapability, 0)
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
		"response":  resp,
		"component": "controllerService",
		"method":    "ControllerGetCapabilities",
	}).Info("controller get capabilities called")

	return resp, nil
}

// CreateVolume is a function for creating volumes
func (d *ECSCSIDriver) CreateVolume(ctx context.Context,
	req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "controllerService",
		"method":    "CreateVolume",
		"requestID": req.GetName(),
	})
	ll.Infof("Got request: %v", req)

	Mutex.Lock()
	ll.Infof("Lock mutex for %s", req.Name)
	defer func() {
		Mutex.Unlock()
		ll.Infof("Unlock mutex for %s", req.Name)
	}()

	// Check arguments
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	var preferredNode = ""
	// If external-provisioner didn't send AR then use all nodes to find disk
	if req.GetAccessibilityRequirements() != nil {
		/*
			If storage class have WaitForFirstConsumer binding mode then scheduler CHOOSE node for pod.
			Then this node appear on first place on Preferred list from AccessibilityRequirements in CreateVolume request
			If external-provisioner sent AR then use the first node from preferred ones (set by WaitForFirstConsumer
			SC mode) to find a disk. Other nodes cannot be used because the pod that uses volumes has been scheduled to
			the first node from preferred ones.
		*/
		preferredNode = req.GetAccessibilityRequirements().Preferred[0].Segments["baremetal-csi/nodeid"]
		ll.Infof("Preferred node: %s", preferredNode)
	}

	requestedCapacityInBytes := req.GetCapacityRange().GetRequiredBytes()
	var nodeID, volumeID string
	var finalCapacity int64 // if volume is created from block device all device capacity will be used
	var err error

	volume := csiVolumesCache.getVolumeByName(req.GetName())
	// Check if volume with req.Name exists in cache
	if volume != nil {
		// If volume exists then fill CreateVolumeResponse with its fields
		ll.Infof("Found volume in cache, will use nodeID %s, volulmeID %s, capacity %d", nodeID, volumeID, finalCapacity)
		finalCapacity = volume.Size
		nodeID = volume.NodeID
		volumeID = volume.VolumeID
	} else {
		if d.LVMMode {
			finalCapacity = requestedCapacityInBytes
			volumeID, nodeID, err = d.createVolumeFromLVM(requestedCapacityInBytes, preferredNode)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			if !NodeAllocatedDisksInitialized {
				GetNodeAllocatedDisks()
				ll.Info("Firstly initialized NodeAllocatedDisks map")
				NodeAllocatedDisksInitialized = true
			}
			volumeID, finalCapacity, nodeID, err = d.createVolumeFromBlockDevice(requestedCapacityInBytes, preferredNode)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		ll.Infof("Got volumeID %s, nodeID %s, finalCapacity %d", volumeID, nodeID, finalCapacity)
		err = csiVolumesCache.addVolumeToCache(&csiVolume{
			Name:     req.GetName(),
			VolumeID: volumeID,
			NodeID:   nodeID,
			Size:     finalCapacity,
		})
		if err != nil {
			return nil, err
		}
	}

	topology := csi.Topology{
		Segments: map[string]string{
			"baremetal-csi/nodeid": nodeID,
		},
	}
	topologyList := []*csi.Topology{&topology}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           volumeID,
			CapacityBytes:      finalCapacity,
			VolumeContext:      req.GetParameters(),
			AccessibleTopology: topologyList,
		},
	}

	ll.WithField("response", resp).Info("volume created with ID: ", volumeID)
	return resp, nil
}

func (d *ECSCSIDriver) createVolumeFromBlockDevice(capInBytes int64, preferredNode string) (volumeID string, capacity int64, nodeID string, err error) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "controllerService",
		"method":    "createVolumeFromBlockDevice",
	})
	ll.Info("Processing")

	capacity, nodeID, volumeID = AllocateDisk(NodeAllocatedDisks, preferredNode, capInBytes)

	if capacity <= 0 {
		return "", 0, "", status.Error(codes.ResourceExhausted, "cannot allocate locale volume on any node")
	}

	return volumeID, capacity, nodeID, nil
}

func (d *ECSCSIDriver) createVolumeFromLVM(capInBytes int64, preferredNode string) (volumeID string, nodeID string, err error) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "controllerService",
		"method":    "createVolumeFromLVM",
	})
	var capacityInGb = math.RoundToEven(float64(capInBytes) / 1024 / 1024 / 1024)
	ll.Infof("Requested capacity: %f", capacityInGb)

	nodeID, volumeID, err = d.SS.PrepareVolume(capacityInGb, preferredNode)
	if err != nil {
		ll.Errorf("Could not create volume size of %f. Error: %v", capacityInGb, err)

		return "", "", err
	}

	return volumeID, nodeID, nil
}

// DeleteVolume is a function for deleting volume
func (d *ECSCSIDriver) DeleteVolume(ctx context.Context,
	req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "controllerService",
		"method":    "DeleteVolume",
		"VolumeID":  req.VolumeId,
	})
	ll.Infof("Request: %v", req)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	if d.LVMMode {
		err := d.SS.ReleaseVolume(strings.Split(req.VolumeId, "_")[0], req.VolumeId) // TODO: handle index out of range error or implement struct for ID
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not remove volume with ID %s. Error: %v", req.VolumeId, err)
		}
		ll.Infof("Volume %s was successfully removed", req.VolumeId)
	} else {
		err := ReleaseDisk(req.VolumeId, NodeAllocatedDisks)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "DeleteVolume has invalid Volume ID format")
		}
	}

	// Delete volume from cache by its ID
	csiVolumesCache.deleteVolumeByID(req.GetVolumeId())

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume is a function for publishing volume
func (d *ECSCSIDriver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "controllerService",
		"method":    "ControllerPublishVolume",
	})
	ll.Infof("Request: %v", req)

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"baremetal-csi/test-volume": req.VolumeId,
		},
	}, nil
}

// ControllerUnpublishVolume is a function for unpublishing volume
func (d *ECSCSIDriver) ControllerUnpublishVolume(ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	ll := logrus.WithFields(logrus.Fields{
		"component": "controllerService",
		"method":    "ControllerUnpublishVolume",
	})
	ll.Infof("Request: %v", req)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnPublishVolume Volume ID must be provided")
	}

	if d.LVMMode {
		ll.Infof("Skip for LVM")

		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	unpublishDiskPath := strings.Split(req.VolumeId, "_")[1] // TODO: handle index out of range error or implement struct for ID

	node := req.GetNodeId()

	for disk := range NodeAllocatedDisks[node] {
		if disk.Path == unpublishDiskPath {
			NodeAllocatedDisks[node][disk] = false
			break
		}
	}

	logrus.Infof("Disks state after unpublish on node %s: %v", node, NodeAllocatedDisks[node])

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
