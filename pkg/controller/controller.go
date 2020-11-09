/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controller contains implementation of CSI Controller component
package controller

import (
	"context"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/common"
	"github.com/dell/csi-baremetal/pkg/controller/node"
	csibmnodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/csibmnode/common"
)

// NodeID is the type for node hostname
type NodeID string

// CSIControllerService is the implementation of ControllerServer interface from GO CSI specification
type CSIControllerService struct {
	k8sclient *k8s.KubeClient

	// mutex for csi request
	reqMu sync.Mutex
	log   *logrus.Entry

	svc common.VolumeOperations

	// to track node health status
	nodeServicesStateMonitor *node.ServicesStateMonitor

	ready bool

	csi.IdentityServer
	grpc_health_v1.HealthServer
}

// NewControllerService is the constructor for CSIControllerService struct
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of CSIControllerService
func NewControllerService(k8sClient *k8s.KubeClient, logger *logrus.Logger,
	featureConf featureconfig.FeatureChecker) *CSIControllerService {
	c := &CSIControllerService{
		k8sclient:                k8sClient,
		log:                      logger.WithField("component", "CSIControllerService"),
		svc:                      common.NewVolumeOperationsImpl(k8sClient, logger, featureConf),
		nodeServicesStateMonitor: node.NewNodeServicesStateMonitor(k8sClient, logger),
		IdentityServer:           NewIdentityServer(base.PluginName, base.PluginVersion),
	}

	// run health monitor
	c.nodeServicesStateMonitor.Run()

	return c
}

// Probe is the implementation of CSI Spec Probe for IdentityServer.
// This method checks if CSI driver is ready to serve requests
// overrides same method from defaultIdentityServer struct
func (c *CSIControllerService) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: len(c.nodeServicesStateMonitor.GetReadyPods()) > 0,
		},
	}, nil
}

// WaitNodeServices waits for the first ready Node. Node readiness means that all Node containers are in Ready state
// and corresponding port is open
// Returns true in case of ready node service and false instead
func (c *CSIControllerService) WaitNodeServices() bool {
	// get information from nodeServicesStateMonitor
	if pods := c.nodeServicesStateMonitor.GetReadyPods(); pods != nil {
		return true
	}

	return false
}

// Check does the health check and changes the status of the server based on drives cache size
func (c *CSIControllerService) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method": "Check",
	})
	// If controller service is ready we don't need to update cache often
	if !c.ready {
		c.nodeServicesStateMonitor.UpdateNodeHealthCache()
	}
	if len(c.nodeServicesStateMonitor.GetReadyPods()) > 0 {
		c.ready = true
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	}
	c.ready = false
	ll.Info("Controller svc is not ready yet")
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
}

// Watch is used by clients to receive updates when the svc status changes.
// Watch only dummy implemented just to satisfy the interface.
func (c *CSIControllerService) Watch(*grpc_health_v1.HealthCheckRequest, grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

// CreateVolume is the implementation of CSI Spec CreateVolume. If k8s SC of driver is set to WaitForFirstConsumer then
// preferred node chosen by k8s Scheduler would be used for Volume otherwise node would be chosen by balanceAC method.
// k8s StorageClass contains parameters field. This field can contain storage type where the Volume will be based.
// For example storageType: HDD, storageType: HDDLVG. If this field is not set then storage type would be ANY.
// Receives golang context and CSI Spec CreateVolumeRequest
// Returns CSI Spec CreateVolumeResponse or error if something went wrong
func (c *CSIControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "CreateVolume",
		"volumeID": req.GetName(),
	})
	ll.Infof("Processing request: %v", req)

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name missing in request")
	}
	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities missing in request")
	}

	preferredNode := ""
	if req.GetAccessibilityRequirements() != nil && len(req.GetAccessibilityRequirements().Preferred) > 0 {
		preferredNode = req.GetAccessibilityRequirements().Preferred[0].Segments[csibmnodeconst.NodeIDAnnotationKey]
		ll.Infof("Preferred node was provided: %s", preferredNode)
	}

	var (
		fsType string
		err    error
		mode   string
		vol    *api.Volume
	)

	if accessType, ok := req.GetVolumeCapabilities()[0].AccessType.(*csi.VolumeCapability_Mount); ok {
		fsType = strings.ToLower(accessType.Mount.FsType) // ext4 by default (from request)
		mode = apiV1.ModeFS
	} else {
		return nil, status.Error(codes.Unimplemented, "Block mode is unimplemented")
	}

	c.reqMu.Lock()
	vol, err = c.svc.CreateVolume(ctx, api.Volume{
		Id:           req.Name,
		StorageClass: util.ConvertStorageClass(req.Parameters[base.StorageTypeKey]),
		NodeId:       preferredNode,
		Size:         req.GetCapacityRange().GetRequiredBytes(),
		Mode:         mode,
		Type:         fsType,
	})
	c.reqMu.Unlock()

	if err != nil {
		return nil, err
	}

	if vol.CSIStatus == apiV1.Creating {
		ll.Infof("Waiting until volume will reach Created status. Current status - %s", vol.CSIStatus)
		if err := c.svc.WaitStatus(ctx, vol.Id, apiV1.Failed, apiV1.Created); err != nil {
			return nil, status.Error(codes.Internal, "Unable to create volume")
		}
	}

	ll.Infof("Construct response based on volume: %v", vol)
	topologyList := []*csi.Topology{
		{Segments: map[string]string{csibmnodeconst.NodeIDAnnotationKey: vol.NodeId}},
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           req.Name,
			CapacityBytes:      vol.Size,
			VolumeContext:      req.GetParameters(),
			AccessibleTopology: topologyList,
		},
	}, nil
}

