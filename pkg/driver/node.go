package driver

import (
	"fmt"
	"strings"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NodeGetInfo is a function for getting node info
func (d *ECSCSIDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	topology := csi.Topology{
		Segments: map[string]string{
			"baremetal-csi/nodeid": d.nodeID,
		},
	}

	logger.WithFields(logrus.Fields{
		"component": "nodeService",
		"method":    "NodeGetInfo",
	}).Infof("NodeGetInfo created topology: %v", topology)

	return &csi.NodeGetInfoResponse{
		NodeId:             d.nodeID,
		AccessibleTopology: &topology,
	}, nil
}

// NodeGetCapabilities is a function for getting node service capabilities
func (d *ECSCSIDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	logger.WithFields(logrus.Fields{
		"component":         "nodeService",
		"method":            "NodeGetCapabilities",
		"node_capabilities": "empty",
	}).Infof("Called")

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{},
	}, nil
}

// NodeStageVolume is a function which call NodeStageVolume request
func (d *ECSCSIDriver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	logger.WithField("request", req).Info("NodeServer: NodeStageVolume() call")

	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume is a function which call NodeUnstageVolume request
func (d *ECSCSIDriver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	logger.WithField("request", req).Info("NodeServer: NodeUnstageVolume() call")

	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume is a function for publishing volume
func (d *ECSCSIDriver) NodePublishVolume(ctx context.Context,
	req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	ll := logger.WithFields(logrus.Fields{
		"component": "nodeService",
		"method":    "NodePublishVolume",
		"volumeID":  req.VolumeId,
	})
	ll.Infof("Request: %v", req)

	Mutex.Lock()
	ll.Info("Lock mutex")
	defer func() {
		Mutex.Unlock()
		ll.Info("Unlock mutex")
	}()

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

	volumeID := req.VolumeId // hostname_/dev/sda or hostname_VG-NAME_LV-NAME
	var source string
	target := req.TargetPath
	if d.LVMMode {
		vgName := strings.Split(volumeID, "_")[1] // TODO: handle index out of range error or implement struct for ID
		lvName := strings.Split(volumeID, "_")[2]
		source = fmt.Sprintf("/dev/%s/%s", vgName, lvName)
	} else {
		source = strings.Split(volumeID, "_")[1]
		// For idempotency support. If device is already mounted to the target path then return NodePublishVolumeResponse
		if util.IsMountedBockDevice(source, target) {
			logger.Infof("Device %s is already mounted to path %s", source, target)
			return &csi.NodePublishVolumeResponse{}, nil
		}
	}
	ll.Infof("Block device name - %s", source)

	err := util.Mount(source, target)

	if err != nil {
		ll.Infof("Failed mount %s to %s", source, target)
		return nil, status.Error(codes.Internal, err.Error())
	}

	ll.Infof("Mounted from %s to %s", source, target)

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume is a function for unpublishing volume
func (d *ECSCSIDriver) NodeUnpublishVolume(ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	ll := logger.WithFields(logrus.Fields{
		"component": "nodeService",
		"method":    "NodeUnpublishVolume",
		"volumeID":  req.VolumeId,
	})
	ll.Infof("Request: %v", req)

	Mutex.Lock()
	ll.Info("Lock mutex")
	defer func() {
		Mutex.Unlock()
		ll.Info("Unlock mutex")
	}()

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path missing in request")
	}

	targetPath := req.GetTargetPath()

	volumeID := req.GetVolumeId()
	source := strings.Split(volumeID, "_")[1]

	ll.Info("Unmount ", targetPath)

	if d.LVMMode { // TODO: IsMountedBockDevice does not work for Logical Volumes
		err := util.Unmount(targetPath)
		if err != nil {
			ll.Error("Unmount ", targetPath, " is failed")
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		// Attempt to unmount only if volume is mounted to target path
		if util.IsMountedBockDevice(source, targetPath) {
			err := util.Unmount(targetPath)
			if err != nil {
				ll.Error("Unmount ", targetPath, " is failed")
				return nil, status.Error(codes.Internal, err.Error())
			}

			s := strings.Split(volumeID, "_")
			_, pathToDisk := s[0], s[1]

			// TODO: move it into Controller DeleteVolume request
			err = util.WipeFS(pathToDisk)
			if err != nil {
				logger.Infof("wipefs command is failed with %v", err)
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			ll.Infof("Device %s is already unmounted from path %s", source, targetPath)
		}
	}

	ll.Infof("volume %s was successfully unmount from %s, unpublish finished", volumeID, targetPath)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetVolumeStats is a function
func (d *ECSCSIDriver) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	logger.Info("NodeServer: NodeGetVolumeStats() call")

	return nil, status.Error(codes.Unimplemented, "")
}

// NodeExpandVolume is a function
func (d *ECSCSIDriver) NodeExpandVolume(context.Context, *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	logger.Info("NodeServer: NodeExpandVolume() call")

	return nil, status.Error(codes.Unimplemented, "")
}
