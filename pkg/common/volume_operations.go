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

// Package common is for common operations with CSI resources such as AvailableCapacity or Volume
package common

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/cache"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	fc "github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// VolumeOperations is the interface that unites common Volume CRs operations. It is designed for inline volume support
// without code duplication
type VolumeOperations interface {
	CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string)
	WaitStatus(ctx context.Context, volumeID string, statuses ...string) error
	ExpandVolume(ctx context.Context, volume *volumecrd.Volume, requiredBytes int64) error
	UpdateCRsAfterVolumeExpansion(ctx context.Context, volID string, requiredBytes int64)
}

// VolumeOperationsImpl is the basic implementation of VolumeOperations interface
type VolumeOperationsImpl struct {
	acProvider             AvailableCapacityOperations
	k8sClient              *k8s.KubeClient
	capacityManagerBuilder capacityplanner.CapacityManagerBuilder
	crHelper               *k8s.CRHelper

	metrics        metrics.Statistic
	cache          cache.Interface
	featureChecker fc.FeatureChecker
	log            *logrus.Entry
}

// NewVolumeOperationsImpl is the constructor for VolumeOperationsImpl struct
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of VolumeOperationsImpl
func NewVolumeOperationsImpl(k8sClient *k8s.KubeClient, logger *logrus.Logger, cache cache.Interface,
	featureConf fc.FeatureChecker) *VolumeOperationsImpl {
	volumeMetrics := metrics.NewMetrics(prometheus.HistogramOpts{
		Name:    "volume_operations_duration",
		Help:    "Volume operations methods duration",
		Buckets: metrics.ExtendedDefBuckets,
	}, "method")
	if err := prometheus.Register(volumeMetrics.Collect()); err != nil {
		logger.WithField("component", "NewVolumeOperationsImpl").
			Errorf("Failed to register metric: %v", err)
	}
	vo := &VolumeOperationsImpl{
		k8sClient:              k8sClient,
		crHelper:               k8s.NewCRHelper(k8sClient, logger),
		acProvider:             NewACOperationsImpl(k8sClient, logger),
		log:                    logger.WithField("component", "VolumeOperationsImpl"),
		featureChecker:         featureConf,
		capacityManagerBuilder: &capacityplanner.DefaultCapacityManagerBuilder{},
		cache:                  cache,
		metrics:                volumeMetrics,
	}
	vo.fillCache()
	return vo
}

// CreateVolume searches AC and creates volume CR or returns existed volume CR
// Receives golang context and api.Volume which is Spec of Volume CR to create
// Returns api.Volume instance that took the place of chosen by SearchAC method AvailableCapacity CR
func (vo *VolumeOperationsImpl) CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error) {
	defer vo.metrics.EvaluateDurationForMethod("CreateVolume")()
	log := vo.log.WithFields(logrus.Fields{
		"method":   "CreateVolume",
		"volumeID": v.Id,
	})
	log.Infof("Creating volume %v", v)

	var (
		ctxWithID = context.WithValue(context.Background(), base.RequestUUID, v.Id)
		volumeCR  = &volumecrd.Volume{}
		err       error
		namespace = base.DefaultNamespace
	)

	// at first check whether volume CR exist or no
	err = vo.k8sClient.ReadCR(ctx, v.Id, namespace, volumeCR)
	switch {
	case err == nil:
		log.Infof("Volume exists, current status: %s.", volumeCR.Spec.CSIStatus)
		if volumeCR.Spec.CSIStatus == apiV1.Failed {
			return nil, fmt.Errorf("corresponding volume CR %s has failed status", volumeCR.Spec.Id)
		}
		// check that volume is in created state or time is over (for creating)
		expiredAt := volumeCR.ObjectMeta.GetCreationTimestamp().Add(base.DefaultTimeoutForVolumeOperations)
		if expiredAt.Before(time.Now()) {
			log.Errorf("Timeout of %s for volume creation exceeded.", base.DefaultTimeoutForVolumeOperations)
			volumeCR.Spec.CSIStatus = apiV1.Failed
			_ = vo.k8sClient.UpdateCRWithAttempts(ctxWithID, volumeCR, 5)
			return nil, status.Error(codes.Internal, "Unable to create volume in allocated time")
		}
		return &volumeCR.Spec, nil
	case !k8sError.IsNotFound(err):
		log.Errorf("Unable to read volume CR: %v", err)
		return nil, status.Error(codes.Aborted, "unable to check volume existence")
	}

	// read information about PersistentVolumeClaim from context value
	var volumeInfo *util.VolumeInfo
	if value := ctx.Value(util.VolumeInfoKey); value != nil {
		volumeInfo = value.(*util.VolumeInfo)
		if !volumeInfo.IsDefaultNamespace() {
			namespace = volumeInfo.Namespace
		}
	}

	if volumeInfo == nil {
		log.Errorf("Volume info is not passed for %s", v.Id)
		return nil, fmt.Errorf("volume info is not passed for %s", v.Id)
	}
	// create volume
	return vo.handleVolumeCreation(ctxWithID, log, v, namespace, volumeInfo.Name)
}

