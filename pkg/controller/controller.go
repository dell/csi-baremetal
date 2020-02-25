package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

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
type CtxKey string

// keep status in volumecrd.ObjecMeta.Annotation
//type VolumeStatus string

const (
	NodeSvcPodsMask           = "baremetal-csi-node"
	NodeIDTopologyKey         = "baremetal-csi/nodeid"
	VolumeStatusAnnotationKey = "dell.emc.csi/volume-status"

	CotrollerRequestUUID            CtxKey = "ControllerRequestUUID"
	DefaultVolumeID                        = "Undefined ID"
	CreateLocalVolumeRequestTimeout        = 300 * time.Second
)

// interface implementation for ControllerServer
type CSIControllerService struct {
	namespace string
	k8sclient.Client
	communicators map[NodeID]api.VolumeManagerClient
	volumeCache   *VolumesCache
	//mutex for crd request
	crdMu sync.Mutex
	log   *logrus.Entry
	// TODO: do not use cache for AC, just read ACs from CRDs AK8S-173
	availableCapacityCache *AvailableCapacityCache
	//mutex for csi request
	reqMu sync.Mutex
}

func NewControllerService(k8sClient k8sclient.Client, logger *logrus.Logger, namespace string) *CSIControllerService {
	c := &CSIControllerService{
		namespace:              namespace,
		Client:                 k8sClient,
		communicators:          make(map[NodeID]api.VolumeManagerClient),
		volumeCache:            &VolumesCache{items: make(map[VolumeID]*volumecrd.Volume)},
		availableCapacityCache: &AvailableCapacityCache{items: make(map[string]map[string]*accrd.AvailableCapacity)},
	}
	c.volumeCache.SetLogger(logger)
	c.availableCapacityCache.SetLogger(logger)
	c.log = logger.WithField("component", "CSIControllerService")
	return c
}

func (c *CSIControllerService) InitController() error {
	ll := c.log.WithField("method", "InitController")

	ll.Info("Initialize communicators ...")
	if err := c.updateCommunicators(); err != nil {
		return fmt.Errorf("unable to initialize communicators for node services: %v", err)
	}

	ll.Info("Initialize available capacity with timeout in 120 seconds ...")
	ctx, cancelFn := context.WithTimeout(context.Background(), 240*time.Second)
	if err := c.updateAvailableCapacityCache(ctx); err != nil {
		ll.Info("Run Cancel context because of error")
		cancelFn()
		return fmt.Errorf("unable to initialize available capacity: %v", err)
	}

	cancelFn()
	return nil
}

