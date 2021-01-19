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

// Package node contains implementation of CSI Node component
package node

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/keymutex"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/cache"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/common"
	"github.com/dell/csi-baremetal/pkg/controller"
	csibmnodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/csibmnode/common"
)

// CSINodeService is the implementation of NodeServer interface from GO CSI specification.
// Contains VolumeManager in a such way that it is a single instance in the driver
type CSINodeService struct {
	svc   common.VolumeOperations
	reqMu sync.Mutex

	log           *logrus.Entry
	livenessCheck LivenessHelper
	cache         cache.WrapCache
	VolumeManager
	csi.IdentityServer
	grpc_health_v1.HealthServer

	// used for locking requests on each volume
	volMu keymutex.KeyMutex
}

const (
	// PodNameKey to read pod name from PodInfoOnMount feature
	PodNameKey = "csi.storage.k8s.io/pod.name"
	// UnknownPodName is used when pod name isn't provided in request
	UnknownPodName = "UNKNOWN"
	// EphemeralKey in volume context means that in node publish request we need to create ephemeral volume
	EphemeralKey = "csi.storage.k8s.io/ephemeral"
)

// NewCSINodeService is the constructor for CSINodeService struct
// Receives an instance of DriveServiceClient to interact with DriveManager, ID of a node where it works, logrus logger
// and base.KubeClient
// Returns an instance of CSINodeService
func NewCSINodeService(client api.DriveServiceClient,
	nodeID string,
	logger *logrus.Logger,
	k8sclient *k8s.KubeClient,
	recorder eventRecorder,
	featureConf featureconfig.FeatureChecker) *CSINodeService {
	e := &command.Executor{}
	e.SetLogger(logger)
	s := &CSINodeService{
		VolumeManager:  *NewVolumeManager(client, e, logger, k8sclient, recorder, nodeID),
		svc:            common.NewVolumeOperationsImpl(k8sclient, logger, featureConf),
		IdentityServer: controller.NewIdentityServer(base.PluginName, base.PluginVersion),
		volMu:          keymutex.NewHashed(0),
		livenessCheck:  NewLivenessCheckHelper(logger, nil, nil),
		cache:          cache.NewCacheWrapper(k8sclient),
	}
	s.log = logger.WithField("component", "CSINodeService")
	return s
}

// Probe is the implementation of CSI Spec Probe for IdentityServer.
// This method checks if CSI driver is ready to serve requests
// overrides same method from identityServer struct in controller package
func (s *CSINodeService) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: s.livenessCheck.Check(),
		},
	}, nil
}

// checkRequestContext checks whether provided context is done or no, return error in case of done context
func (s *CSINodeService) checkRequestContext(ctx context.Context, logger *logrus.Entry) error {
	select {
	case <-ctx.Done():
		msg := fmt.Sprintf("context is done after volume lock. err: %s", ctx.Err())
		logger.Warn(msg)
		return errors.New(msg)
	default:
		logger.Info("Processing request")
		return nil
	}
}