// TODO - refactor this method https://github.com/dell/csi-baremetal/issues/371
func (vo *VolumeOperationsImpl) handleVolumeCreation(ctx context.Context, log *logrus.Entry, v api.Volume,
	podNamespace string, reservationName string) (*api.Volume, error) {
	var (
		sc             string
		requiredBytes  = v.Size
		allocatedBytes int64
		locationType   string
		csiStatus      = apiV1.Creating
	)

	if util.IsStorageClassLVG(sc) {
		requiredBytes = capacityplanner.AlignSizeByPE(requiredBytes)
	}

	resReader := capacityplanner.NewACRReader(vo.k8sClient, vo.log, true)
	reservations, err := resReader.ReadReservations(ctx)
	if err != nil {
		return nil, err
	}

	var podReservation *acrcrd.AvailableCapacityReservation
	var requestNum int
	for _, reservation := range reservations {
		reservation := reservation
		if reservation.Spec.Status != apiV1.ReservationConfirmed {
			continue
		}

		if reservation.Spec.Namespace == podNamespace {
			for i, request := range reservation.Spec.ReservationRequests {
				name := request.CapacityRequest.Name
				if name == reservationName {
					podReservation = &reservation
					requestNum = i
					break
				}
			}
		}
		// exit if found
		if podReservation != nil {
			log.Debugf("Reservation for volume %s found: %v", v.Id, podReservation)
			break
		}
	}

	if podReservation == nil {
		return nil, status.Error(codes.ResourceExhausted, fmt.Sprintf("Reservation for volume %s not found", v.Id))
	}

	isFound := false
	ac := &accrd.AvailableCapacity{}
	for _, capacityName := range podReservation.Spec.ReservationRequests[requestNum].Reservations {
		// read available capacity
		err = vo.k8sClient.ReadCR(ctx, capacityName, "", ac)
		if err != nil {
			log.Errorf("Failed to read capacity %s: %v", capacityName, err)
			return nil, err
		}

		if ac.Spec.NodeId == v.NodeId {
			isFound = true
			break
		}
	}

	if !isFound {
		return nil, status.Error(codes.ResourceExhausted, fmt.Sprintf("there is no suitable drive for volume %s", v.Id))
	}

	if ac.Spec.StorageClass != v.StorageClass && util.IsStorageClassLVG(v.StorageClass) {
		// AC needs to be converted to LogicalVolumeGroup AC, LogicalVolumeGroup doesn't exist yet
		if ac = vo.acProvider.RecreateACToLVGSC(ctx, v.StorageClass, *ac); ac == nil {
			return nil, status.Errorf(codes.Internal,
				"unable to prepare underlying storage for storage class %s", v.StorageClass)
		}
	}
	log.Infof("AC %v was selected", ac)

	// if sc was parsed as an ANY then we can choose AC with any storage class and then
	// volume should be created with that particular SC
	sc = ac.Spec.StorageClass

	if util.IsStorageClassLVG(sc) {
		allocatedBytes = requiredBytes
		locationType = apiV1.LocationTypeLVM
	} else {
		allocatedBytes = ac.Spec.Size
		locationType = apiV1.LocationTypeDrive
	}

	// create volume CR
	apiVolume := api.Volume{
		Id:                v.Id,
		NodeId:            ac.Spec.NodeId,
		Size:              allocatedBytes,
		Location:          ac.Spec.Location,
		CSIStatus:         csiStatus,
		StorageClass:      sc,
		Ephemeral:         v.Ephemeral,
		Health:            apiV1.HealthGood,
		LocationType:      locationType,
		OperationalStatus: apiV1.OperationalStatusOperative,
		Usage:             apiV1.VolumeUsageInUse,
		Mode:              v.Mode,
		Type:              v.Type,
	}
	volumeCR := vo.k8sClient.ConstructVolumeCR(v.Id, podNamespace, apiVolume)

	if err = vo.k8sClient.CreateCR(ctx, v.Id, volumeCR); err != nil {
		log.Errorf("Unable to create CR, error: %v", err)
		return nil, status.Errorf(codes.Internal, "unable to create volume CR")
	}
	vo.cache.Set(v.Id, podNamespace)

	// decrease AC size
	ac.Spec.Size -= allocatedBytes
	if err = vo.k8sClient.UpdateCRWithAttempts(ctx, ac, 5); err != nil {
		log.Errorf("Unable to set size for AC %s to %d, error: %v", ac.Name, ac.Spec.Size, err)
	}
	if vo.featureChecker.IsEnabled(fc.FeatureACReservation) {
		capReader := capacityplanner.NewACReader(vo.k8sClient, vo.log, true)
		resHelper := capacityplanner.NewReservationHelper(vo.log, vo.k8sClient, capReader, resReader)

		if err := resHelper.ReleaseReservation(ctx, podReservation, requestNum); err != nil {
			return nil, err
		}
	}

	return &volumeCR.Spec, nil
}

