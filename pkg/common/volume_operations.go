// Package common is for common operations with CSI resources such as AvailableCapacity or Volume
package common

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

// VolumeOperations is the interface that unites common Volume CRs operations. It is designed for inline volume support
// without code duplication
type VolumeOperations interface {
	CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string)
	WaitStatus(ctx context.Context, volumeID string, statuses ...string) error
	ReadVolumeAndChangeStatus(volumeID string, newStatus string) error
}

// VolumeOperationsImpl is the basic implementation of VolumeOperations interface
type VolumeOperationsImpl struct {
	acProvider AvailableCapacityOperations
	k8sClient  *base.KubeClient

	log *logrus.Entry
}

// NewVolumeOperationsImpl is the constructor for VolumeOperationsImpl struct
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of VolumeOperationsImpl
func NewVolumeOperationsImpl(k8sClient *base.KubeClient, logger *logrus.Logger) *VolumeOperationsImpl {
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
		volumeCR = &volumecrd.Volume{}
		err      error
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
		expiredAt := volumeCR.ObjectMeta.GetCreationTimestamp().Add(base.DefaultTimeoutForOperations)
		if expiredAt.Before(time.Now()) {
			ll.Errorf("Timeout of %s for volume creation exceeded.", base.DefaultTimeoutForOperations)
			volumeCR.Spec.CSIStatus = apiV1.Failed
			_ = vo.k8sClient.UpdateCRWithAttempts(ctx, volumeCR, 5)
			return nil, status.Error(codes.Internal, "Unable to create volume in allocated time")
		}
	case !k8sError.IsNotFound(err):
		ll.Errorf("Unable to read volume CR: %v", err)
		return nil, status.Error(codes.Aborted, "unable to check volume existence")
	default:
		// create volume
		var (
			ctxWithID      = context.WithValue(ctx, base.RequestUUID, v.Id)
			ac             *accrd.AvailableCapacity
			sc             string
			allocatedBytes int64
			locationType   string
		)

		if ac = vo.acProvider.SearchAC(ctxWithID, v.NodeId, v.Size, v.StorageClass); ac == nil {
			ll.Error("There is no suitable drive for volume")
			return nil, status.Errorf(codes.ResourceExhausted, "there is no suitable drive for request %s", v.Id)
		}
		ll.Infof("AC %v was selected.", ac.Spec)
		// if sc was parsed as an ANY then we can choose AC with any storage class and then
		// volume should be created with that particular SC
		sc = ac.Spec.StorageClass

		switch sc {
		case apiV1.StorageClassHDDLVG, apiV1.StorageClassSSDLVG:
			allocatedBytes = v.Size
			locationType = apiV1.LocationTypeLVM
		default:
			allocatedBytes = ac.Spec.Size
			locationType = apiV1.LocationTypeDrive
		}

		// create volume CR
		apiVolume := api.Volume{
			Id:                v.Id,
			NodeId:            ac.Spec.NodeId,
			Size:              allocatedBytes,
			Location:          ac.Spec.Location,
			CSIStatus:         apiV1.Creating,
			StorageClass:      sc,
			Ephemeral:         v.Ephemeral,
			Health:            apiV1.HealthGood,
			LocationType:      locationType,
			OperationalStatus: apiV1.OperationalStatusOperative,
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

	switch volumeCR.Spec.CSIStatus {
	case apiV1.Failed:
		return status.Error(codes.Internal, "volume has reached FailToRemove status")
	case apiV1.Removed, apiV1.Removing:
		return nil
	default:
		volumeCR.Spec.CSIStatus = apiV1.Removing
		return vo.k8sClient.UpdateCR(ctx, volumeCR)
	}
}

// UpdateCRsAfterVolumeDeletion should considered as a second step in DeleteVolume,
// remove Volume CR and if volume was in LVG SC - update corresponding AC CR
// does not return anything because that method does not change real drive on the node
func (vo *VolumeOperationsImpl) UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string) {
	ll := logrus.WithFields(logrus.Fields{
		"method":   "UpdateCRsAfterVolumeDeletion",
		"volumeID": ctx.Value(base.RequestUUID),
	})

	var (
		volumeCR = volumecrd.Volume{}
		err      error
	)

	if err = vo.k8sClient.ReadCR(ctx, volumeID, &volumeCR); err != nil {
		if !k8sError.IsNotFound(err) {
			ll.Errorf("Unable to read volume CR: %v. Volume CR will not be removed", volumeID)
		}
		return
	}

	// if volume is in LVG - update corresponding AC size
	// if such AC isn't exist - do nothing (AC should be recreated by VolumeMgr)
	if volumeCR.Spec.StorageClass == apiV1.StorageClassHDDLVG || volumeCR.Spec.StorageClass == apiV1.StorageClassSSDLVG {
		var (
			acCR   = accrd.AvailableCapacity{}
			acList = accrd.AvailableCapacityList{}
		)
		if err = vo.k8sClient.ReadList(ctx, &acList); err != nil {
			ll.Errorf("Volume was deleted but corresponding AC with SC %s hadn't updated, unable to read list: %v",
				volumeCR.Spec.StorageClass, err)
		}

		for _, a := range acList.Items {
			if a.Spec.Location == volumeCR.Spec.Location {
				acCR = a
				break
			}
		}
		if acCR.Name != "" {
			// AC was found, update it size (increase)
			acCR.Spec.Size += volumeCR.Spec.Size
			if err = vo.k8sClient.UpdateCRWithAttempts(ctx, &acCR, 5); err != nil {
				ll.Errorf("Unable to update AC %s size: %v", acCR.Name, err)
			}
		}
	}

	if err = vo.k8sClient.DeleteCR(ctx, &volumeCR); err != nil {
		ll.Errorf("unable to delete volume CR %s: %v", volumeID, err)
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
			ll.Warnf("Context is done but volume still not reach one of the expected state.")
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

// ReadVolumeAndChangeStatus reads Volume CR (10 attempts) and updates it with newStatus (10 attempts)
// Receives volumeID of a Volume CR which should be modified and Spec.CSIStatus - newStatus for that Volume CR
// Returns error if something went wrong
func (vo *VolumeOperationsImpl) ReadVolumeAndChangeStatus(volumeID string, newStatus string) error {
	vo.log.WithFields(logrus.Fields{
		"method":   "ReadVolumeAndChangeStatus",
		"volumeID": volumeID,
	}).Infof("Read volume and set status to %s", newStatus)

	var (
		v        = &volumecrd.Volume{}
		attempts = 10
		ctx      = context.WithValue(context.Background(), base.RequestUUID, volumeID)
	)

	if err := vo.k8sClient.ReadCRWithAttempts(volumeID, v, attempts); err != nil {
		return err
	}

	// change status
	v.Spec.CSIStatus = newStatus
	if err := vo.k8sClient.UpdateCRWithAttempts(ctx, v, attempts); err != nil {
		return err
	}
	return nil
}