// NodeStageVolume is the implementation of CSI Spec NodeStageVolume. Performs when the first pod consumes a volume.
// This method mounts volume with appropriate VolumeID into the StagingTargetPath from request.
// Receives golang context and CSI Spec NodeStageVolumeRequest
// Returns CSI Spec NodeStageVolumeResponse or error if something went wrong
func (s *CSINodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeStageVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("locking volume on request: %v", req)
	s.volMu.LockKey(req.GetVolumeId())
	defer func() {
		err := s.volMu.UnlockKey(req.GetVolumeId())
		if err != nil {
			ll.Warnf("Unlocking  volume with error %s", err)
		}
	}()
	if err := s.checkRequestContext(ctx, ll); err != nil {
		return nil, err
	}

	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Stage Path missing in request")
	}
	var (
		volumeID  = req.VolumeId
		newVolume *volumecrd.Volume
	)
	volumeCR, err := s.crHelper.GetVolumeByID(volumeID)
	if err != nil {
		message := fmt.Sprintf("Unable to find volume with ID %s", volumeID)
		ll.Error(message)
		return nil, status.Error(codes.NotFound, message)
	}
	namespace, err := s.cache.GetVolumeNamespace(volumeID)
	if err != nil || namespace == "" {
		ll.Errorf("Failed to get volume namespace: %v", err)
		return nil, status.Error(codes.Unavailable, "Something went wrong with k8s client")
	}
	if volumeCR.Namespace != namespace {
		volumeCR.Spec.CSIStatus = apiV1.Empty
		volumeCR.Finalizers = nil

		if err := s.k8sClient.UpdateCR(ctx, volumeCR); err != nil {
			ll.Errorf("Unable to update volume, error: %v", err)
			return nil, status.Errorf(codes.Internal, "unable to update volume")
		}

		if err := s.k8sClient.DeleteCR(ctx, volumeCR); err != nil {
			ll.Errorf("Unable to delete volume, error: %v", err)
			return nil, status.Errorf(codes.Internal, "unable to delete volume")
		}

		volumeCR.Spec.CSIStatus = apiV1.Created
		newVolume = s.k8sClient.ConstructVolumeCR(volumeCR.Spec.Id, namespace, volumeCR.Spec)

		if err := s.k8sClient.CreateCR(ctx, volumeCR.Spec.Id, newVolume); err != nil {
			ll.Errorf("Unable to create volume, error: %v", err)
			return nil, status.Errorf(codes.Internal, "unable to create volume")
		}
	}
	if newVolume == nil {
		newVolume = volumeCR
	}
	currStatus := newVolume.Spec.CSIStatus
	// if currStatus not in [Created (first call), VolumeReady (retry), Published (multiple pods)]
	if currStatus != apiV1.Created && currStatus != apiV1.VolumeReady && currStatus != apiV1.Published {
		ll.Errorf("Current volume CR status - %s, expected to be in - [%s, %s, %s]",
			currStatus, apiV1.Created, apiV1.VolumeReady, apiV1.Published)
		return nil, fmt.Errorf("corresponding volume CR is in unexpected state - %s",
			currStatus)
	}

	targetPath := req.StagingTargetPath

	partition, err := s.getProvisionerForVolume(&newVolume.Spec).GetVolumePath(newVolume.Spec)
	if err != nil {
		ll.Errorf("failed to get partition, for volume %v: %v", newVolume.Spec, err)
		return nil, status.Error(codes.Internal, "failed to stage volume: partition error")
	}
	ll.Infof("Work with partition %s", partition)

	var (
		resp        = &csi.NodeStageVolumeResponse{}
		errToReturn error
		newStatus   = apiV1.VolumeReady
	)
	if err := s.fsOps.PrepareAndPerformMount(partition, targetPath, false); err != nil {
		ll.Errorf("Unable to prepare and mount: %v. Going to set volumes status to failed", err)
		newStatus = apiV1.Failed
		resp, errToReturn = nil, status.Error(codes.Internal, "failed to stage volume: mount error")
	}

	if currStatus != apiV1.VolumeReady || newStatus == apiV1.Failed {
		newVolume.Spec.CSIStatus = newStatus
		if err := s.crHelper.UpdateVolumeCRSpec(newVolume.Name, newVolume.Namespace, newVolume.Spec); err != nil {
			ll.Errorf("Unable to set volume status to %s: %v", newStatus, err)
			resp, errToReturn = nil, fmt.Errorf("failed to stage volume: update volume CR error")
		}
	}

	return resp, errToReturn
}