// DeleteVolume changes volume CR state and updates it,
// if volume CR doesn't exists return Not found error and that error should be handled by caller.
// Receives golang context and a volume ID to delete
// Returns error if something went wrong or Volume with volumeID wasn't found
func (vo *VolumeOperationsImpl) DeleteVolume(ctx context.Context, volumeID string) error {
	defer vo.metrics.EvaluateDurationForMethod("DeleteVolume")()
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "DeleteVolume",
		"volumeID": volumeID,
	})
	ll.Info("Processing")

	var (
		volumeCR = &volumecrd.Volume{}
		err      error
	)

	namespace, err := vo.cache.Get(volumeID)
	if err != nil {
		ll.Errorf("Unable to get volume namespace, volume doesn't exists: %v", err)
		return status.Errorf(codes.NotFound, "volume doesn't exists in cache")
	}
	if err = vo.k8sClient.ReadCR(ctx, volumeID, namespace, volumeCR); err != nil {
		return err
	}

	if !volumeCR.Spec.Ephemeral {
		switch volumeCR.Spec.CSIStatus {
		case apiV1.Created:
		case apiV1.Failed:
			return status.Error(codes.Internal, "volume has reached failed status")
		case apiV1.Removed:
			ll.Debug("Volume has Removed status")
			return nil
		case apiV1.Removing:
			ll.Debug("Volume has Removing status")
			return nil
		default:
			return status.Errorf(codes.FailedPrecondition,
				"Volume CR status hadn't been set to %s, current status - %s, expected - %s",
				apiV1.Removing, volumeCR.Spec.CSIStatus, apiV1.Created)
		}
	} else if volumeCR.Spec.CSIStatus != apiV1.Published { // expect Published status for ephemeral volume
		return status.Errorf(codes.FailedPrecondition,
			"CSIStatus for ephemeral volume hadn't been set to %s, current status - %s, expected - %s",
			apiV1.Removing, volumeCR.Spec.CSIStatus, apiV1.Published)
	}

	volumeCR.Spec.CSIStatus = apiV1.Removing
	return vo.k8sClient.UpdateCR(ctx, volumeCR)
}

