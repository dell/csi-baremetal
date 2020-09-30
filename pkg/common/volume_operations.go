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
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// VolumeOperations is the interface that unites common Volume CRs operations. It is designed for inline volume support
// without code duplication
type VolumeOperations interface {
	CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string)
	WaitStatus(ctx context.Context, volumeID string, statuses ...string) error
}

// VolumeOperationsImpl is the basic implementation of VolumeOperations interface
type VolumeOperationsImpl struct {
	acProvider AvailableCapacityOperations
	k8sClient  *k8s.KubeClient

	log *logrus.Entry
}

// NewVolumeOperationsImpl is the constructor for VolumeOperationsImpl struct
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of VolumeOperationsImpl
func NewVolumeOperationsImpl(k8sClient *k8s.KubeClient, logger *logrus.Logger) *VolumeOperationsImpl {
	return &VolumeOperationsImpl{
		k8sClient:  k8sClient,
		acProvider: NewACOperationsImpl(k8sClient, logger),
		log:        logger.WithField("component", "VolumeOperationsImpl"),
	}
}

// CreateVolume searches AC and creates volume CR or returns existed volume CR
// Receives golang context and api.Volume which is Spec of Volume CR to create
// Returns api.Volume instance that took the place of chosen by SearchAC method AvailableCapacity CR
func (vo *VolumeOperationsImpl) CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error) {
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "CreateVolume",
		"volumeID": v.Id,
	})
	ll.Infof("Creating volume %v", v)

	var (
		ctxWithID = context.WithValue(context.Background(), k8s.RequestUUID, v.Id)
		volumeCR  = &volumecrd.Volume{}
		err       error
	)
	// at first check whether volume CR exist or no
	err = vo.k8sClient.ReadCR(ctx, v.Id, volumeCR)
	switch {
	case err == nil:
		ll.Infof("Volume exists, current status: %s.", volumeCR.Spec.CSIStatus)
		if volumeCR.Spec.CSIStatus == apiV1.Failed {
			return nil, fmt.Errorf("corresponding volume CR %s has failed status", volumeCR.Spec.Id)
		}
		// check that volume is in created state or time is over (for creating)
		expiredAt := volumeCR.ObjectMeta.GetCreationTimestamp().Add(base.DefaultTimeoutForVolumeOperations)
		if expiredAt.Before(time.Now()) {
			ll.Errorf("Timeout of %s for volume creation exceeded.", base.DefaultTimeoutForVolumeOperations)
			volumeCR.Spec.CSIStatus = apiV1.Failed
			_ = vo.k8sClient.UpdateCRWithAttempts(ctxWithID, volumeCR, 5)
			return nil, status.Error(codes.Internal, "Unable to create volume in allocated time")
		}
	case !k8sError.IsNotFound(err):
		ll.Errorf("Unable to read volume CR: %v", err)
		return nil, status.Error(codes.Aborted, "unable to check volume existence")
	default:
		// create volume
		var (
			ac             *accrd.AvailableCapacity
			sc             string
			requiredBytes  = v.Size
			allocatedBytes int64
			locationType   string
			csiStatus      = apiV1.Creating
		)

		if util.IsStorageClassLVG(sc) {
			requiredBytes = AlignSizeByPE(requiredBytes)
		}

		ac = vo.acProvider.SearchAC(ctxWithID, v.NodeId, requiredBytes, v.StorageClass)
		if ac == nil {
			if util.IsStorageClassLVG(v.StorageClass) {
				subSC := util.GetSubStorageClass(v.StorageClass)
				// requiredBytes increase wanted size for additional costs
				ac = vo.acProvider.SearchAC(ctxWithID, v.NodeId, requiredBytes+LvgDefaultMetadataSize, subSC) // search volume for LVG in subSC
			}
			if ac == nil {
				ll.Errorf("There is no suitable drive for volume with sc %s.", sc)
				return nil, status.Errorf(codes.ResourceExhausted, "there is no suitable drive for volume %s", v.Id)
			}
			// here we get AC that should be recreated to LVG, LVG hasn't existed yet
			if ac = vo.acProvider.RecreateACToLVGSC(ctxWithID, v.StorageClass, *ac); ac == nil {
				return nil, status.Errorf(codes.Internal, "unable to prepare underlying storage for storage class %s", v.StorageClass)
			}
		}
		ll.Infof("AC %v was selected", ac.Spec)

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
			Mode:              v.Mode,
			Type:              v.Type,
		}
		volumeCR = vo.k8sClient.ConstructVolumeCR(v.Id, apiVolume)

		if err = vo.k8sClient.CreateCR(ctxWithID, v.Id, volumeCR); err != nil {
			ll.Errorf("Unable to create CR, error: %v", err)
			return nil, status.Errorf(codes.Internal, "unable to create volume CR")
		}

		// decrease AC size
		ac.Spec.Size -= allocatedBytes
		if err = vo.k8sClient.UpdateCRWithAttempts(ctxWithID, ac, 5); err != nil {
			ll.Errorf("Unable to set size for AC %s to %d, error: %v", ac.Name, ac.Spec.Size, err)
		}
	}
	return &volumeCR.Spec, nil
}