// NodeUnstageVolume is the implementation of CSI Spec NodeUnstageVolume. Performs when the last pod stops consume
// a volume. This method unmounts volume with appropriate VolumeID from the StagingTargetPath from request.
// Receives golang context and CSI Spec NodeUnstageVolumeRequest
// Returns CSI Spec NodeUnstageVolumeResponse or error if something went wrong
func (s *CSINodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeUnstageVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("locking volume on request: %v", req)
	s.volMu.LockKey(req.GetVolumeId())
	defer func() {
		err := s.volMu.UnlockKey(req.GetVolumeId())
		if err != nil {
			ll.Warnf("Unlocking  volume with error %s", err)
		}
	}()
	if err := s.checkRequestContext(ctx, ll); err != nil {
		return nil, err
	}

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Stage Path missing in request")
	}
	volumeCR, err := s.crHelper.GetVolumeByID(req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "Unable to find volume")
	}

	currStatus := volumeCR.Spec.CSIStatus
	if currStatus == apiV1.Created {
		ll.Info("Volume has been already unstaged")
		return &csi.NodeUnstageVolumeResponse{}, nil
	} else if currStatus != apiV1.VolumeReady {
		msg := fmt.Sprintf("current volume CR status - %s, expected to be in [%s, %s]",
			currStatus, apiV1.Created, apiV1.VolumeReady)
		ll.Error(msg)
		return nil, status.Error(codes.FailedPrecondition, msg)
	}

	// This is a temporary solution to clear all owners during NodeUnstage
	// because NodeUnpublishRequest doesn't contain info about pod
	// TODO: remove owner from Owners slice during Unpublish properly - https://github.com/dell/csi-baremetal/issues/86
	// volumeCR.Spec.Owners = nil
	volumeCR.Spec.CSIStatus = apiV1.Created

	var (
		resp        = &csi.NodeUnstageVolumeResponse{}
		errToReturn error
	)
	if errToReturn = s.fsOps.UnmountWithCheck(req.GetStagingTargetPath()); errToReturn != nil {
		volumeCR.Spec.CSIStatus = apiV1.Failed
		resp = nil
	}

	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, req.GetVolumeId())
	if updateErr := s.k8sClient.UpdateCR(ctxWithID, volumeCR); updateErr != nil {
		ll.Errorf("Unable to update volume CR: %v", updateErr)
		resp, errToReturn = nil, fmt.Errorf("failed to unstage volume: update volume CR error")
	}

	ll.Debugf("Unstaged - %v", errToReturn == nil)
	return resp, errToReturn
}

// NodePublishVolume is the implementation of CSI Spec NodePublishVolume. Performs each time pod starts consume
// a volume. This method perform bind mount of volume with appropriate VolumeID from the StagingTargetPath to TargetPath.
// Receives golang context and CSI Spec NodePublishVolumeRequest
// Returns CSI Spec NodePublishVolumeResponse or error if something went wrong
func (s *CSINodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodePublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("locking volume on request: %v", req)
	s.volMu.LockKey(req.GetVolumeId())
	defer func() {
		err := s.volMu.UnlockKey(req.GetVolumeId())
		if err != nil {
			ll.Warnf("Unlocking  volume with error %s", err)
		}
	}()
	if err := s.checkRequestContext(ctx, ll); err != nil {
		return nil, err
	}

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
	var (
		inline bool
		err    error
	)

	if req.GetVolumeContext() != nil {
		val, ok := req.GetVolumeContext()[EphemeralKey]
		if ok {
			inline, err = strconv.ParseBool(val)
			if err != nil {
				ll.Errorf("Failed to parse bool: %v", err)
				return nil, status.Error(codes.Internal, "failed to determine whether volume ephemeral or no")
			}
		}
	}

	var (
		volumeID = req.GetVolumeId()
		srcPath  = req.GetStagingTargetPath()
		dstPath  = req.GetTargetPath()
		bind     = true // for mount option
	)
	// Inline volume has the same cycle as usual volume,
	// but k8s calls only Publish/Unpulish methods so we need to call CreateVolume before publish it
	if inline {
		vol, err := s.createInlineVolume(ctx, volumeID, req)
		if err != nil {
			ll.Errorf("Failed to create inline volume: %v", err)
			return nil, status.Error(codes.Internal, "unable to create inline volume")
		}
		srcPath, err = s.getProvisionerForVolume(vol).GetVolumePath(*vol)
		if err != nil {
			ll.Errorf("failed to get partition for volume %v: %v", vol, err)
			return nil, status.Error(codes.Internal, "failed to publish inline volume: partition error")
		}
		// For inline volume mount is performed without options
		bind = false
	} else if len(srcPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging Path missing in request")
	}

	volumeCR, err := s.crHelper.GetVolumeByID(volumeID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Unable to find volume")
	}

	currStatus := volumeCR.Spec.CSIStatus
	// if currStatus not in [VolumeReady, Published], but for inline volume we expect Created status
	if currStatus != apiV1.VolumeReady && currStatus != apiV1.Published && !inline {
		msg := fmt.Sprintf("current volume CR status - %s, expected to be in [%s, %s]",
			currStatus, apiV1.VolumeReady, apiV1.Published)
		ll.Error(msg)
		return nil, status.Error(codes.FailedPrecondition, msg)
	}

	var (
		resp        = &csi.NodePublishVolumeResponse{}
		newStatus   = apiV1.Published
		errToReturn error
	)

	if err := s.fsOps.PrepareAndPerformMount(srcPath, dstPath, bind); err != nil {
		ll.Errorf("Unable to mount volume: %v", err)
		newStatus = apiV1.Failed
		resp, errToReturn = nil, fmt.Errorf("failed to publish volume: mount error")
	}

	// TODO: need to provide better logic for volumes Owners https://github.com/dell/csi-baremetal/issues/86
	// add volume owner info
	// var podName string
	// podName, ok := req.VolumeContext[PodNameKey]
	// if !ok {
	//	podName = UnknownPodName
	//	ll.Warnf("flag podInfoOnMount isn't provided will add %s for volume owners", podName)
	// }
	//
	// owners := volumeCR.Spec.Owners
	// if !util.ContainsString(owners, podName) { // check whether podName already in owners or no
	//	owners = append(owners, podName)
	//	volumeCR.Spec.Owners = owners
	// }

	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, volumeID)
	volumeCR.Spec.CSIStatus = newStatus
	if err = s.k8sClient.UpdateCR(ctxWithID, volumeCR); err != nil {
		ll.Errorf("Unable to update volume CR to %v, error: %v", volumeCR, err)
		resp, errToReturn = nil, fmt.Errorf("failed to publish volume: update volume CR error")
	}
	return resp, errToReturn
}

