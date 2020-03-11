package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	coreV1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	apisV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

type NodeID string

const (
	NodeSvcPodsMask           = "baremetal-csi-node"
	NodeIDTopologyKey         = "baremetal-csi/nodeid"
	VolumeStatusAnnotationKey = "dell.emc.csi/volume-status"

	// timeout for gRPC request(CreateLocalVolume) to the node service
	CreateLocalVolumeRequestTimeout = 300 * time.Second
	// timeout in which we expect that volume will be created (Volume CR status became created or failedToCreate)
	CreateVolumeTimeout = 10 * time.Minute
	// if AC size becomes lower then acSizeMinThresholdBytes controller decides that AC should be removed
	acSizeMinThresholdBytes = 1024 * 1024 // 1MB
)

// interface implementation for ControllerServer
type CSIControllerService struct {
	k8sclient *base.KubeClient

	communicators map[NodeID]api.VolumeManagerClient
	//mutex for csi request
	reqMu sync.Mutex
	log   *logrus.Entry

	k8sClient.Client
}

func NewControllerService(k8sClient *base.KubeClient, logger *logrus.Logger) *CSIControllerService {
	c := &CSIControllerService{
		k8sclient:     k8sClient,
		communicators: make(map[NodeID]api.VolumeManagerClient),
	}
	c.log = logger.WithField("component", "CSIControllerService")
	return c
}

func (c *CSIControllerService) InitController() error {
	ll := c.log.WithField("method", "InitController")

	ll.Info("Initialize communicators ...")
	if err := c.updateCommunicators(); err != nil {
		return fmt.Errorf("unable to initialize communicators for node services: %v", err)
	}

	timeout := 240 * time.Second
	ll.Infof("Initialize available capacity with timeout in %s", timeout)
	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	if err := c.updateAvailableCapacityCRs(ctx); err != nil {
		return fmt.Errorf("unable to initialize available capacity: %v", err)
	}

	return nil
}

