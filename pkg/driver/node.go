package driver

import (
	"os/exec"
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
	logrus.Info("NodeServer: NodeGetInfo() call")

	topology := csi.Topology{
		Segments: map[string]string{
			"baremetal-csi/nodeid": d.nodeID,
		},
	}
	logrus.Info("NodeGetInfo created topology: ", topology)
	return &csi.NodeGetInfoResponse{
		NodeId:             d.nodeID,
		AccessibleTopology: &topology,
	}, nil
}

// NodeGetCapabilities is a function for getting node service capabilities
func (d *ECSCSIDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	logrus.WithFields(logrus.Fields{
		"node_capabilities": "empty",
		"method":            "node_get_capabilities",
	}).Info("node get capabilities called")

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{},
	}, nil
}

// NodeStageVolume is a function which call NodeStageVolume request
func (d *ECSCSIDriver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	logrus.WithField("request", req).Info("NodeServer: NodeStageVolume() call")
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume is a function which call NodeUnstageVolume request
func (d *ECSCSIDriver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	logrus.WithField("request", req).Info("NodeServer: NodeUnstageVolume() call")
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume is a function for publishing volume
func (d *ECSCSIDriver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	logrus.WithField("request", req).Info("NodeServer: NodePublishVolume() call")

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

	volumeID := req.VolumeId // hostname_/dev/sda here
	source := strings.Split(volumeID, "_")[1]
	logrus.Info("Block device name - ", source)

	target := req.TargetPath

	err := util.Mount(source, source, target)
	if err != nil {
		logrus.Info("Failed mount ", source, " to ", target)
		return nil, status.Error(codes.Internal, err.Error())
	}

	logrus.Info("Mounted from ", source, "to ", target)

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume is a function for unpublishing volume
func (d *ECSCSIDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	logrus.WithField("request", req).Info("NodeServer: NodeUnPublishVolume() call")
	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path missing in request")
	}
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()

	logrus.Info("Unmount ", targetPath)
	err := util.Unmount(targetPath)
	if err != nil {
		logrus.Error("Unmount ", targetPath, " is failed")
		return nil, status.Error(codes.Internal, err.Error())
	}
	logrus.Infof("Volume %s/%s has been unmounted.", targetPath, volumeID)

	//TODO: take from a database
	// s[0] - node, s[1] - disk
	s := strings.Split(volumeID, "_")

	node, pathToDisk := s[0], s[1]

	//TODO: try to avoid using -f
	cmd := exec.Command("wipefs", "-af", pathToDisk)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Infof("wipefs command is failed with %s, output%s\n", err, out)
		return nil, status.Error(codes.Internal, err.Error())
	}

	logrus.Infof("Disk - %s on node - %s is unpublished", pathToDisk, node)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetVolumeStats is a function
func (d *ECSCSIDriver) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	logrus.Info("NodeServer: NodeGetVolumeStats() call")
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeExpandVolume is a function
func (d *ECSCSIDriver) NodeExpandVolume(context.Context, *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	logrus.Info("NodeServer: NodeExpandVolume() call")
	return nil, status.Error(codes.Unimplemented, "")
}