// createInlineVolume encapsulate logic for creating inline volumes
func (s *CSINodeService) createInlineVolume(ctx context.Context, volumeID string, req *csi.NodePublishVolumeRequest) (*api.Volume, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "createInlineVolume",
		"volumeID": volumeID,
	})

	var (
		volumeContext = req.GetVolumeContext() // verified in NodePublishVolume method
		bytesStr      = volumeContext[base.SizeKey]
		fsType        = ""
		mode          string
		scl           string
		bytes         int64
		err           error
	)

	if bytes, err = util.StrToBytes(bytesStr); err != nil {
		return nil, err
	}

	if accessType, ok := req.GetVolumeCapability().AccessType.(*csi.VolumeCapability_Mount); ok {
		fsType = strings.ToLower(accessType.Mount.FsType)
		if fsType == "" {
			fsType = base.DefaultFsType
			ll.Infof("FS type wasn't provide. Will use %s as a default value", fsType)
		}
		mode = apiV1.ModeFS
	}

	scl = util.ConvertStorageClass(volumeContext[base.StorageTypeKey])
	if scl == apiV1.StorageClassAny {
		scl = apiV1.StorageClassHDD // do not use sc ANY for inline volumes
	}

	s.reqMu.Lock()
	vol, err := s.svc.CreateVolume(ctx, api.Volume{
		Id:           volumeID,
		StorageClass: scl,
		NodeId:       s.nodeID,
		Size:         bytes,
		Ephemeral:    true,
		Mode:         mode,
		Type:         fsType,
	})
	s.reqMu.Unlock()
	if err != nil {
		return nil, err
	}

	if vol.CSIStatus == apiV1.Creating {
		if err = s.svc.WaitStatus(ctx, vol.Id, apiV1.Failed, apiV1.Created); err != nil {
			return nil, err
		}
	}

	return vol, nil
}