func (c *CSIControllerService) updateAvailableCapacityCache(ctx context.Context) error {
	ll := c.log.WithFields(logrus.Fields{
		"method": "updateAvailableCapacityCache",
	})
	wasError := false
	for nodeID, mgr := range c.communicators {
		response, err := mgr.GetAvailableCapacity(ctx, &api.AvailableCapacityRequest{NodeId: string(nodeID)})
		if err != nil {
			ll.Errorf("Error during GetAvailableCapacity request to node %s: %v", nodeID, err)
			wasError = true
		}
		availableCapacity := response.GetAvailableCapacity()
		ll.Info("Current available capacity is: ", availableCapacity)
		for _, ac := range availableCapacity {
			//name of available capacity crd is node id + drive location
			name := ac.NodeId + "-" + strings.ToLower(ac.Location)
			if c.availableCapacityCache.Get(ac.NodeId, ac.Location) == nil {
				newAC := c.constructAvailableCapacityCRD(name, ac)
				if err := c.CreateCRD(context.WithValue(ctx, CotrollerRequestUUID, name), newAC, name); err != nil {
					ll.Errorf("Error during CreateAvailableCapacity request to k8s: %v, error: %v", ac, err)
					wasError = true
				}
				if err = c.availableCapacityCache.Create(newAC, ac.NodeId, ac.Location); err != nil {
					ll.Errorf("Error during available crd addition to cache: %v, error: %v", ac, err)
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

// TODO: update communicators and available capacity in background AK8S-174
func (c *CSIControllerService) updateCommunicators() error {
	ll := c.log.WithField("method", "updateCommunicators")
	pods, err := c.getPods(context.Background(), NodeSvcPodsMask)
	if err != nil {
		return err
	}

	ll.Infof("Found %d pods with node service", len(pods))

	for _, pod := range pods {
		endpoint := fmt.Sprintf("tcp://%s:%d", pod.Status.PodIP, base.DefaultVolumeManagerPort)
		client, err := base.NewClient(nil, endpoint, c.log.Logger)
		if err != nil {
			c.log.Errorf("Unable to initialize gRPC client for communicating with pod %s, error: %v",
				pod.Name, err)
			continue
		}
		c.communicators[NodeID(pod.Spec.NodeName)] = api.NewVolumeManagerClient(client.GRPCClient)
		ll.Infof("Add communicator for node %s on endpoint %s", pod.Spec.NodeName, endpoint)
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

	ctxWithID := context.WithValue(ctx, CotrollerRequestUUID, req.GetName())

	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	c.reqMu.Lock()
	// if volumes is in cache that mean that volume CRD was created
	// check volumes status and wait Created status
	// TODO: do not use local cache, use volume CRD instead AK8S-171
	if v := c.volumeCache.getVolumeByID(req.Name); v != nil {
		ll.Info("Volume was found in items, inspect status ...")
		c.reqMu.Unlock()
		return c.pullCreateStatus(ctxWithID, req)
	}

	var (
		ac            *accrd.AvailableCapacity
		requiredBytes = req.GetCapacityRange().GetRequiredBytes()
		preferredNode = ""
		err           error
	)
	if req.GetAccessibilityRequirements() != nil {
		preferredNode = req.GetAccessibilityRequirements().Preferred[0].Segments["baremetal-csi/nodeid"]
		ll.Infof("Preferred node was provided: %s", preferredNode)
	}

	if ac = c.searchAvailableCapacity(preferredNode, requiredBytes); ac == nil {
		c.reqMu.Unlock()
		ll.Info("There is no suitable drive for volume")
		return nil, status.Errorf(codes.ResourceExhausted, "there is no suitable drive for volume %s", req.Name)
	}
	ll.Infof("Disk with S/N %s on node %s was selected.", ac.Spec.Location, ac.Spec.NodeId)

	vol := c.constructVolumeCRD(&api.Volume{
		Id:       req.Name,
		Owner:    ac.Spec.NodeId,
		Size:     ac.Spec.Size,
		Location: ac.Spec.Location,
		Status:   api.OperationalStatus_Creating,
	})

	// create volume CRD
	if err = c.CreateCRD(ctxWithID, vol, req.Name); err != nil {
		ll.Errorf("Unable to create CRD, error: %v", err)
		c.reqMu.Unlock()
		return nil, status.Errorf(codes.Internal, "volume was created but CRD wasn't")
	}

	// add volume to cache
	if err = c.volumeCache.addVolumeToCache(vol, req.Name); err != nil {
		ll.Errorf("Unable to place volume in items: %v", err)
		c.reqMu.Unlock()
		return nil, status.Errorf(codes.Internal, "volume was created but seems like same volume was created before")
	}

	// delete Available Capacity CRD
	if err = c.DeleteCRD(ctxWithID, ac); err != nil {
		ll.Errorf("Unable to delete Available Capacity CRD, error: %v", err)
	}
	// delete Available Capacity from cache
	c.availableCapacityCache.Delete(ac.Spec.NodeId, ac.Spec.Location)
	c.reqMu.Unlock()

	go func() {
		ll.Infof("Sending CreateLocalVolume request")
		c.createVolumeOnNode(ac.Spec.NodeId, &api.CreateLocalVolumeRequest{
			PvcUUID:  req.Name,
			Capacity: requiredBytes,
			Sc:       "hdd",
			Location: ac.Spec.Location,
		})
	}()

	return c.pullCreateStatus(ctx, req)
}

// pullCreateStatus check volume status until it become Created on context will closed
func (c *CSIControllerService) pullCreateStatus(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "pullCreateStatus",
		"volumeID": req.Name,
	})

	var (
		v          = c.volumeCache.getVolumeByID(req.Name)
		currStatus api.OperationalStatus
	)
	ll.Infof("Current status is: %s", api.OperationalStatus_name[int32(v.Spec.Status)])

	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done and volume still not become Created, current status %s",
				api.OperationalStatus_name[int32(currStatus)])
			return nil, status.Errorf(codes.Aborted, "Volume is in %s state",
				api.OperationalStatus_name[int32(currStatus)])
		case <-time.After(time.Second):
			c.reqMu.Lock()
			v = c.volumeCache.getVolumeByID(req.Name)
			currStatus = v.Spec.Status
			switch currStatus {
			case api.OperationalStatus_Created:
				{
					ll.Infof("Volume %s has become Created, return that volume", req.Name)
					resp := c.constructCreateVolumeResponse(v.Spec.Owner, v.Spec.Size, req)
					c.reqMu.Unlock()
					return resp, nil
				}
			case api.OperationalStatus_FailedToCreate:
				{
					c.reqMu.Unlock()
					ll.Errorf("Unable to create volume. Will not retry ...")
					return nil, status.Error(codes.Internal, "Unable to create volume on local node.")
				}
			}
			c.reqMu.Unlock()
		}
	}
}

// createVolumeOnNode send blocking gRPC request to appropriate node with context timeout
// and set volume status based on request results
func (c *CSIControllerService) createVolumeOnNode(nodeID string, req *api.CreateLocalVolumeRequest) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "createVolumeOnNode",
		"pvcUUID": req.PvcUUID,
		"nodeID":  nodeID,
	})
	ll.Infof("RPC on node %s with timeout in %f seconds", nodeID, CreateLocalVolumeRequestTimeout.Seconds())

	ctx, cancelFn := context.WithTimeout(context.Background(), CreateLocalVolumeRequestTimeout)
	resp, err := c.communicators[NodeID(nodeID)].CreateLocalVolume(ctx, req)
	// close context
	cancelFn()

	c.reqMu.Lock()
	defer c.reqMu.Unlock()
	if err != nil {
		ll.Errorf("Unable to create volume size of %d bytes on node %s. Error: %v. Set volume status to FailedToCreate",
			req.Capacity, nodeID, err)
		c.volumeCache.setVolumeStatus(req.PvcUUID, api.OperationalStatus_FailedToCreate)
	} else {
		ll.Infof("CreateLocalVolume for node %s returned response: %v. Set status to Created", nodeID, resp)
		c.volumeCache.setVolumeStatus(req.PvcUUID, api.OperationalStatus_Created)
		if err = c.UpdateCRD(context.Background(), c.volumeCache.getVolumeByID(req.PvcUUID)); err != nil {
			ll.Error("Unable to set volume CRD status to Created")
		}
	}
}