// DeleteVolume changes volume CR state and updates it,
// if volume CR doesn't exists return Not found error and that error should be handled by caller.
// Receives golang context and a volume ID to delete
// Returns error if something went wrong or Volume with volumeID wasn't found
func (vo *VolumeOperationsImpl) DeleteVolume(ctx context.Context, volumeID string) error {
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "DeleteVolume",
		"volumeID": volumeID,
	})
	ll.Info("Processing")

	var (
		volumeCR = &volumecrd.Volume{}
		err      error
	)

	if err = vo.k8sClient.ReadCR(ctx, volumeID, volumeCR); err != nil {
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
// remove Volume CR and if volume was in LVG SC - update corresponding AC CR
// does not return anything because that method does not change real drive on the node
func (vo *VolumeOperationsImpl) UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string) {
	ll := vo.log.WithFields(logrus.Fields{
		"method":   "UpdateCRsAfterVolumeDeletion",
		"volumeID": ctx.Value(k8s.RequestUUID),
	})

	var (
		volumeCR = volumecrd.Volume{}
		err      error
	)

	if err = vo.k8sClient.ReadCR(ctx, volumeID, &volumeCR); err != nil {
		if !k8sError.IsNotFound(err) {
			ll.Errorf("Unable to read volume CR %s: %v. Volume CR will not be removed", volumeID, err)
		}
		return
	}

	if err = vo.k8sClient.DeleteCR(ctx, &volumeCR); err != nil {
		ll.Errorf("unable to delete volume CR %s: %v", volumeID, err)
	}

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

	// for LVG SCs we need to delete AC CR when no volumes remain to avoid new allocations since
	// underlying LVG CR is destroying. For other SC just to increase size
	isDeleted := false
	lvg := &lvgcrd.LVG{}
	if util.IsStorageClassLVG(volumeCR.Spec.StorageClass) {
		if err = vo.k8sClient.ReadCR(context.Background(), volumeCR.Spec.Location, lvg); err != nil {
			ll.Errorf("Unable to get LVG %s: %v", volumeCR.Spec.Location, err)
			return
		}

		if isDeleted, err = vo.deleteLVGIfVolumesNotExistOrUpdate(lvg, volumeCR.Name, &acCR); err != nil {
			ll.Errorf("Unable to remove volume reference from LVG %s: %v", volumeCR.Spec.Location, err)
		}
	}

	// if LVG wasn't deleted increase AC size
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
	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done but volume still not reach one of the expected status: %v", statuses)
			return fmt.Errorf("volume context is done")
		case <-time.After(timeoutBetweenCheck):
			if err = vo.k8sClient.ReadCR(ctx, volumeID, v); err != nil {
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

// deleteLVGIfVolumesNotExistOrUpdate tries to remove volume ID into VolumeRefs slice from LVG struct
// and updates according LVG
// If VolumeRefs length equals 0, then deletes according AC and LVG
// Receives LVG and volumeID of a Volume CR which should be removed
// Returns true if LVG CR was deleted and false otherwise, error if something went wrong
func (vo *VolumeOperationsImpl) deleteLVGIfVolumesNotExistOrUpdate(lvg *lvgcrd.LVG,
	volID string, ac *accrd.AvailableCapacity) (bool, error) {
	log := vo.log.WithFields(logrus.Fields{
		"method":   "deleteLVGIfVolumesNotExistOrUpdate",
		"volumeID": volID,
	})

	driveUUID := append(vo.k8sClient.GetSystemDriveUUID(context.Background(), lvg.Spec.Node), base.SystemDriveAsLocation)
	// if only one volume remains - remove AC first and LVG then
	if len(lvg.Spec.VolumeRefs) == 1 && !util.IsDriveSystem(lvg.Spec.Locations[0], driveUUID) {
		if err := vo.k8sClient.DeleteCR(context.Background(), ac); err != nil {
			log.Errorf("Unable to delete AC %s: %v", ac.Name, err)
			return false, err
		}
		return true, vo.k8sClient.DeleteCR(context.Background(), lvg)
	}

	// search for volume index
	for i, id := range lvg.Spec.VolumeRefs {
		if volID == id {
			log.Debugf("Remove volume %s from LVG %v", volID, lvg)
			l := len(lvg.Spec.VolumeRefs)
			lvg.Spec.VolumeRefs[i] = lvg.Spec.VolumeRefs[l-1]
			lvg.Spec.VolumeRefs = lvg.Spec.VolumeRefs[:l-1]

			return false, vo.k8sClient.UpdateCR(context.Background(), lvg)
		}
	}

	log.Errorf("Reference to volume %s in LVG %v not found", volID, lvg)
	return false, errors.New("LVG CR wasn't updated")
}
