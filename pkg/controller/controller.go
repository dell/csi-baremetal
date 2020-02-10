package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"

	"github.com/sirupsen/logrus"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v13 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeID string

const NodeSvcPodsMask = "baremetal-csi-node"

// interface implementation for ControllerServer
type CSIControllerService struct {
	k8sclient.Client
	communicators map[NodeID]api.VolumeManagerClient
	volumeCache   *VolumesCache
	//mutex for crd request
	crdMu                  sync.Mutex
	log                    *logrus.Entry
	availableCapacityCache *AvailableCapacityCache
	//mutex for csi request
	reqMu sync.Mutex
}

func NewControllerService(k8sClient k8sclient.Client, logger *logrus.Logger) *CSIControllerService {
	c := &CSIControllerService{
		Client:                 k8sClient,
		communicators:          make(map[NodeID]api.VolumeManagerClient),
		volumeCache:            &VolumesCache{items: make(map[VolumeID]*csiVolume)},
		availableCapacityCache: &AvailableCapacityCache{items: make(map[string]*accrd.AvailableCapacity)},
	}
	c.volumeCache.SetLogger(logger)
	c.availableCapacityCache.SetLogger(logger)
	c.log = logger.WithField("component", "CSIControllerService")
	return c
}

func (c *CSIControllerService) updateAvailableCapacityCache(ctx context.Context) error {
	ll := c.log.WithFields(logrus.Fields{
		"component": "controller",
		"method":    "updateAvailableCapacityCache",
	})
	wasError := false
	for nodeID, mgr := range c.communicators {
		response, err := mgr.GetAvailableCapacity(ctx, &api.AvailableCapacityRequest{NodeId: string(nodeID)})
		if err != nil {
			ll.Errorf("Error during GetAvailableCapacity request to node %s: %v", nodeID, err)
			wasError = true
		}
		availableCapacity := response.GetAvailableCapacity()
		logrus.Info("Available capacity: ", availableCapacity)
		for _, ac := range availableCapacity {
			//name of available capacity crd is node id + drive location
			name := ac.NodeId + "-" + strings.ToLower(ac.Location)
			if c.availableCapacityCache.Get(name) == nil {
				crd, err := c.CreateAvailableCapacity(ctx, *ac, "default", name)
				if err != nil {
					ll.Errorf("Error during CreateAvailableCapacity request to k8s: %v, error: %v", ac, err)
					wasError = true
				}
				err = c.availableCapacityCache.Create(crd, name)
				if err != nil {
					ll.Errorf("Error during available accrd addition to cache: %v, error: %v", ac, err)
					wasError = true
				}
			}
		}
	}
	if wasError {
		return errors.New("not all available capacity were created")
	}
	return nil
}

