package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	v13 "k8s.io/api/core/v1"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	v1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeID string

// interface implementation for ControllerServer
type CSIControllerService struct {
	k8sclient.Client
	communicators map[NodeID]api.VolumeManagerClient
	mu            sync.Mutex
	volumeCache   *VolumesCache
	crdMu         sync.Mutex
	log           *logrus.Entry
}

func NewControllerService(k8sClient k8sclient.Client, logger *logrus.Logger) *CSIControllerService {
	c := &CSIControllerService{
		Client:        k8sClient,
		communicators: make(map[NodeID]api.VolumeManagerClient),
		volumeCache:   &VolumesCache{items: make(map[VolumeID]*csiVolume)},
	}
	c.log = logger.WithField("component", "CSIControllerService")
	return c
}

func (c *CSIControllerService) initCommunicators() error {
	pods, err := c.getPods(context.Background(), "baremetal-csi-node")
	if err != nil {
		return err
	}
	for _, pod := range pods {
		endpoint := fmt.Sprintf("tcp://%s:%d", pod.Status.PodIP, base.DefaultVolumeManagerPort)
		client, err := base.NewClient(nil, endpoint, c.log.Logger)
		if err != nil {
			c.log.Errorf("Unable to initialize gRPC client for communicating with pod %s, error: %v",
				pod.Name, err)
		}
		c.communicators[NodeID(pod.Spec.NodeName)] = api.NewVolumeManagerClient(client.GRPCClient)
	}

	if len(c.communicators) == 0 {
		return errors.New("unable to initialize communicators")
	}

	return nil
}

func (c *CSIControllerService) CreateVolume(ctx context.Context,
	req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "CreateVolume",
		"volumeID": req.GetName(),
	})

	ll.Infof("Processing request: %v", req)

	c.mu.Lock()
	defer func() {
		c.mu.Unlock()
		ll.Info("Unlock mutex")
	}()

	if v := c.volumeCache.getVolumeByID(req.Name); v != nil {
		ll.Info("volume was found in items")
		return c.constructCreateVolumeResponse(v.NodeID, v.Size, req), nil
	}

	if len(c.communicators) == 0 {
		ll.Info("Initialize communicators ...")

		if err := c.initCommunicators(); err != nil {
			ll.Fatalf("Unable to initialize communicators for node services: %v", err)
		}
		ll.Info("Communicators initialize successfully")
		for n := range c.communicators {
			ll.Infof("Node - %s", n)
		}
	}

	// Check arguments
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	var preferredNode = ""
	if req.GetAccessibilityRequirements() != nil {
		preferredNode = req.GetAccessibilityRequirements().Preferred[0].Segments["baremetal-csi/nodeid"]
		ll.Infof("Preferred node: %s", preferredNode)
	} else {
		ll.Fatalf("Preferred node must be provided")
	}

	resp, err := c.communicators[NodeID(preferredNode)].CreateLocalVolume(ctx, &api.CreateLocalVolumeRequest{
		PvcUUID:  req.Name,
		Capacity: req.GetCapacityRange().GetRequiredBytes(),
		Sc:       "hdd",
	})
	if err != nil {
		ll.Errorf("Unable to create volume size of %d bytes on node %s. Error: %v",
			req.GetCapacityRange().GetRequiredBytes(), preferredNode, err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("Unable to create volume on node \"%s\"", preferredNode))
	}
	ll.Infof("CreateLocalVolume for node %s returned response: %v", preferredNode, resp)

	err = c.volumeCache.addVolumeToCache(&csiVolume{
		NodeID:   preferredNode,
		VolumeID: req.GetName(),
		Size:     resp.Capacity,
	}, req.GetName())

	if err != nil {
		ll.Errorf("Unable to place volume in items: %v", err)
		return nil, status.Errorf(codes.Internal, "volume was created but can't place them in items")
	}

	_, err = c.CreateVolumeCRD(ctx, api.Volume{
		Id:       req.Name,
		Owner:    preferredNode,
		Size:     resp.Capacity,
		Location: resp.Drive,
	}, "default")
	if err != nil {
		ll.Errorf("Unable to create CRD, error: %v", err)
	}

	return c.constructCreateVolumeResponse(preferredNode, resp.Capacity, req), nil
}

func (c *CSIControllerService) constructCreateVolumeResponse(node string, capacity int64,
	req *csi.CreateVolumeRequest) *csi.CreateVolumeResponse {
	topology := csi.Topology{
		Segments: map[string]string{
			"baremetal-csi/nodeid": node, // TODO: do not hardcode key
		},
	}
	topologyList := []*csi.Topology{&topology}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           req.GetName(),
			CapacityBytes:      capacity,
			VolumeContext:      req.GetParameters(),
			AccessibleTopology: topologyList,
		},
	}
}