// searchAvailableCapacity search appropriate available capacity and remove it from cache
func (c *CSIControllerService) searchAvailableCapacity(preferredNode string, requiredBytes int64) *accrd.AvailableCapacity {
	ll := c.log.WithFields(logrus.Fields{
		"method":        "searchAvailableCapacity",
		"requiredBytes": fmt.Sprintf("%.3fG", float64(requiredBytes)/float64(base.GBYTE)),
	})

	ll.Info("Search appropriate available capacity")

	var (
		allocatedCapacity int64 = math.MaxInt64
		foundAC           *accrd.AvailableCapacity
		maxLen            = 0
	)
	if preferredNode == "" {
		for nodeID, ac := range c.availableCapacityCache.items {
			if len(ac) > maxLen {
				// TODO: what if node doesn't have AC size of requiredBytes
				preferredNode = nodeID
				maxLen = len(ac)
			}
		}
	}

	ll.Infof("Node %s was selected, search drive size of %d on it", preferredNode, requiredBytes)

	for _, capacity := range c.availableCapacityCache.items[preferredNode] {
		if capacity.Spec.Size < allocatedCapacity && capacity.Spec.Size >= requiredBytes {
			foundAC = capacity
			allocatedCapacity = capacity.Spec.Size
		}
	}
	return foundAC
}

