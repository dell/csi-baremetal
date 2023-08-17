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
	"path"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/keymutex"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/cache"
	"github.com/dell/csi-baremetal/pkg/base/command"
	baseerr "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/common"
	"github.com/dell/csi-baremetal/pkg/controller"
	"github.com/dell/csi-baremetal/pkg/controller/mountoptions"
	csibmnodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/metrics"
	metricscomm "github.com/dell/csi-baremetal/pkg/metrics/common"
)

const (
	stagingFileName = "dev"

	fakeAttachVolumeAnnotation = "fake-attach"
	fakeAttachVolumeKey        = "yes"

	fakeDeviceVolumeAnnotation = "fake-device"
	fakeDeviceSrcFileDir       = "/var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/fake/"

	wbtChangedVolumeAnnotation = "wbt-changed"
	wbtChangedVolumeKey        = "yes"
)

// CSINodeService is the implementation of NodeServer interface from GO CSI specification.
// Contains VolumeManager in a such way that it is a single instance in the driver
type CSINodeService struct {
	svc common.VolumeOperations

	log           *logrus.Entry
	livenessCheck LivenessHelper
	VolumeManager
	csi.IdentityServer
	grpc_health_v1.HealthServer

	// used for locking requests on each volume
	volMu               keymutex.KeyMutex
	metricStageVolume   metrics.StatisticWithCustomLabels
	metricPublishVolume metrics.StatisticWithCustomLabels
}

const (
	// UnknownPodName is used when pod name isn't provided in request
	UnknownPodName = "UNKNOWN"
)