func (c *CSIControllerService) DeleteVolume(ctx context.Context,
	req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "DeleteVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing request: %v", req)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	c.mu.Lock()
	defer func() {
		c.mu.Unlock()
		ll.Info("unlock mutex")
	}()

	volume := c.volumeCache.getVolumeByID(req.VolumeId)
	if volume == nil {
		return nil, errors.New("unable to find volume in cache for getting volume ID")
	}

	node := volume.NodeID
	resp, err := c.communicators[NodeID(node)].DeleteLocalVolume(ctx, &api.DeleteLocalVolumeRequest{
		PvcUUID: req.VolumeId,
	})
	if err != nil {
		ll.Errorf("failed to delete local volume with %s", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("unable to delete volume on node \"%s\"", node))
	}

	if !resp.Ok {
		return nil, status.Error(codes.Internal, fmt.Sprintf("response for delete local volume is not ok"))
	}
	c.volumeCache.deleteVolumeByID(req.VolumeId)
	return &csi.DeleteVolumeResponse{}, nil
}

func (c *CSIControllerService) ControllerPublishVolume(ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "ControllerPublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing for node: %s", req.GetNodeId())

	c.mu.Lock()
	defer c.mu.Unlock()

	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (c *CSIControllerService) ControllerUnpublishVolume(ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "ControllerUnpublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing for node: %s", req.GetNodeId())

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnPublishVolume Volume ID must be provided")
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (c *CSIControllerService) ValidateVolumeCapabilities(context.Context, *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (c *CSIControllerService) ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (c *CSIControllerService) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (c *CSIControllerService) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method": "ControllerGetCapabilities",
	})

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

	ll.Infof("ControllerGetCapabilities returns response: %v", resp)

	return resp, nil
}

func (c *CSIControllerService) CreateSnapshot(context.Context, *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (c *CSIControllerService) DeleteSnapshot(context.Context, *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (c *CSIControllerService) ListSnapshots(context.Context, *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (c *CSIControllerService) ControllerExpandVolume(context.Context, *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (c *CSIControllerService) CreateVolumeCRD(ctx context.Context, volume api.Volume, namespace string) (*v1.Volume, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "CreateVolumeCRD",
		"volumeID": volume.GetId(),
	})

	c.crdMu.Lock()
	defer c.crdMu.Unlock()

	ll.Infof("Creating CRD for volume: %v", volume)
	vol := &v1.Volume{
		TypeMeta: v12.TypeMeta{
			Kind:       "Volume",
			APIVersion: "volume.dell.com/v1",
		},
		ObjectMeta: v12.ObjectMeta{
			//Currently name is volume id
			Name:      volume.Id,
			Namespace: namespace,
		},
		Spec:   v1.VolumeSpec{Volume: volume},
		Status: v1.VolumeStatus{},
	}
	instance := &v1.Volume{}
	err := c.Get(ctx, k8sclient.ObjectKey{Name: volume.Id, Namespace: namespace}, instance)
	if err != nil {
		if k8serror.IsNotFound(err) {
			e := c.Create(ctx, vol)
			if e != nil {
				return nil, e
			}
		} else {
			return nil, err
		}
	}
	ll.Info("Creating successfully")
	return vol, nil
}

func (c *CSIControllerService) ReadVolume(ctx context.Context, name string, namespace string) (*v1.Volume, error) {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()
	volume := v1.Volume{}
	err := c.Get(ctx, k8sclient.ObjectKey{Name: name, Namespace: namespace}, &volume)
	if err != nil {
		return nil, err
	}
	return &volume, nil
}

func (c *CSIControllerService) ReadVolumeList(ctx context.Context, namespace string) (*v1.VolumeList, error) {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()
	volumes := v1.VolumeList{}
	err := c.List(ctx, &volumes, k8sclient.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return &volumes, nil
}

func (c *CSIControllerService) getPods(ctx context.Context, mask string) ([]*v13.Pod, error) {
	namespace := "default"
	pods := v13.PodList{}
	err := c.List(ctx, &pods, k8sclient.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	p := make([]*v13.Pod, 0)
	for i := range pods.Items {
		podName := pods.Items[i].ObjectMeta.Name
		if strings.Contains(podName, mask) {
			p = append(p, &pods.Items[i])
		}
	}
	return p, nil
}
