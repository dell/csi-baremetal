package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	coreV1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

type NodeID string

const (
	NodeSvcPodsMask           = "baremetal-csi-node"
	NodeIDTopologyKey         = "baremetal-csi/nodeid"
	VolumeStatusAnnotationKey = "dell.emc.csi/volume-status"

	// timeout for gRPC request(CreateLocalVolume) to the node service
	GRPCTimeout = 300 * time.Second
	// timeout in which we expect that any operation should be finished
	DefaultTimeoutForOperations = 10 * time.Minute
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
		expiredAt := volumeCR.ObjectMeta.GetCreationTimestamp().Add(DefaultTimeoutForOperations)
		ll.Infof("Volume exists, current status: %s.", api.OperationalStatus_name[int32(volumeCR.Spec.Status)])
		if expiredAt.Before(time.Now()) {
			ll.Errorf("Timeout of %s for volume creation exceeded.", DefaultTimeoutForOperations)
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

		if ac = c.searchAvailableCapacity(ctxWithID, preferredNode, requiredBytes, sc); ac == nil {
			c.reqMu.Unlock()
			ll.Info("There is no suitable drive for volume")
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
			Owner:        ac.Spec.NodeId,
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

// searchAvailableCapacity search appropriate available capacity and remove it's CR
// if SC is in LVM and there is no AC with such SC then LVG should be created based
// on non-LVM AC's and new AC should be created on point in LVG
func (c *CSIControllerService) searchAvailableCapacity(ctx context.Context, preferredNode string, requiredBytes int64,
	storageClass api.StorageClass) *accrd.AvailableCapacity {
	ll := c.log.WithFields(logrus.Fields{
		"method":        "searchAvailableCapacity",
		"volumeID":      ctx.Value(base.RequestUUID),
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

	ll.Infof("Node %s was selected, search available capacity size of %d bytes and storageClass %s",
		preferredNode, requiredBytes, storageClass.String())

	for _, ac := range acNodeMap[preferredNode] {
		switch storageClass {
		case api.StorageClass_ANY:
			if ac.Spec.Size < allocatedCapacity && ac.Spec.Size >= requiredBytes {
				foundAC = ac
				allocatedCapacity = ac.Spec.Size
			}
		default:
			if ac.Spec.Size < allocatedCapacity && ac.Spec.Size >= requiredBytes &&
				ac.Spec.StorageClass == storageClass {
				foundAC = ac
				allocatedCapacity = ac.Spec.Size
			}
		}
	}

	if storageClass == api.StorageClass_HDDLVG || storageClass == api.StorageClass_SSDLVG {
		if foundAC != nil {
			// check whether LVG being deleted or no
			lvgCR := &lvgcrd.LVG{}
			err := c.k8sclient.ReadCR(context.Background(), foundAC.Spec.Location, lvgCR)
			if err == nil && lvgCR.DeletionTimestamp.IsZero() {
				return foundAC
			}
		}
		// if storageClass is related to LVG and there is no AC with that storageClass
		// search drive with subclass on which LVG is being creating
		subSC := api.StorageClass_HDD
		if storageClass == api.StorageClass_SSDLVG {
			subSC = api.StorageClass_SSD
		}
		ll.Infof("StorageClass is in LVG, search AC with subStorageClass %s", subSC.String())
		foundAC = c.searchAvailableCapacity(ctx, preferredNode, requiredBytes, subSC)
		if foundAC == nil {
			return nil
		}
		ll.Infof("Got AC %v", foundAC)
		return c.recreateACToLVG(storageClass, foundAC)
	}

	return foundAC
}

// recreateACToLVG creates LVG(based on ACs), ensure it become ready,
// creates AC based on that LVG and removes provided list of ACs
// returns created AC or nil
func (c *CSIControllerService) recreateACToLVG(sc api.StorageClass, acs ...*accrd.AvailableCapacity) *accrd.AvailableCapacity {
	ll := c.log.WithField("method", "recreateACToLVG")

	lvgLocations := make([]string, len(acs))
	var lvgSize int64
	for i, ac := range acs {
		lvgLocations[i] = ac.Spec.Location
		lvgSize += ac.Spec.Size
	}

	// create LVG CR based on ACs
	var (
		err    error
		name   = uuid.New().String()
		apiLVG = api.LogicalVolumeGroup{
			Node:      acs[0].Spec.NodeId, // all ACs from the same node
			Name:      name,
			Locations: lvgLocations,
			Size:      lvgSize,
			Status:    api.OperationalStatus_Creating,
		}
	)

	lvg := c.k8sclient.ConstructLVGCR(name, apiLVG)
	if err = c.k8sclient.CreateCR(context.Background(), lvg, name); err != nil {
		ll.Errorf("Unable to create LVG CR: %v", err)
		return nil
	}
	ll.Infof("LVG %v was created. Wait until it become ready.", apiLVG)
	// here we should to wait until VG is reconciled by volumemgr
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeoutForOperations)
	defer cancelFn()
	var newAPILVG *api.LogicalVolumeGroup
	if newAPILVG = c.waitUntilLVGWillBeCreated(ctx, name); newAPILVG == nil {
		if err = c.k8sclient.DeleteCR(context.Background(), lvg); err != nil {
			ll.Errorf("Unable to remove LVG %v: %v", lvg.Spec, err)
		}
		return nil
	}

	// create new AC
	newACCRName := acs[0].Spec.NodeId + "-" + lvg.Name
	newACCR := c.k8sclient.ConstructACCR(newACCRName, api.AvailableCapacity{
		Location:     lvg.Name,
		NodeId:       acs[0].Spec.NodeId,
		StorageClass: sc,
		Size:         newAPILVG.Size,
	})
	if err = c.k8sclient.CreateCR(context.Background(), newACCR, newACCRName); err != nil {
		ll.Errorf("Unable to create AC %v, error: %v", newACCRName, err)
		return nil
	}
	// remove ACs
	for _, ac := range acs {
		if err = c.k8sclient.DeleteCR(context.Background(), ac); err != nil {
			ll.Errorf("Unable to remove AC %v, error: %v. At now that AC.Location has already in LVG AC.", ac, err)
		}
	}
	ll.Infof("AC was created: %v", newACCR)
	return newACCR
}

// waitUntilLVGWillBeCreated check LVG CR status
// return LVG.Spec if LVG.Spec.Status == created, or return nil instead
// check that during context timeout
func (c *CSIControllerService) waitUntilLVGWillBeCreated(ctx context.Context, lvgName string) *api.LogicalVolumeGroup {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "waitUntilLVGWillBeCreated",
		"lvgName": lvgName,
	})
	ll.Infof("Pulling LVG")

	var (
		lvg = &lvgcrd.LVG{}
		err error
	)

	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done and LVG still not become created, consider that it was failed")
			return nil
		case <-time.After(2 * time.Second):
			err = c.k8sclient.ReadCR(ctx, lvgName, lvg)
			switch {
			case err != nil:
				ll.Errorf("Unable to read LVG CR: %v", err)
			case lvg.Spec.Status == api.OperationalStatus_Created:
				ll.Info("LVG was created")
				return &lvg.Spec
			case lvg.Spec.Status == api.OperationalStatus_FailedToCreate:
				ll.Warn("LVG was reached FailedToCreate status")
				return nil
			}
		}
	}
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
		acName = volume.Spec.Owner + "-" + strings.ToLower(volume.Spec.Location)
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
			// AC was found, update it size
			if err := c.updateACSizeOrDelete(&acCR, volume.Spec.Size); err != nil {
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
		NodeId:       volume.Spec.Owner,
	}

	ll.Infof("Creating AC %v, SC - %s", ac, api.StorageClass_name[int32(sc)])
	cr := c.k8sclient.ConstructACCR(acName, ac)
	ll.Infof("ACCRD: %v", cr)
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