// NewCSINodeService is the constructor for CSINodeService struct
// Receives an instance of DriveServiceClient to interact with DriveManager, ID of a node where it works, logrus logger
// and base.KubeClient
// Returns an instance of CSINodeService
func NewCSINodeService(client api.DriveServiceClient,
	nodeID string,
	nodeName string,
	logger *logrus.Logger,
	k8sClient *k8s.KubeClient,
	k8sCache k8s.CRReader,
	recorder eventRecorder,
	featureConf featureconfig.FeatureChecker) *CSINodeService {
	metricStageVolume := metricscomm.DbgNodeStageDuration
	metricPublishVolume := metricscomm.DbgNodePublishDuration
	e := command.NewExecutor(logger)
	s := &CSINodeService{
		VolumeManager:       *NewVolumeManager(client, e, logger, k8sClient, k8sCache, recorder, nodeID, nodeName),
		svc:                 common.NewVolumeOperationsImpl(k8sClient, logger, cache.NewMemCache(), featureConf),
		IdentityServer:      controller.NewIdentityServer(base.PluginName, base.PluginVersion),
		volMu:               keymutex.NewHashed(0),
		livenessCheck:       NewLivenessCheckHelper(logger, nil, nil),
		metricStageVolume:   metricStageVolume,
		metricPublishVolume: metricPublishVolume,
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

func getStagingPath(logger *logrus.Entry, stagingPath string) string {
	if stagingPath != "" {
		stagingPath = path.Join(stagingPath, stagingFileName)
	}
	logger.Debugf("staging path is: %s", stagingPath)
	return stagingPath
}

func (s *CSINodeService) processFakeAttachInNodeStageVolume(ll *logrus.Entry, volumeCR *volumecrd.Volume, targetPath string, isFakeAttachNeed bool) error {
	volumeID := volumeCR.Spec.Id
	if isFakeAttachNeed {
		if volumeCR.Annotations[fakeAttachVolumeAnnotation] != fakeAttachVolumeKey {
			volumeCR.Annotations[fakeAttachVolumeAnnotation] = fakeAttachVolumeKey
			ll.Warningf("Adding fake-attach annotation to the volume with ID %s", volumeID)
			s.VolumeManager.recorder.Eventf(volumeCR, eventing.FakeAttachInvolved,
				"Fake-attach involved for volume with ID %s", volumeID)
		}
		// mount fake device in the non-fs mode
		if volumeCR.Spec.Mode != apiV1.ModeFS {
			fakeDevice, err := s.VolumeManager.createFakeDeviceIfNecessary(ll, volumeCR)
			if err != nil {
				ll.Errorf("unable to create fake device in stage volume request with error: %v", err)
				return status.Error(codes.Internal, fmt.Sprintf("failed to create fake device in stage volume: %s", err.Error()))
			}
			volumeCR.Annotations[fakeDeviceVolumeAnnotation] = fakeDevice
			if err := s.fsOps.PrepareAndPerformMount(fakeDevice, targetPath, true, false); err != nil {
				ll.Errorf("unable to mount fake device in stage volume request with error: %v", err)
				return status.Error(codes.Internal, fmt.Sprintf("failed to mount device in stage volume: %s", err.Error()))
			}
		}
	} else if volumeCR.Annotations[fakeAttachVolumeAnnotation] == fakeAttachVolumeKey {
		delete(volumeCR.Annotations, fakeAttachVolumeAnnotation)
		ll.Warningf("Removing fake-attach annotation for volume %s", volumeID)
		s.VolumeManager.recorder.Eventf(volumeCR, eventing.FakeAttachCleared,
			"Fake-attach cleared for volume with ID %s", volumeID)
		// clean fake device in the non-fs mode
		if volumeCR.Spec.Mode != apiV1.ModeFS {
			s.VolumeManager.cleanFakeDevice(ll, volumeCR)
			delete(volumeCR.Annotations, fakeDeviceVolumeAnnotation)
		}
	}
	return nil
}

// NodeStageVolume is the implementation of CSI Spec NodeStageVolume. Performs when the first pod consumes a volume.
// This method mounts volume with appropriate VolumeID into the StagingTargetPath from request.
// Receives golang context and CSI Spec NodeStageVolumeRequest
// Returns CSI Spec NodeStageVolumeResponse or error if something went wrong
func (s *CSINodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (retresp *csi.NodeStageVolumeResponse, reterr error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodeStageVolume",
		"volumeID": req.GetVolumeId(),
	})
	defer s.metricStageVolume.EvaluateDurationForMethod("NodeStageVolume", prometheus.Labels{"volume_name": req.GetVolumeId()})()
	defer func() {
		if retresp != nil && reterr == nil && ctx.Err() == nil {
			go func() {
				time.Sleep(time.Second * metrics.DbgMetricHoldTime)
				s.metricStageVolume.Clear(prometheus.Labels{"volume_name": req.GetVolumeId()})
			}()
		}
	}()

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

	volumeID := req.GetVolumeId()
	volumeCR, err := s.crHelper.GetVolumeByID(volumeID)
	if err != nil {
		message := fmt.Sprintf("Unable to find volume with ID %s", volumeID)
		ll.Error(message)
		return nil, status.Error(codes.NotFound, message)
	}

	currStatus := volumeCR.Spec.CSIStatus
	switch currStatus {
	// expected currStatus in [Created (first call), VolumeReady (retry), Published (multiple pods)]
	case apiV1.Created, apiV1.VolumeReady, apiV1.Published:
		ll.Infof("Volume status: %s", currStatus)
	// also need to retry on FAILED
	case apiV1.Failed:
		ll.Warningf("Volume status: %s. Need to retry.", currStatus)
	default:
		ll.Errorf("Unexpected volume status: %s", currStatus)
		return nil, fmt.Errorf("corresponding volume is in unexpected state - %s", currStatus)
	}

	var (
		resp        = &csi.NodeStageVolumeResponse{}
		errToReturn error
		newStatus   = apiV1.VolumeReady
	)

	if volumeCR.Annotations == nil {
		volumeCR.Annotations = make(map[string]string)
	}

	targetPath := getStagingPath(ll, req.GetStagingTargetPath())

	isFakeAttachNeed := false
	ignoreErrorIfFakeAttach := func(err error) {
		if s.isPVCNeedFakeAttach(volumeID) {
			isFakeAttachNeed = true
		} else {
			newStatus = apiV1.Failed
			resp, errToReturn = nil, status.Error(codes.Internal, fmt.Sprintf("failed to stage volume: %s", err.Error()))
		}
	}

	partition, err := s.getProvisionerForVolume(&volumeCR.Spec).GetVolumePath(&volumeCR.Spec)
	if err != nil {
		if err == baseerr.ErrorGetDriveFailed {
			return nil, err
		}
		ll.Errorf("failed to get partition for volume %v: %v", volumeCR.Spec, err)
		ignoreErrorIfFakeAttach(err)
	} else {
		ll.Infof("Partition to stage: %s", partition)
		if err := s.fsOps.PrepareAndPerformMount(partition, targetPath, true, false); err != nil {
			ll.Errorf("Unable to stage volume: %v", err)
			ignoreErrorIfFakeAttach(err)
		}
	}

	if err := s.processFakeAttachInNodeStageVolume(ll, volumeCR, targetPath, isFakeAttachNeed); err != nil {
		return nil, err
	}

	if s.VolumeManager.checkWbtChangingEnable(ctx, volumeCR) {
		if err := s.VolumeManager.setWbtValue(volumeCR); err != nil {
			ll.Errorf("Unable to set custom WBT value for volume %s: %v", volumeCR.Name, err)
			s.VolumeManager.recorder.Eventf(volumeCR, eventing.WBTValueSetFailed,
				"Unable to set custom WBT value for volume %s", volumeCR.Name)
		} else {
			volumeCR.Annotations[wbtChangedVolumeAnnotation] = wbtChangedVolumeKey
		}
	}

	if currStatus != apiV1.VolumeReady {
		volumeCR.Spec.CSIStatus = newStatus
		if err := s.k8sClient.UpdateCR(ctx, volumeCR); err != nil {
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

	volumeID := req.GetVolumeId()
	volumeCR, err := s.crHelper.GetVolumeByID(volumeID)
	if err != nil {
		// If NodeUnstageVolume return error for not found Volume CR
		// kubelet will be call NodeUnstageVolume again, it means infinity calls
		if err == baseerr.ErrorNotFound {
			return &csi.NodeUnstageVolumeResponse{}, nil
		}
		message := fmt.Sprintf("Unable to find volume with ID %s: %s", volumeID, err.Error())
		return nil, status.Error(codes.Internal, message)
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

	volumeCR.Spec.CSIStatus = apiV1.Created

	var (
		resp        = &csi.NodeUnstageVolumeResponse{}
		errToReturn error
	)

	if volumeCR.Annotations[fakeAttachVolumeAnnotation] != fakeAttachVolumeKey || volumeCR.Spec.Mode != apiV1.ModeFS {
		targetPath := getStagingPath(ll, req.GetStagingTargetPath())
		errToReturn = s.fsOps.UnmountWithCheck(targetPath)
		if errToReturn == nil {
			errToReturn = s.fsOps.RmDir(targetPath)
		}

		if errToReturn != nil {
			volumeCR.Spec.CSIStatus = apiV1.Failed
			resp = nil
		}
	}

	if val, ok := volumeCR.Annotations[wbtChangedVolumeAnnotation]; ok && val == wbtChangedVolumeKey {
		delete(volumeCR.Annotations, wbtChangedVolumeAnnotation)
		if err := s.VolumeManager.restoreWbtValue(volumeCR); err != nil {
			ll.Errorf("Unable to restore WBT value for volume %s: %v", volumeCR.Name, err)
			s.VolumeManager.recorder.Eventf(volumeCR, eventing.WBTValueSetFailed,
				"Unable to restore WBT value for volume %s", volumeCR.Name)
		}
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
func (s *CSINodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (retresp *csi.NodePublishVolumeResponse, reterr error) {
	ll := s.log.WithFields(logrus.Fields{
		"method":   "NodePublishVolume",
		"volumeID": req.GetVolumeId(),
	})
	defer s.metricPublishVolume.EvaluateDurationForMethod("NodePublishVolume", prometheus.Labels{"volume_name": req.GetVolumeId()})()
	defer func() {
		if retresp != nil && reterr == nil && ctx.Err() == nil {
			go func() {
				time.Sleep(time.Second * metrics.DbgMetricHoldTime)
				s.metricPublishVolume.Clear(prometheus.Labels{"volume_name": req.GetVolumeId()})
			}()
		}
	}()

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
		err          error
		mountOptions []string
	)

	if accessType, ok := req.GetVolumeCapability().AccessType.(*csi.VolumeCapability_Mount); ok {
		mountOptions = mountoptions.FilterWithType(mountoptions.PublishCmdOpt, accessType.Mount.GetMountFlags())
	}

	var (
		volumeID = req.GetVolumeId()
		srcPath  = getStagingPath(ll, req.GetStagingTargetPath())
		dstPath  = req.GetTargetPath()
	)

	if len(srcPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Staging Path missing in request")
	}

	volumeCR, err := s.crHelper.GetVolumeByID(volumeID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Unable to find volume")
	}

	currStatus := volumeCR.Spec.CSIStatus
	// if currStatus not in [VolumeReady, Published]
	if currStatus != apiV1.VolumeReady && currStatus != apiV1.Published {
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

	if volumeCR.Annotations[fakeAttachVolumeAnnotation] == fakeAttachVolumeKey && volumeCR.Spec.Mode == apiV1.ModeFS {
		if err := s.fsOps.MountFakeTmpfs(volumeID, dstPath); err != nil {
			newStatus = apiV1.Failed
			resp, errToReturn = nil, fmt.Errorf("failed to publish volume: fake attach error %s", err.Error())
		}
	} else {
		_, isBlock := req.GetVolumeCapability().GetAccessType().(*csi.VolumeCapability_Block)
		if err := s.fsOps.PrepareAndPerformMount(srcPath, dstPath, isBlock, !isBlock, mountOptions...); err != nil {
			ll.Errorf("Unable to mount volume: %v", err)
			newStatus = apiV1.Failed
			resp, errToReturn = nil, fmt.Errorf("failed to publish volume: mount error %s", err.Error())
		}
	}

	var podName string
	podName, ok := req.VolumeContext[util.PodNameKey]
	if !ok {
		podName = UnknownPodName
		ll.Warnf("flag podInfoOnMount isn't provided will add %s for volume owners", podName)
	}

	owners := volumeCR.Spec.Owners
	if !util.ContainsString(owners, podName) { // check whether podName already in owners or no
		owners = append(owners, podName)
		volumeCR.Spec.Owners = owners
	}

	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, volumeID)
	volumeCR.Spec.CSIStatus = newStatus
	if err = s.k8sClient.UpdateCR(ctxWithID, volumeCR); err != nil {
		ll.Errorf("Unable to update volume CR to %v, error: %v", volumeCR, err)
		resp, errToReturn = nil, fmt.Errorf("failed to publish volume: update volume CR error")
	}
	return resp, errToReturn
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

	ctxWithID := context.WithValue(ctx, base.RequestUUID, req.GetVolumeId())
	if err := s.fsOps.UnmountWithCheck(req.GetTargetPath()); err != nil {
		ll.Errorf("Unable to unmount volume: %v", err)
		volumeCR.Spec.CSIStatus = apiV1.Failed
		if updateErr := s.k8sClient.UpdateCR(ctxWithID, volumeCR); updateErr != nil {
			ll.Errorf("Unable to set volume CR status to failed: %v", updateErr)
		}
		return nil, status.Error(codes.Internal, "unmount error")
	}

	// support volume sharing by multiple pods, here we need only remove the owner from volume owners list
	// set volume state to VolumeReady only if Volume Owners is EMPTY
	var pod corev1.Pod
	// initialize it to avoid lint error: considering preallocating (prealloc)
	owners := []string{}
	for _, owner := range volumeCR.Spec.Owners {
		if err := s.k8sClient.ReadCR(ctx, owner, volumeCR.Namespace, &pod); err != nil {
			// CSI SPEC natively does not contain pod information in NodeUnpubishVolume request
			// We use a pod owner filter to find it. But in case no pod found due to some unexpected
			// behavior should not block NodeUnpublishVolume, we should cotinue to finish volume unpublish
			// Here we just log an error
			ll.Errorf("Unable to read volume owner pod information: %s, %v", owner, err)
			continue
		}
		ss := strings.Split(req.GetTargetPath(), "/")
		var found bool
		for _, s := range ss {
			if string(pod.UID) == s {
				found = true
			}
		}
		if found {
			continue
		}
		owners = append(owners, pod.Name)
	}

	volumeCR.Spec.Owners = owners
	if len(volumeCR.Spec.Owners) == 0 {
		volumeCR.Spec.CSIStatus = apiV1.VolumeReady
	}
	if updateErr := s.k8sClient.UpdateCR(ctxWithID, volumeCR); updateErr != nil {
		ll.Errorf("Unable to set volume CR status to VolumeReady: %v", updateErr)
		return nil, status.Error(codes.Internal, updateErr.Error())
	}

	ll.Debugf("Unpublished successfully")
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetVolumeStats returns empty response
func (s *CSINodeService) NodeGetVolumeStats(_ context.Context, _ *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return &csi.NodeGetVolumeStatsResponse{}, nil
}

// NodeExpandVolume returns empty response
func (s *CSINodeService) NodeExpandVolume(_ context.Context, _ *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

// NodeGetCapabilities is the implementation of CSI Spec NodeGetCapabilities.
// Provides Node capabilities of CSI driver to k8s. STAGE/UNSTAGE Volume for now.
// Receives golang context and CSI Spec NodeGetCapabilitiesRequest
// Returns CSI Spec NodeGetCapabilitiesResponse and nil error
func (s *CSINodeService) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
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
// Returns CSI Spec NodeGetInfoResponse with topology NodeIDTopologyLabelKey: NodeID and nil error
func (s *CSINodeService) NodeGetInfo(_ context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method": "NodeGetInfo",
	})

	topology := csi.Topology{
		Segments: map[string]string{
			csibmnodeconst.NodeIDTopologyLabelKey: s.nodeID,
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
	// parameters aren't used
	_, _ = ctx, req
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
func (s *CSINodeService) Watch(_ *grpc_health_v1.HealthCheckRequest, _ grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

// GetLivenessHelper return instance of livenesshelper used by node service
func (s *CSINodeService) GetLivenessHelper() LivenessHelper {
	return s.livenessCheck
}