// constructCreateVolumeResponse constructs csi.CreateVolumeResponse based on provided arguments
func (c *CSIControllerService) constructCreateVolumeResponse(node string, capacity int64,
	req *csi.CreateVolumeRequest) *csi.CreateVolumeResponse {
	topology := csi.Topology{
		Segments: map[string]string{
			NodeIDTopologyKey: node,
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
	defer c.reqMu.Unlock()

	volume := c.volumeCache.getVolumeByID(req.VolumeId)
	if volume == nil {
		return nil, fmt.Errorf("unable to find volume with ID %s in cache", req.VolumeId)
	}

	node := volume.Spec.Owner //volume.NodeID

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

	ll.Info("DeleteLocalVolume return Ok")
	localVolume := resp.GetVolume()
	if localVolume == nil {
		return nil, status.Error(codes.Internal, "Unable to delete volume from node")
	}

	var (
		volumeCRD = &volumecrd.Volume{}
		ctxWithID = context.WithValue(ctx, CotrollerRequestUUID, req.GetVolumeId())
	)

	// remove volume CRD
	if err = c.ReadCRD(context.Background(), req.VolumeId, volumeCRD); err != nil {
		ll.Errorf("Unable to read volume CRD with name %s, err: %v", req.VolumeId, err)
	} else if err = c.DeleteCRD(ctxWithID, volumeCRD); err != nil {
		ll.Errorf("Delete CRD with name %s failed, error: %v", req.VolumeId, err)
		return nil, status.Errorf(codes.Internal, "can't delete volume crd: %s", err.Error())
	}

	c.volumeCache.deleteVolumeByID(req.VolumeId)

	ac := &api.AvailableCapacity{
		Size:     localVolume.Size,
		Type:     api.StorageClass_ANY,
		Location: localVolume.Location,
		NodeId:   node,
	}

	location := strings.ToLower(localVolume.Location)
	name := node + "-" + location
	if c.availableCapacityCache.Get(node, location) == nil {
		crd := c.constructAvailableCapacityCRD(name, ac)
		if err := c.CreateCRD(ctxWithID, crd, name); err != nil {
			ll.Errorf("Can't create AvailableCapacity CRD %v error: %v", crd, err)
		} else if err = c.availableCapacityCache.Create(crd, node, localVolume.Location); err != nil {
			ll.Errorf("Error during available capacity addition to cache: %v, error: %v", *ac, err)
		}
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (c *CSIControllerService) ControllerPublishVolume(ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	c.log.WithFields(logrus.Fields{
		"method":   "ControllerPublishVolume",
		"volumeID": req.GetVolumeId(),
	}).Info("Return empty response, ok.")

	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (c *CSIControllerService) ControllerUnpublishVolume(ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	c.log.WithFields(logrus.Fields{
		"method":   "ControllerUnpublishVolume",
		"volumeID": req.GetVolumeId(),
	}).Info("Return empty response, ok")

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

func (c *CSIControllerService) constructAvailableCapacityCRD(name string, ac *api.AvailableCapacity) *accrd.AvailableCapacity {
	return &accrd.AvailableCapacity{
		TypeMeta: v12.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: "availablecapacity.dell.com/v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      name,
			Namespace: c.namespace,
		},
		Spec: *ac,
	}
}

func (c *CSIControllerService) constructVolumeCRD(vol *api.Volume) *volumecrd.Volume {
	v := &volumecrd.Volume{
		TypeMeta: v12.TypeMeta{
			Kind:       "Volume",
			APIVersion: "volume.dell.com/v1",
		},
		ObjectMeta: v12.ObjectMeta{
			//Currently volumeId is volume id
			Name:      vol.Id,
			Namespace: c.namespace,
		},
		Spec: *vol,
	}

	v.ObjectMeta.Annotations = map[string]string{VolumeStatusAnnotationKey: api.OperationalStatus_name[int32(vol.Status)]}
	return v
}

func (c *CSIControllerService) CreateCRD(ctx context.Context, obj runtime.Object, name string) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()

	volumeID := ctx.Value(CotrollerRequestUUID)
	if volumeID == nil {
		volumeID = DefaultVolumeID
	}

	ll := c.log.WithFields(logrus.Fields{
		"method":   "CreateCRD",
		"volumeID": volumeID.(string),
	})
	ll.Infof("Creating CRD %s with name %s", obj.GetObjectKind().GroupVersionKind().Kind, name)

	err := c.Get(ctx, k8sclient.ObjectKey{Name: name, Namespace: c.namespace}, obj)
	if err != nil {
		if k8serror.IsNotFound(err) {
			e := c.Create(ctx, obj)
			if e != nil {
				return e
			}
		} else {
			return err
		}
	}
	ll.Infof("CRD with id %s was created successfully", name)
	return nil
}

func (c *CSIControllerService) ReadCRD(ctx context.Context, name string, obj runtime.Object) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()

	volumeID := ctx.Value(CotrollerRequestUUID)
	if volumeID == nil {
		volumeID = DefaultVolumeID
	}

	c.log.WithFields(logrus.Fields{
		"method":   "ReadCRD",
		"volumeID": volumeID.(string),
	}).Infof("Reading CRD %s with name %s", obj.GetObjectKind().GroupVersionKind().Kind, name)

	return c.Get(ctx, k8sclient.ObjectKey{Name: name, Namespace: c.namespace}, obj)
}

func (c *CSIControllerService) ReadListCRD(ctx context.Context, object runtime.Object) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()
	c.log.WithField("method", "ReadListCRD").Info("Reading list")
	return c.List(ctx, object, k8sclient.InNamespace(c.namespace))
}

func (c *CSIControllerService) UpdateCRD(ctx context.Context, obj runtime.Object) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()

	volumeID := ctx.Value(CotrollerRequestUUID)
	if volumeID == nil {
		volumeID = DefaultVolumeID
	}

	c.log.WithFields(logrus.Fields{
		"method":   "UpdateCRD",
		"volumeID": volumeID.(string),
	}).Infof("Updating CRD %s", obj.GetObjectKind().GroupVersionKind().Kind)

	return c.Update(ctx, obj)
}

func (c *CSIControllerService) DeleteCRD(ctx context.Context, obj runtime.Object) error {
	c.crdMu.Lock()
	defer c.crdMu.Unlock()

	volumeID := ctx.Value(CotrollerRequestUUID)
	if volumeID == nil {
		volumeID = DefaultVolumeID
	}

	c.log.WithFields(logrus.Fields{
		"method":   "DeleteCRD",
		"volumeID": volumeID.(string),
	}).Infof("Deleting CRD %s", obj.GetObjectKind().GroupVersionKind().Kind)

	return c.Delete(ctx, obj)
}

func (c *CSIControllerService) getPods(ctx context.Context, mask string) ([]*v13.Pod, error) {
	pods := v13.PodList{}
	err := c.List(ctx, &pods, k8sclient.InNamespace(c.namespace))
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