// UpdateCRsAfterVolumeDeletion should considered as a second step in DeleteVolume,
// remove Volume CR and if volume was in LogicalVolumeGroup SC - update corresponding AC CR
// does not return anything because that method does not change real drive on the node
func (vo *VolumeOperationsImpl) UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string) {
	defer vo.metrics.EvaluateDurationForMethod("UpdateCRsAfterVolumeDeletion")()
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "UpdateCRsAfterVolumeDeletion",
		"volumeID": ctx.Value(base.RequestUUID),
	})

	var (
		volumeCR = volumecrd.Volume{}
		err      error
	)

	namespace, err := vo.cache.Get(volumeID)
	if err != nil {
		ll.Errorf("Unable to get volume namespace: %v", err)
		return
	}

	if err = vo.k8sClient.ReadCR(ctx, volumeID, namespace, &volumeCR); err != nil {
		if !k8sError.IsNotFound(err) {
			ll.Errorf("Unable to read volume CR %s: %v. Volume CR will not be removed", volumeID, err)
		}
		return
	}

	if err = vo.k8sClient.DeleteCR(ctx, &volumeCR); err != nil {
		ll.Errorf("unable to delete volume CR %s: %v", volumeID, err)
	}

	vo.cache.Delete(volumeID)
	// find corresponding AC CR
	acList := accrd.AvailableCapacityList{}
	if err = vo.k8sClient.ReadList(ctx, &acList); err != nil {
		ll.Errorf("Volume was deleted but corresponding AC with SC %s hadn't updated, unable to read list: %v",
			volumeCR.Spec.StorageClass, err)
	}

	// search for AC
	acCR := accrd.AvailableCapacity{}
	for _, a := range acList.Items {
		if a.Spec.Location == volumeCR.Spec.Location {
			acCR = a
			break
		}
	}
	// AC CR must exist
	if acCR.Name == "" {
		ll.Errorf("Unable to find available capacity resource for volume %s", volumeID)
		return
	}

	// for LogicalVolumeGroup SCs we need to delete AC CR when no volumes remain to avoid new allocations since
	// underlying LogicalVolumeGroup CR is destroying. For other SC just to increase size
	isDeleted := false
	lvg := &lvgcrd.LogicalVolumeGroup{}
	if util.IsStorageClassLVG(volumeCR.Spec.StorageClass) {
		if err = vo.k8sClient.ReadCR(context.Background(), volumeCR.Spec.Location, "", lvg); err != nil {
			ll.Errorf("Unable to get LogicalVolumeGroup %s: %v", volumeCR.Spec.Location, err)
			return
		}

		if isDeleted, err = vo.deleteLVGIfVolumesNotExistOrUpdate(lvg, volumeCR.Name, &acCR); err != nil {
			ll.Errorf("Unable to remove volume reference from LogicalVolumeGroup %s: %v", volumeCR.Spec.Location, err)
		}
	}

	// if LogicalVolumeGroup wasn't deleted increase AC size
	if !isDeleted {
		// Increase size of AC using volume size
		acCR.Spec.Size += volumeCR.Spec.Size
		if err = vo.k8sClient.UpdateCRWithAttempts(ctx, &acCR, 5); err != nil {
			ll.Errorf("Unable to update AC %s size: %v", acCR.Name, err)
		}
	}
}

// WaitStatus check volume status until it will be reached one of the statuses
// return error if context is done or volume reaches failed status, return nil if reached status != failed
func (vo *VolumeOperationsImpl) WaitStatus(ctx context.Context, volumeID string, statuses ...string) error {
	defer vo.metrics.EvaluateDurationForMethod("WaitStatus")()
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "WaitStatus",
		"volumeID": volumeID,
	})

	ll.Infof("Pulling volume status")

	var (
		v                   = &volumecrd.Volume{}
		timeoutBetweenCheck = time.Second
		err                 error
	)
	namespace, err := vo.cache.Get(volumeID)
	if err != nil {
		ll.Errorf("Unable to get volume namespace: %v", err)
		return fmt.Errorf("unable to get volume namespace")
	}
	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done but volume still not reach one of the expected status: %v", statuses)
			return fmt.Errorf("volume context is done")
		case <-time.After(timeoutBetweenCheck):
			if err = vo.k8sClient.ReadCR(ctx, volumeID, namespace, v); err != nil {
				ll.Errorf("Unable to read volume CR: %v", err)
				if k8sError.IsNotFound(err) {
					ll.Error("Volume CR doesn't exist")
					return fmt.Errorf("volume isn't found")
				}
				continue
			}
			for _, s := range statuses {
				if v.Spec.CSIStatus == s {
					if s == apiV1.Failed {
						return fmt.Errorf("volume has reached Failed status")
					}
					return nil
				}
			}
		}
	}
}