// TODO: do we need re-init communicators some times
func (c *CSIControllerService) updateCommunicators() error {
	pods, err := c.getPods(context.Background(), NodeSvcPodsMask)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		endpoint := fmt.Sprintf("tcp://%s:%d", pod.Status.PodIP, base.DefaultVolumeManagerPort)
		client, err := base.NewClient(nil, endpoint, c.log.Logger)
		if err != nil {
			c.log.Errorf("Unable to initialize gRPC client for communicating with pod %s, error: %v",
				pod.Name, err)
			continue
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
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	ll := c.log.WithFields(logrus.Fields{
		"method":   "CreateVolume",
		"volumeID": req.GetName(),
	})
	ll.Infof("Processing request: %v", req)

	// Check arguments
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	if v := c.volumeCache.getVolumeByID(req.Name); v != nil {
		ll.Info("volume was found in items, return it")
		return c.constructCreateVolumeResponse(v.NodeID, v.Size, req), nil
	}

	if len(c.communicators) == 0 {
		ll.Info("Initialize communicators ...")

		if err := c.updateCommunicators(); err != nil {
			ll.Errorf("Unable to initialize communicators for node services: %v", err)
			return nil, status.Error(codes.Internal, "Controller service was not initialized")
		}
		ll.Info("Communicators initialize successfully")
		for n := range c.communicators {
			ll.Infof("Node - %s", n)
		}
	}

	if len(c.availableCapacityCache.items) == 0 {
		ll.Info("Initialize available capacity ...")
		if err := c.updateAvailableCapacityCache(ctx); err != nil {
			ll.Errorf("Unable to initialize available capacity: %v", err)
		}
	}

	var preferredNode string
	if req.GetAccessibilityRequirements() != nil {
		preferredNode = req.GetAccessibilityRequirements().Preferred[0].Segments["baremetal-csi/nodeid"]
		ll.Infof("Preferred node: %s", preferredNode)
	} else {
		ll.Errorf("Preferred node must be provided. Check that driver's volumeBindingMode is WaitForFirstConsumer")
		return nil, status.Error(codes.InvalidArgument, "Preferred node must be provided.")
	}

	resp, err := c.communicators[NodeID(preferredNode)].CreateLocalVolume(ctx, &api.CreateLocalVolumeRequest{
		PvcUUID:  req.Name,
		Capacity: req.GetCapacityRange().GetRequiredBytes(),
		Sc:       "hdd",
	})
	if err != nil {
		ll.Errorf("Unable to create volume size of %d bytes on node %s. Error: %v",
			req.GetCapacityRange().GetRequiredBytes(), preferredNode, err)
		return nil, status.Errorf(codes.Internal, "Unable to create volume on node %s", preferredNode)
	}
	ll.Infof("CreateLocalVolume for node %s returned response: %v", preferredNode, resp)

	err = c.volumeCache.addVolumeToCache(&csiVolume{
		NodeID:   preferredNode,
		VolumeID: req.GetName(),
		Size:     resp.Capacity,
	}, req.GetName())

	if err != nil {
		ll.Errorf("Unable to place volume in items: %v", err)
		return nil, status.Errorf(codes.Internal, "volume was created but seems like same volume was created before")
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
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	c.reqMu.Lock()
	defer func() {
		c.reqMu.Unlock()
		ll.Info("unlock mutex")
	}()

	volume := c.volumeCache.getVolumeByID(req.VolumeId)
	if volume == nil {
		return nil, fmt.Errorf("unable to find volume with ID %s in cache", req.VolumeId)
	}

	node := volume.NodeID
	resp, err := c.communicators[NodeID(node)].DeleteLocalVolume(ctx, &api.DeleteLocalVolumeRequest{
		PvcUUID: req.VolumeId,
	})
	if err != nil {
		ll.Errorf("failed to delete local volume with %s", err)
		return nil, status.Errorf(codes.Internal, "unable to delete volume on node %s", node)
	}

	if !resp.Ok {
		return nil, status.Error(codes.Internal, "response for delete local volume is not ok")
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

	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (c *CSIControllerService) ControllerUnpublishVolume(ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "ControllerUnpublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing for node: %s", req.GetNodeId())

	// TODO: do we need to validate parameters from sidecars?
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

func (c *CSIControllerService) CreateVolumeCRD(ctx context.Context, volume api.Volume, namespace string) (*volumecrd.Volume, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "CreateVolumeCRD",
		"volumeID": volume.GetId(),
	})

	vol := &volumecrd.Volume{
		TypeMeta: v12.TypeMeta{
			Kind:       "Volume",
			APIVersion: "volume.dell.com/v1",
		},
		ObjectMeta: v12.ObjectMeta{
			//Currently volumeId is volume id
			Name:      volume.Id,
			Namespace: namespace,
		},
		Spec: volume,
	}
	instance := &volumecrd.Volume{}
	err := c.Get(ctx, k8sclient.ObjectKey{Name: volume.Id, Namespace: namespace}, instance)
	if err != nil {
		if k8serror.IsNotFound(err) {
			c.crdMu.Lock()
			defer c.crdMu.Unlock()
			e := c.Create(ctx, vol)
			if e != nil {
				return nil, e
			}
		} else {
			return nil, err
		}
	}
	ll.Infof("VolumeCRD with id %s was created successfully", volume.Id)
	return vol, nil
}

func (c *CSIControllerService) CreateAvailableCapacity(ctx context.Context,
	ac api.AvailableCapacity, namespace string, name string) (*accrd.AvailableCapacity, error) {
	ll := c.log.WithFields(logrus.Fields{
		"component": "CSIControllerService",
		"method":    "CreateAvailableCapacity"})
	ll.Infof("Creating CRD for availableCapacity: %v", ac)
	newAC := &accrd.AvailableCapacity{
		TypeMeta: v12.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: "availablecapacity.dell.com/v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ac,
	}
	instance := &accrd.AvailableCapacity{}
	err := c.Get(ctx, k8sclient.ObjectKey{Name: name, Namespace: namespace}, instance)
	if err != nil {
		if k8serror.IsNotFound(err) {
			c.crdMu.Lock()
			defer c.crdMu.Unlock()
			e := c.Create(ctx, newAC)
			if e != nil {
				return nil, e
			}
		} else {
			return nil, err
		}
	}
	ll.Infof("AvailableCapacity created successfully on node %s with drive %s: ", ac.NodeId, ac.Location)
	return newAC, nil
}

func (c *CSIControllerService) ReadCRD(ctx context.Context, name string, namespace string, object runtime.Object) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()
	return c.Get(ctx, k8sclient.ObjectKey{Name: name, Namespace: namespace}, object)
}

func (c *CSIControllerService) ReadListCRD(ctx context.Context, namespace string, object runtime.Object) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()
	return c.List(ctx, object, k8sclient.InNamespace(namespace))
}

func (c *CSIControllerService) UpdateAvailableCapacity(ctx context.Context, capacity accrd.AvailableCapacity) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()
	return c.Update(ctx, &capacity)
}

func (c *CSIControllerService) DeleteAvailableCapacity(ctx context.Context, capacity accrd.AvailableCapacity) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()
	return c.Delete(ctx, &capacity)
}

func (c *CSIControllerService) getPods(ctx context.Context, mask string) ([]*v13.Pod, error) {
	namespace := "default"
	pods := v13.PodList{}
	err := c.List(ctx, &pods, k8sclient.InNamespace(namespace))
	// TODO: how does simulate error here?
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
