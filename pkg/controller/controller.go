package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	coreV1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/common"
)

type NodeID string

const (
	NodeSvcPodsMask           = "baremetal-csi-node"
	NodeIDTopologyKey         = "baremetal-csi/nodeid"
	VolumeStatusAnnotationKey = "dell.emc.csi/volume-status"

	// timeout for gRPC request(CreateLocalVolume) to the node service
	GRPCTimeout = 300 * time.Second
)

// interface implementation for ControllerServer
type CSIControllerService struct {
	k8sclient *base.KubeClient

	communicators map[NodeID]api.VolumeManagerClient
	//mutex for csi request
	reqMu sync.Mutex
	log   *logrus.Entry

	acProvider common.AvailableCapacityOperations
}

func NewControllerService(k8sClient *base.KubeClient, logger *logrus.Logger) *CSIControllerService {
	c := &CSIControllerService{
		k8sclient:     k8sClient,
		communicators: make(map[NodeID]api.VolumeManagerClient),
		acProvider:    common.NewACOperationsImpl(k8sClient, logger),
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
	collected := 0
	for nodeID, mgr := range c.communicators {
		response, err := mgr.GetAvailableCapacity(ctx, &api.AvailableCapacityRequest{NodeId: string(nodeID)})
		if err != nil {
			ll.Errorf("Error during GetAvailableCapacity request to node %s: %v", nodeID, err)
			wasError = true
		} else {
			collected++
		}
		availableCapacity := response.GetAvailableCapacity()
		for _, acPtr := range availableCapacity {
			name := acPtr.NodeId + "-" + strings.ToLower(acPtr.Location)
			if err := c.k8sclient.ReadCR(context.WithValue(ctx, base.RequestUUID, name), name, &accrd.AvailableCapacity{}); err != nil {
				if k8sError.IsNotFound(err) {
					newAC := c.k8sclient.ConstructACCR(name, *acPtr)
					if err := c.k8sclient.CreateCR(context.WithValue(ctx, base.RequestUUID, name), newAC, name); err != nil {
						ll.Errorf("Error during CreateAvailableCapacity request to k8s: %v, error: %v", acPtr, err)
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

	if wasError && collected == 0 {
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

func (c *CSIControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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
	// at first check whether volume CR exist or no
	// on mutex because AC based on LVG may be in creating state and controller should wait until AC will be updated
	c.reqMu.Lock()
	err = c.k8sclient.ReadCR(ctx, reqName, volumeCR)
	switch {
	case err == nil:
		// volume is exist, check that it have created state or time is over (for creating)
		expiredAt := volumeCR.ObjectMeta.GetCreationTimestamp().Add(base.DefaultTimeoutForOperations)
		ll.Infof("Volume exists, current status: %s.", volumeCR.Spec.Status.String())
		if volumeCR.Spec.Status == api.OperationalStatus_FailedToCreate {
			return nil, fmt.Errorf("corresponding volume CR %s reached failed status", volumeCR.Spec.Id)
		}
		if expiredAt.Before(time.Now()) {
			ll.Errorf("Timeout of %s for volume creation exceeded.", base.DefaultTimeoutForOperations)
			if err = c.k8sclient.ChangeVolumeStatus(req.GetName(), api.OperationalStatus_FailedToCreate); err != nil {
				ll.Error(err.Error())
			}
			c.reqMu.Unlock()
			return nil, status.Error(codes.Internal, "Unable to create volume in allocated time")
		}
	case !k8sError.IsNotFound(err):
		ll.Errorf("Unable to read volume CR: %v", err)
		c.reqMu.Unlock()
		return nil, status.Error(codes.Aborted, "unable to check volume existence")
	default:
		// create volume
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

		if ac = c.acProvider.SearchAC(ctxWithID, preferredNode, requiredBytes, sc); ac == nil {
			ll.Info("There is no suitable drive for volume")
			c.reqMu.Unlock()
			return nil, status.Errorf(codes.ResourceExhausted, "there is no suitable drive for request %s", req.GetName())
		}
		ll.Infof("AC %v was selected.", ac.Spec)
		// if sc was parsed as an ANY then we can choose AC with any storage class and then volume should be created
		// with that particular SC
		sc = ac.Spec.StorageClass

		switch sc {
		case api.StorageClass_HDDLVG, api.StorageClass_SSDLVG:
			allocatedBytes = requiredBytes
		default:
			allocatedBytes = ac.Spec.Size
		}

		// create volume CR
		apiVolume := api.Volume{
			Id:           reqName,
			NodeId:       ac.Spec.NodeId,
			Size:         allocatedBytes,
			Location:     ac.Spec.Location,
			Status:       api.OperationalStatus_Creating,
			StorageClass: sc,
		}
		volumeCR = c.k8sclient.ConstructVolumeCR(reqName, apiVolume)
		volumeCR.ObjectMeta.Annotations = map[string]string{
			VolumeStatusAnnotationKey: api.OperationalStatus_name[int32(api.OperationalStatus_Creating)],
		}

		if err = c.k8sclient.CreateCR(ctxWithID, volumeCR, reqName); err != nil {
			ll.Errorf("Unable to create CR, error: %v", err)
			c.reqMu.Unlock()
			return nil, status.Errorf(codes.Internal, "unable to create volume CR")
		}

		if err = c.acProvider.UpdateACSizeOrDelete(ac, -requiredBytes); err != nil {
			ll.Errorf("Unable to modify/delete Available Capacity %s, error: %v", ac.Name, err)
		}
	}
	c.reqMu.Unlock()

	ll.Info("Waiting until volume will reach Created status")
	reached, st := c.waitVCRStatus(ctx, req.GetName(),
		api.OperationalStatus_Created, api.OperationalStatus_FailedToCreate)

	if reached {
		if st == api.OperationalStatus_FailedToCreate {
			// this is should be a non-retryable error
			return nil, status.Error(codes.Internal, "Unable to create volume on local node.")
		}

		ll.Infof("Construct response with nodeId: %s, size: %d", volumeCR.Spec.NodeId, volumeCR.Spec.Size)

		topologyList := []*csi.Topology{
			{Segments: map[string]string{NodeIDTopologyKey: volumeCR.Spec.NodeId}},
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

func (c *CSIControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "DeleteVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing request: %v", req)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	var (
		volume         = &volumecrd.Volume{}
		ctxT, cancelFn = context.WithTimeout(context.WithValue(ctx, base.RequestUUID, req.GetVolumeId()),
			GRPCTimeout)
	)

	c.reqMu.Lock()
	defer func() {
		cancelFn() // close context
		c.reqMu.Unlock()
	}()

	if err := c.k8sclient.ReadCR(ctxT, req.VolumeId, volume); err != nil {
		if k8sError.IsNotFound(err) {
			ll.Infof("Volume CR doesn't exist, volume had removed before.")
			return &csi.DeleteVolumeResponse{}, nil
		}
		ll.Errorf("Unable to read volume: %v", err)
		return nil, fmt.Errorf("unable to find volume with ID %s", req.VolumeId)
	}

	if err := c.k8sclient.ChangeVolumeStatus(req.VolumeId, api.OperationalStatus_Removing); err != nil {
		ll.Error(err.Error())
		return nil, status.Errorf(codes.Internal,
			"unable to set status %s for volume %s", api.OperationalStatus_Removing, req.VolumeId)
	}

	ll.Info("Waiting until volume will reach Removed status")
	reached, st := c.waitVCRStatus(ctx, req.VolumeId, api.OperationalStatus_FailToRemove, api.OperationalStatus_Removed)

	if !reached {
		return nil, fmt.Errorf("unable to delete volume with ID %s", req.VolumeId)
	}

	if st == api.OperationalStatus_FailToRemove {
		return nil, status.Errorf(codes.Internal, "volume %s has FailToRemove status", req.VolumeId)
	}

	// remove volume CR
	if err := c.k8sclient.DeleteCR(ctxT, volume); err != nil {
		ll.Errorf("Delete volume CR with name %s failed, error: %v", req.VolumeId, err)
		return nil, status.Errorf(codes.Internal, "can't delete volume CR: %s", err.Error())
	}

	var (
		acName = volume.Spec.NodeId + "-" + strings.ToLower(volume.Spec.Location)
		sc     = volume.Spec.StorageClass
		acList = &accrd.AvailableCapacityList{}
		acCR   = accrd.AvailableCapacity{}
	)

	ll.Infof("Volume CR was deleted, going to create AC with name %s", acName)
	// if SC is LVM - try to find AC with that LVM
	if volume.Spec.StorageClass == api.StorageClass_HDDLVG || volume.Spec.StorageClass == api.StorageClass_SSDLVG {
		if err := c.k8sclient.ReadList(context.Background(), acList); err != nil {
			ll.Errorf("Volume was deleted but corresponding AC hadn't updated/created, unable to read list: %v", err)
			return &csi.DeleteVolumeResponse{}, nil // volume was deleted
		}

		for _, a := range acList.Items {
			if a.Spec.Location == volume.Spec.Location {
				acCR = a
				break
			}
		}
		if acCR.Name != "" {
			// AC was found, update it size (increase)
			if err := c.acProvider.UpdateACSizeOrDelete(&acCR, volume.Spec.Size); err != nil {
				ll.Errorf("Unable to update AC %s size: %v", acCR.Name, err)
			}
			return &csi.DeleteVolumeResponse{}, nil // volume was deleted
		}
	}

	// create AC
	ac := api.AvailableCapacity{
		Size:         volume.Spec.Size,
		StorageClass: sc,
		Location:     volume.Spec.Location,
		NodeId:       volume.Spec.NodeId,
	}

	ll.Infof("Creating AC %v, SC - %s", ac, api.StorageClass_name[int32(sc)])
	cr := c.k8sclient.ConstructACCR(acName, ac)

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

func (c *CSIControllerService) getPods(ctx context.Context, mask string) ([]*coreV1.Pod, error) {
	pods := coreV1.PodList{}

	if err := c.k8sclient.List(ctx, &pods, k8s.InNamespace(c.k8sclient.Namespace)); err != nil {
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