func (c *CSIControllerService) updateAvailableCapacityCRs(ctx context.Context) error {
	ll := c.log.WithFields(logrus.Fields{
		"method": "updateAvailableCapacityCRs",
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
			//name of available capacity cr is node id + drive location
			name := ac.NodeId + "-" + strings.ToLower(ac.Location)
			if err := c.k8sclient.ReadCR(context.WithValue(ctx, base.RequestUUID, name), name, &accrd.AvailableCapacity{}); err != nil {
				if k8sError.IsNotFound(err) {
					newAC := c.constructAvailableCapacityCR(name, ac)
					if err := c.k8sclient.CreateCR(context.WithValue(ctx, base.RequestUUID, name), newAC, name); err != nil {
						ll.Errorf("Error during CreateAvailableCapacity request to k8s: %v, error: %v", ac, err)
						wasError = true
					}
				} else {
					ll.Errorf("Unable to read Available Capacity %s, error: %v", name, err)
					wasError = true
				}
			} else {
				ll.Infof("Available Capacity %s already exist", name)
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

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	var (
		reqName  = req.GetName()
		volumeCR = &volumecrd.Volume{}
		err      error
	)
	// check whether volume CRD exist or no
	err = c.k8sclient.ReadCR(ctx, reqName, volumeCR)
	switch {
	case err == nil:
		expiredAt := volumeCR.ObjectMeta.GetCreationTimestamp().Add(CreateVolumeTimeout)
		ll.Infof("Volume exists, current status: %s.", api.OperationalStatus_name[int32(volumeCR.Spec.Status)])
		if expiredAt.Before(time.Now()) {
			ll.Errorf("Timeout of %s for volume creation exceeded.", CreateVolumeTimeout)
			if err = c.k8sclient.ChangeVolumeStatus(req.GetName(), api.OperationalStatus_FailedToCreate); err != nil {
				ll.Error(err.Error())
			}
			return nil, status.Error(codes.Internal, "Unable to create volume in allocated time")
		}
	case !k8sError.IsNotFound(err):
		ll.Errorf("unable to read volume CR: %v", err)
		return nil, status.Error(codes.Aborted, "unable to check volume existence")
	default:
		// create volume
		c.reqMu.Lock()
		var (
			ctxWithID      = context.WithValue(ctx, base.RequestUUID, req.GetName())
			sc             = base.ConvertStorageClass(req.Parameters["storageType"])
			requiredBytes  = req.GetCapacityRange().GetRequiredBytes()
			preferredNode  = ""
			ac             *accrd.AvailableCapacity
			allocatedBytes int64
		)
		if req.GetAccessibilityRequirements() != nil {
			preferredNode = req.GetAccessibilityRequirements().Preferred[0].Segments[NodeIDTopologyKey]
			ll.Infof("Preferred node was provided: %s", preferredNode)
		}

		if ac = c.searchAvailableCapacity(preferredNode, requiredBytes, sc); ac == nil {
			c.reqMu.Unlock()
			ll.Info("There is no suitable drive for volume")
			return nil, status.Errorf(codes.ResourceExhausted, "there is no suitable drive for request %s", req.GetName())
		}
		ll.Infof("Disk with S/N %s on node %s was selected.", ac.Spec.Location, ac.Spec.NodeId)

		switch sc {
		case api.StorageClass_HDDLVG, api.StorageClass_SSDLVG:
			allocatedBytes = requiredBytes
		default:
			allocatedBytes = ac.Spec.Size
		}

		// create volume CR
		volumeCR = &volumecrd.Volume{
			TypeMeta: apisV1.TypeMeta{
				Kind:       "Volume",
				APIVersion: "volume.dell.com/v1",
			},
			ObjectMeta: apisV1.ObjectMeta{
				Name:      reqName,
				Namespace: c.k8sclient.Namespace,
				Annotations: map[string]string{
					VolumeStatusAnnotationKey: api.OperationalStatus_name[int32(api.OperationalStatus_Creating)],
				},
			},
			Spec: api.Volume{
				Id:           reqName,
				Owner:        ac.Spec.NodeId,
				Size:         allocatedBytes,
				Location:     ac.Spec.Location,
				Status:       api.OperationalStatus_Creating,
				StorageClass: ac.Spec.Type,
			},
		}

		if err = c.k8sclient.CreateCR(ctxWithID, volumeCR, reqName); err != nil {
			ll.Errorf("Unable to create CR, error: %v", err)
			c.reqMu.Unlock()
			return nil, status.Errorf(codes.Internal, "unable to create volume CR")
		}

		// delete or modify Available Capacity CR based on storage class
		switch sc {
		case api.StorageClass_HDDLVG, api.StorageClass_SSDLVG:
			err = c.updateACSizeOrDelete(ac, -requiredBytes) // shrink size or delete AC
		default:
			err = c.k8sclient.DeleteCR(ctxWithID, ac)
		}
		if err != nil {
			ll.Errorf("Unable to modify/delete Available Capacity %s, error: %v", ac.Name, err)
		}

		c.reqMu.Unlock()
	}

	ll.Info("Waiting until volume will reach Created status")
	reached, st := c.waitVCRStatus(ctx, req.GetName(),
		api.OperationalStatus_Created, api.OperationalStatus_FailedToCreate)

	if reached {
		if st == api.OperationalStatus_FailedToCreate {
			// this is should be a non-retryable error
			return nil, status.Error(codes.Internal, "Unable to create volume on local node.")
		}

		ll.Infof("Construct response with owner: %s, size: %d", volumeCR.Spec.Owner, volumeCR.Spec.Size)

		topologyList := []*csi.Topology{
			{Segments: map[string]string{NodeIDTopologyKey: volumeCR.Spec.Owner}},
		}

		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:           req.GetName(),
				CapacityBytes:      volumeCR.Spec.Size,
				VolumeContext:      req.GetParameters(),
				AccessibleTopology: topologyList,
			},
		}, nil
	}

	return nil, status.Errorf(codes.Aborted, "CreateVolume is in progress")
}

// waitVCRStatus check volume status until it will be reached one of the statuses
// return true if one of the status had reached, or return false instead
// also return status that had reached or -1
func (c *CSIControllerService) waitVCRStatus(ctx context.Context,
	volumeID string,
	statuses ...api.OperationalStatus) (bool, api.OperationalStatus) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "waitVCRStatus",
		"volumeID": volumeID,
	})
	ll.Infof("Pulling volume status")

	var (
		v   = &volumecrd.Volume{}
		err error
	)

	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done but volume still not become in expected state")
			return false, -1
		case <-time.After(time.Second):
			if err = c.k8sclient.ReadCR(context.WithValue(ctx, base.RequestUUID, volumeID), volumeID, v); err != nil {
				ll.Errorf("Unable to read volume CR and check status: %v", err)
				continue
			}
			for _, s := range statuses {
				if v.Spec.Status == s {
					ll.Infof("Volume has reached %s state.", api.OperationalStatus_name[int32(s)])
					return true, s
				}
			}
		}
	}
}