// DeleteVolume is the implementation of CSI Spec DeleteVolume. This method sets Volume CR's Spec.CSIStatus to Removing.
// And waits for Volume to be removed by Reconcile loop of appropriate Node.
// Receives golang context and CSI Spec DeleteVolumeRequest
// Returns CSI Spec DeleteVolumeResponse or error if something went wrong
func (c *CSIControllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "DeleteVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("Processing request: %v", req)

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}
	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, req.VolumeId)

	c.reqMu.Lock()
	err := c.svc.DeleteVolume(ctxWithID, req.GetVolumeId())
	c.reqMu.Unlock()

	if err != nil {
		if k8sError.IsNotFound(err) {
			ll.Infof("Volume doesn't exist")
			return &csi.DeleteVolumeResponse{}, nil
		}
		ll.Errorf("Unable to delete volume: %v", err)
		return nil, err
	}
	if err = c.svc.WaitStatus(ctx, req.VolumeId, apiV1.Failed, apiV1.Removed); err != nil {
		return nil, status.Error(codes.Internal, "Unable to delete volume")
	}

	c.reqMu.Lock()
	c.svc.UpdateCRsAfterVolumeDeletion(ctxWithID, req.VolumeId)
	c.reqMu.Unlock()

	ll.Debug("Volume was successfully deleted")

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume is the implementation of CSI Spec ControllerPublishVolume. This method just checks existence
// of provided Volume CR and returns success response if the Volume CR exists.
// Receives golang context and CSI Spec ControllerPublishVolumeRequest
// Returns CSI Spec ControllerPublishVolumeResponse or error if something went wrong
func (c *CSIControllerService) ControllerPublishVolume(ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "ControllerPublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume: Node ID must be provided")
	}

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume: Volume ID must be provided")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume: Volume capabilities"+
			" must be provided")
	}

	vol := &volumecrd.Volume{}
	if err := c.k8sclient.ReadCR(ctx, req.VolumeId, vol); err != nil {
		if k8sError.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "Volume is not found")
		}
		ll.Errorf("k8s client can't read volume CR: %v", err)
		return nil, status.Error(codes.Unavailable, "Something went wrong with k8s client")
	}

	ll.Info("Return empty response, ok.")

	return &csi.ControllerPublishVolumeResponse{}, nil
}

// ControllerUnpublishVolume is the implementation of CSI Spec ControllerUnpublishVolume.
// This method just returns empty response.
// Receives golang context and CSI Spec ControllerUnpublishVolumeRequest
// Returns CSI Spec ControllerUnpublishVolumeResponse or error if Volume ID is not provided in request
func (c *CSIControllerService) ControllerUnpublishVolume(ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":   "ControllerUnpublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume: Volume ID must be provided")
	}

	ll.Info("Return empty response, ok")

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities is not implemented yet
func (c *CSIControllerService) ValidateVolumeCapabilities(context.Context, *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

// ListVolumes is not implemented yet
func (c *CSIControllerService) ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

// GetCapacity is not implemented yet
func (c *CSIControllerService) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

// ControllerGetCapabilities is the implementation of CSI Spec ControllerGetCapabilities.
// Provides Controller capabilities of CSI driver to k8s CREATE/DELETE Volume and PUBLISH/UNPUBLISH Volume for now.
// Receives golang context and CSI Spec ControllerGetCapabilitiesRequest
// Returns CSI Spec ControllerGetCapabilitiesResponse and nil error
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

// CreateSnapshot is not implemented yet
func (c *CSIControllerService) CreateSnapshot(context.Context, *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

// DeleteSnapshot is not implemented yet
func (c *CSIControllerService) DeleteSnapshot(context.Context, *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

// ListSnapshots is not implemented yet
func (c *CSIControllerService) ListSnapshots(context.Context, *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

// ControllerExpandVolume is not implemented yet
func (c *CSIControllerService) ControllerExpandVolume(context.Context, *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