// NodeUnpublishVolume is the implementation of CSI Spec NodePublishVolume. Performs each time pod stops consume a volume.
// This method unmounts volume with appropriate VolumeID from the TargetPath.
// Receives golang context and CSI Spec NodeUnpublishVolumeRequest
// Returns CSI Spec NodeUnpublishVolumeResponse or error if something went wrong
func (s *CSINodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeUnpublishVolume",
		"volumeID": req.GetVolumeId(),
	})

	ll.Infof("locking volume on request: %v", req)
	s.volMu.LockKey(req.GetVolumeId())
	defer func() {
		err := s.volMu.UnlockKey(req.GetVolumeId())
		if err != nil {
			ll.Warnf("Unlocking volume with error %s", err)
		}
	}()
	if err := s.checkRequestContext(ctx, ll); err != nil {
		return nil, err
	}

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target Path missing in request")
	}

	volumeCR, err := s.crHelper.GetVolumeByID(req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "Unable to find volume")
	}

	currStatus := volumeCR.Spec.CSIStatus
	// if currStatus not in [VolumeReady, Published]
	if currStatus != apiV1.VolumeReady && currStatus != apiV1.Published {
		msg := fmt.Sprintf("current volume CR status - %s, expected to be in [%s, %s]",
			currStatus, apiV1.VolumeReady, apiV1.Published)
		ll.Error(msg)
		return nil, status.Error(codes.FailedPrecondition, msg)
	}

	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, req.GetVolumeId())
	if err := s.fsOps.UnmountWithCheck(req.GetTargetPath()); err != nil {
		ll.Errorf("Unable to unmount volume: %v", err)
		volumeCR.Spec.CSIStatus = apiV1.Failed
		if updateErr := s.k8sClient.UpdateCR(ctxWithID, volumeCR); updateErr != nil {
			ll.Errorf("Unable to set volume CR status to failed: %v", updateErr)
		}
		return nil, status.Error(codes.Internal, "unmount error")
	}
	// If volume has more than 1 owner pods then keep its status as Published
	// if len(volumeCR.Spec.Owners) > 1 {
	//	return &csi.NodeUnpublishVolumeResponse{}, nil
	// }

	// k8s doesn't call DeleteVolume for inline volumes, so we perform DeleteVolume operation in Unpublish request
	if volumeCR.Spec.Ephemeral {
		s.reqMu.Lock()
		err := s.svc.DeleteVolume(ctxWithID, req.GetVolumeId())
		s.reqMu.Unlock()

		if err != nil {
			if k8sError.IsNotFound(err) {
				ll.Infof("Volume doesn't exist")
				return &csi.NodeUnpublishVolumeResponse{}, nil
			}
			ll.Errorf("Unable to delete volume: %v", err)
			return nil, err
		}

		if err = s.svc.WaitStatus(ctx, req.VolumeId, apiV1.Failed, apiV1.Removed); err != nil {
			ll.Warn("Status wasn't reached")
			return nil, status.Error(codes.Internal, "Unable to delete volume")
		}
		s.reqMu.Lock()
		s.svc.UpdateCRsAfterVolumeDeletion(ctxWithID, req.VolumeId)
		s.reqMu.Unlock()
	} else {
		volumeCR.Spec.CSIStatus = apiV1.VolumeReady
		if updateErr := s.k8sClient.UpdateCR(ctxWithID, volumeCR); updateErr != nil {
			ll.Errorf("Unable to set volume CR status to VolumeReady: %v", updateErr)
		}
	}

	ll.Debugf("Unpublished successfully")
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetVolumeStats returns empty response
func (s *CSINodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil
}

// NodeExpandVolume returns empty response
func (s *CSINodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

// NodeGetCapabilities is the implementation of CSI Spec NodeGetCapabilities.
// Provides Node capabilities of CSI driver to k8s. STAGE/UNSTAGE Volume for now.
// Receives golang context and CSI Spec NodeGetCapabilitiesRequest
// Returns CSI Spec NodeGetCapabilitiesResponse and nil error
func (s *CSINodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{Capabilities: []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		}},
	}, nil
}

// NodeGetInfo is the implementation of CSI Spec NodeGetInfo. It plays a role in CSI Topology feature when Controller
// chooses a node where to deploy a volume.
// Receives golang context and CSI Spec NodeGetInfoRequest
// Returns CSI Spec NodeGetInfoResponse with topology NodeIDAnnotationKey: NodeID and nil error
func (s *CSINodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method": "NodeGetInfo",
	})

	topology := csi.Topology{
		Segments: map[string]string{
			csibmnodeconst.NodeIDAnnotationKey: s.nodeID,
		},
	}

	ll.Infof("NodeGetInfo created topology: %v", topology)

	return &csi.NodeGetInfoResponse{
		NodeId:             s.nodeID,
		AccessibleTopology: &topology,
	}, nil
}

// Check does the health check and changes the status of the server based on drives cache size
func (s *CSINodeService) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method": "Check",
	})

	if !s.initialized {
		ll.Info("Node svc is not ready yet")
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

// Watch is used by clients to receive updates when the svc status changes.
// Watch only dummy implemented just to satisfy the interface.
func (s *CSINodeService) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

// GetLivenessHelper return instance of livenesshelper used by node service
func (s *CSINodeService) GetLivenessHelper() LivenessHelper {
	return s.livenessCheck
}