// searchAvailableCapacity search appropriate available capacity and remove it from cache
func (c *CSIControllerService) searchAvailableCapacity(preferredNode string, requiredBytes int64,
	storageClass api.StorageClass) *accrd.AvailableCapacity {
	ll := c.log.WithFields(logrus.Fields{
		"method":        "searchAvailableCapacity",
		"requiredBytes": fmt.Sprintf("%.3fG", float64(requiredBytes)/float64(base.GBYTE)),
	})

	ll.Info("Search appropriate available ac")

	var (
		allocatedCapacity int64 = math.MaxInt64
		foundAC           *accrd.AvailableCapacity
		acList            = &accrd.AvailableCapacityList{}
		acNodeMap         map[string][]*accrd.AvailableCapacity
		maxLen            = 0
	)

	err := c.k8sclient.ReadList(context.Background(), acList)
	if err != nil {
		ll.Errorf("Unable to read Available Capacity list, error: %v", err)
		return nil // it does mean a non-retryable error
	}
	acNodeMap = c.acNodeMapping(acList.Items)

	// search node with max amount of available capacity instances
	if preferredNode == "" {
		for nodeID, acs := range acNodeMap {
			if len(acs) > maxLen {
				// TODO: what if node doesn't have AC size of requiredBytes
				preferredNode = nodeID
				maxLen = len(acs)
			}
		}
	}

	ll.Infof("Node %s was selected, search available capacity size of %d on it with storageClass %s",
		preferredNode, requiredBytes, storageClass.String())

	for _, ac := range acNodeMap[preferredNode] {
		if storageClass == api.StorageClass_ANY {
			if ac.Spec.Size < allocatedCapacity && ac.Spec.Size >= requiredBytes {
				foundAC = ac
				allocatedCapacity = ac.Spec.Size
			}
		} else {
			if ac.Spec.Size < allocatedCapacity && ac.Spec.Size >= requiredBytes &&
				ac.Spec.Type == storageClass {
				foundAC = ac
				allocatedCapacity = ac.Spec.Size
			}
		}
	}
	return foundAC
}

// updateACSizeOrDelete update size of AC or delete that AC if new size is low that some threshold
// bytes - difference in size, could be negative number (decrease size)
func (c *CSIControllerService) updateACSizeOrDelete(ac *accrd.AvailableCapacity, bytes int64) error {
	ctx, fn := context.WithTimeout(context.Background(), 10*time.Second)
	defer fn()

	newSize := ac.Spec.Size + bytes
	if newSize > acSizeMinThresholdBytes {
		c.log.WithField("method", "updateACSizeOrDelete").
			Infof("Updating size of AC %s to %d bytes", ac.Name, newSize)
		ac.Spec.Size = newSize
		return c.k8sclient.UpdateCR(ctx, ac)
	}
	return c.k8sclient.DeleteCR(ctx, ac)
}