// ExpandVolume updates Volume status to Resizing to trigger expansion in reconcile, if volume has already had status
// Resizing or Resized, function doesn't do anything. In case of statuses beside VolumeReady, Created, Published function return error
// Receive golang context, volume CR, requiredBytes as int
// Return volume spec, error
func (vo *VolumeOperationsImpl) ExpandVolume(ctx context.Context, volume *volumecrd.Volume, requiredBytes int64) error {
	defer vo.metrics.EvaluateDurationForMethod("ExpandVolume")()
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "ExpandVolume",
		"volumeID": volume.Spec.Id,
	})
	currStatus := volume.Spec.CSIStatus
	switch currStatus {
	case apiV1.Resizing, apiV1.Resized:
		ll.Debug("Volume is already expanding")
	case apiV1.VolumeReady, apiV1.Created, apiV1.Published:
		if !util.IsStorageClassLVG(volume.Spec.StorageClass) {
			return status.Error(codes.FailedPrecondition,
				fmt.Sprintf("StorageClass %s doesn't support resizing", volume.Spec.StorageClass))
		}
		capacity, err := vo.crHelper.GetACByLocation(volume.Spec.Location)
		if err != nil {
			ll.Errorf("Failed to get AC by location %s", volume.Spec.Location)
			return status.Error(codes.Internal, "Unable to read AC")
		}

		acSize := requiredBytes - volume.Spec.Size
		if capacity.Spec.Size < acSize {
			return status.Error(codes.OutOfRange,
				fmt.Sprintf("Not enough capacity to expand volume: requested - %d, available - %d", requiredBytes, capacity.Spec.Size))
		}
		capacity.Spec.Size -= acSize
		if err := vo.k8sClient.UpdateCRWithAttempts(ctx, capacity, 5); err != nil {
			ll.Errorf("Failed to update AC, error: %v", err)
			return status.Error(codes.Internal, "Unable to reserve AC")
		}

		if volume.Annotations == nil {
			volume.Annotations = make(map[string]string)
		}
		volume.Annotations[apiV1.VolumePreviousStatus] = currStatus
		volume.Annotations[apiV1.VolumePreviousCapacity] = strconv.FormatInt(volume.Spec.Size, 10)
		volume.Spec.CSIStatus = apiV1.Resizing
		volume.Spec.Size = requiredBytes

		if err := vo.k8sClient.UpdateCRWithAttempts(ctx, volume, 5); err != nil {
			ll.Errorf("Failed to update volume, error: %v", err)
			return status.Error(codes.Internal, "Unable to update volume")
		}
	default:
		return status.Errorf(codes.FailedPrecondition, "Volume in status %s can't be expanded", volume.Spec.CSIStatus)
	}
	return nil
}

