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

type VolumeOperations interface {
	CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string)
	WaitStatus(ctx context.Context, volumeID string, statuses ...string) (bool, string)
	ReadVolumeAndChangeStatus(volumeID string, newStatus string) error
}

type VolumeOperationsImpl struct {
	acProvider AvailableCapacityOperations
	k8sClient  *base.KubeClient

	log *logrus.Entry
}

func NewVolumeOperationsImpl(k8sClient *base.KubeClient, logger *logrus.Logger) *VolumeOperationsImpl {
	return &VolumeOperationsImpl{
		k8sClient:  k8sClient,
		acProvider: NewACOperationsImpl(k8sClient, logger),
		log:        logger.WithField("component", "VolumeOperationsImpl"),
	}
}

// CreateVolume search AC and create volume CR or returns existed volume CR
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
			sc             api.StorageClass
			allocatedBytes int64
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
		case api.StorageClass_HDDLVG, api.StorageClass_SSDLVG:
			allocatedBytes = v.Size
		default:
			allocatedBytes = ac.Spec.Size
		}

		// create volume CR
		apiVolume := api.Volume{
			Id:           v.Id,
			NodeId:       ac.Spec.NodeId,
			Size:         allocatedBytes,
			Location:     ac.Spec.Location,
			CSIStatus:    apiV1.Creating,
			StorageClass: sc,
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
// if volume CR doesn't exists return Not found error and that error should be handled by caller
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
	if volumeCR.Spec.StorageClass == api.StorageClass_HDDLVG || volumeCR.Spec.StorageClass == api.StorageClass_SSDLVG {
		var (
			acCR   = accrd.AvailableCapacity{}
			acList = accrd.AvailableCapacityList{}
		)
		if err = vo.k8sClient.ReadList(ctx, &acList); err != nil {
			ll.Errorf("Volume was deleted but corresponding AC with SC %s hadn't updated, unable to read list: %v",
				volumeCR.Spec.StorageClass.String(), err)
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
// return true if one of the status had reached, or return false instead
// also return status that had reached or "", pull status while context is active
func (vo *VolumeOperationsImpl) WaitStatus(ctx context.Context, volumeID string, statuses ...string) (bool, string) {
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
			return false, ""
		case <-time.After(timeoutBetweenCheck):
			if err = vo.k8sClient.ReadCR(ctx, volumeID, v); err != nil {
				ll.Errorf("Unable to read volume CR: %v", err)
				if k8sError.IsNotFound(err) {
					ll.Error("Volume CR doesn't exist")
					return false, ""
				}
				continue
			}
			for _, s := range statuses {
				if v.Spec.CSIStatus == s {
					ll.Infof("Volume has reached %s state.", s)
					return true, s
				}
			}
		}
	}
}

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