// acNodeMapping constructs map with key - nodeID(hostname), value - AC instance
func (c *CSIControllerService) acNodeMapping(acs []accrd.AvailableCapacity) map[string][]*accrd.AvailableCapacity {
	var (
		acNodeMap = make(map[string][]*accrd.AvailableCapacity)
		node      string
	)

	for _, ac := range acs {
		node = ac.Spec.NodeId
		if _, ok := acNodeMap[node]; !ok {
			acNodeMap[node] = make([]*accrd.AvailableCapacity, 0)
		}
		acTmp := ac
		acNodeMap[node] = append(acNodeMap[node], &acTmp)
	}
	return acNodeMap
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

	var (
		volume         = &volumecrd.Volume{TypeMeta: apisV1.TypeMeta{Kind: "Volume"}} // set Kind for better logging
		ctxT, cancelFn = context.WithTimeout(context.WithValue(ctx, base.RequestUUID, req.GetVolumeId()),
			CreateLocalVolumeRequestTimeout)
	)

	c.reqMu.Lock()
	defer func() {
		c.reqMu.Unlock()
		cancelFn() // close context
	}()

	if err := c.k8sclient.ReadCR(ctxT, req.VolumeId, volume); err != nil {
		if k8sError.IsNotFound(err) {
			ll.Infof("Volume CR doesn't exist, volume had removed before.")
			return &csi.DeleteVolumeResponse{}, nil
		}
		ll.Errorf("Unable to read volume: %v", err)
		return nil, fmt.Errorf("unable to find volume with ID %s", req.VolumeId)
	}

	node := volume.Spec.Owner //volume.NodeID

	ll.Infof("RPC on node %s", node)
	resp, err := c.communicators[NodeID(node)].DeleteLocalVolume(ctxT, &api.DeleteLocalVolumeRequest{
		PvcUUID: req.VolumeId,
	})

	localVolume := resp.GetVolume()
	if err != nil || !resp.Ok || localVolume == nil {
		ll.Errorf("failed to delete local volume, %v", err)
		return nil, status.Errorf(codes.Internal, "unable to delete volume on node %s", node)
	}

	ll.Info("DeleteLocalVolume return Ok")
	// remove volume CR
	if err = c.k8sclient.DeleteCR(ctxT, volume); err != nil {
		ll.Errorf("Delete volume CR with name %s failed, error: %v", req.VolumeId, err)
		return nil, status.Errorf(codes.Internal, "can't delete volume CR: %s", err.Error())
	}

	var (
		acName = node + "-" + strings.ToLower(localVolume.Location)
		sc     = api.StorageClass_ANY
	)

	// if SC is LVM - try to find AC with that LVM
	if volume.Spec.StorageClass == api.StorageClass_HDDLVG || volume.Spec.StorageClass == api.StorageClass_SSDLVG {
		ac := &accrd.AvailableCapacity{}
		if err := c.k8sclient.ReadCR(context.Background(), acName, ac); err != nil {
			if !k8sError.IsNotFound(err) {
				ll.Errorf("Unable to check whether AC %s exist or no, error: %v", acName, err)
				return nil, status.Error(codes.Internal, "unable to restore capacity")
			}
			// AC wasn't found and going to be created with next sc
			sc = volume.Spec.StorageClass
		} else {
			// AC was found, update it size
			if err = c.updateACSizeOrDelete(ac, localVolume.Size); err != nil {
				ll.Errorf("Unable to update AC %s size: %v", ac.Name, err)
			}
			return &csi.DeleteVolumeResponse{}, nil // volume was deleted
		}
	}

	// create AC
	ac := &api.AvailableCapacity{
		Size:     localVolume.Size,
		Type:     sc,
		Location: localVolume.Location,
		NodeId:   node,
	}

	cr := c.constructAvailableCapacityCR(acName, ac)
	if err := c.k8sclient.CreateCR(ctxT, cr, acName); err != nil {
		ll.Errorf("Can't create AvailableCapacity CR %v error: %v", cr, err)
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
	for _, c := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	} {
		caps = append(caps, newCap(c))
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

func (c *CSIControllerService) constructAvailableCapacityCR(name string, ac *api.AvailableCapacity) *accrd.AvailableCapacity {
	return &accrd.AvailableCapacity{
		TypeMeta: apisV1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: "availablecapacity.dell.com/v1",
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:      name,
			Namespace: c.k8sclient.Namespace,
		},
		Spec: *ac,
	}
}

func (c *CSIControllerService) getPods(ctx context.Context, mask string) ([]*coreV1.Pod, error) {
	pods := coreV1.PodList{}

	if err := c.k8sclient.List(ctx, &pods, k8sClient.InNamespace(c.k8sclient.Namespace)); err != nil {
		return nil, err
	}
	p := make([]*coreV1.Pod, 0)
	for i := range pods.Items {
		podName := pods.Items[i].ObjectMeta.Name
		if strings.Contains(podName, mask) {
			p = append(p, &pods.Items[i])
		}
	}
	return p, nil
}