// UpdateCRsAfterVolumeExpansion update volume and AC crs after volume expansion
// Receive golang context, volume spec
// Return error
func (vo *VolumeOperationsImpl) UpdateCRsAfterVolumeExpansion(ctx context.Context, volID string, requiredBytes int64) {
	defer vo.metrics.EvaluateDurationForMethod("ExpandVolume")()
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "UpdateCRsAfterVolumeExpansion",
		"volumeID": volID,
	})
	var (
		namespace string
		err       error
		volume    = &volumecrd.Volume{}
	)
	namespace, err = vo.cache.Get(volID)
	if err != nil {
		ll.Errorf("Failed to get namespace from cache, error: %v", err)
		return
	}
	if err = vo.k8sClient.ReadCR(ctx, volID, namespace, volume); err != nil {
		ll.Errorf("Failed to read volume: %v", err)
		return
	}
	switch volume.Spec.CSIStatus {
	case apiV1.Failed:
		capacity, err := strconv.ParseInt(volume.Annotations[apiV1.VolumePreviousCapacity], 10, 64)
		if err != nil {
			ll.Errorf("Failed to convert volume annotation %s to int, error : %v", apiV1.VolumePreviousCapacity, err)
			return
		}
		volume.Spec.Size = capacity
		ac, err := vo.crHelper.GetACByLocation(volume.Spec.Location)
		if err != nil {
			ll.Errorf("Failed to read AC: %v", err)
		} else {
			acSize := requiredBytes - volume.Spec.Size
			ac.Spec.Size += acSize
			if err = vo.k8sClient.UpdateCRWithAttempts(ctx, ac, 5); err != nil {
				ll.Errorf("Failed to update AC: %v", err)
			}
		}
	case apiV1.Resized:
		if _, ok := volume.Annotations[apiV1.VolumePreviousStatus]; !ok {
			ll.Errorf("Failed to set previous status, annotation %s wasn't found for volume %s", apiV1.VolumePreviousStatus, volID)
		} else {
			volume.Spec.CSIStatus = volume.Annotations[apiV1.VolumePreviousStatus]
		}
	default:
		ll.Warnf("Volume status is %s, expected %s or %s", volume.Spec.CSIStatus, apiV1.Resized, apiV1.Failed)
		return
	}
	delete(volume.Annotations, apiV1.VolumePreviousCapacity)
	delete(volume.Annotations, apiV1.VolumePreviousStatus)
	if updateErr := vo.k8sClient.UpdateCR(ctx, volume); updateErr != nil {
		ll.Error("Unable to set new status for volume")
	}
}

// deleteLVGIfVolumesNotExistOrUpdate tries to remove volume ID into VolumeRefs slice from LogicalVolumeGroup struct
// and updates according LogicalVolumeGroup
// If VolumeRefs length equals 0, then deletes according AC and LogicalVolumeGroup
// Receives LogicalVolumeGroup and volumeID of a Volume CR which should be removed
// Returns true if LogicalVolumeGroup CR was deleted and false otherwise, error if something went wrong
func (vo *VolumeOperationsImpl) deleteLVGIfVolumesNotExistOrUpdate(lvg *lvgcrd.LogicalVolumeGroup,
	volID string, ac *accrd.AvailableCapacity) (bool, error) {
	log := vo.log.WithFields(logrus.Fields{
		"method":   "deleteLVGIfVolumesNotExistOrUpdate",
		"volumeID": volID,
	})

	drivesUUIDs := vo.k8sClient.GetSystemDriveUUIDs()
	// if only one volume remains - remove AC first and LogicalVolumeGroup then
	if len(lvg.Spec.VolumeRefs) == 1 && !util.ContainsString(drivesUUIDs, lvg.Spec.Locations[0]) {
		if err := vo.k8sClient.DeleteCR(context.Background(), ac); err != nil {
			log.Errorf("Unable to delete AC %s: %v", ac.Name, err)
			return false, err
		}
		return true, vo.k8sClient.DeleteCR(context.Background(), lvg)
	}

	// search for volume index
	for i, id := range lvg.Spec.VolumeRefs {
		if volID == id {
			log.Debugf("Remove volume %s from LogicalVolumeGroup %v", volID, lvg)
			l := len(lvg.Spec.VolumeRefs)
			lvg.Spec.VolumeRefs[i] = lvg.Spec.VolumeRefs[l-1]
			lvg.Spec.VolumeRefs = lvg.Spec.VolumeRefs[:l-1]

			return false, vo.k8sClient.UpdateCR(context.Background(), lvg)
		}
	}

	log.Errorf("Reference to volume %s in LogicalVolumeGroup %v not found", volID, lvg)
	return false, errors.New("lvg CR wasn't updated")
}

// fillCache tries to fill volume/namespace cache after VolumeOperationsImpl initialization
func (vo *VolumeOperationsImpl) fillCache() {
	ll := vo.log.WithFields(logrus.Fields{
		"method": "fillCache",
	})
	volList := &volumecrd.VolumeList{}
	if err := vo.k8sClient.ReadList(context.Background(), volList); err != nil {
		ll.Errorf("Failed to fill volume cache, error %v", err)
	}
	for _, volume := range volList.Items {
		vo.cache.Set(volume.Name, volume.Namespace)
	}
}
