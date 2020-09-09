package testutils

import (
	"context"
	"time"

	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

// VolumeReconcileImitation looking for volume CR with name volId and sets it's status to newStatus
func VolumeReconcileImitation(k8sClient *k8s.KubeClient, volID string, newStatus string) {
	for {
		<-time.After(200 * time.Millisecond)
		err := ReadVolumeAndChangeStatus(k8sClient, volID, newStatus)
		if err != nil {
			return
		}
	}
}

// AddAC create test AvailableCapacities
func AddAC(k8sClient *k8s.KubeClient, acs ...*accrd.AvailableCapacity) error {
	for _, ac := range acs {
		if err := k8sClient.CreateCR(context.Background(), ac.Name, ac); err != nil {
			return err
		}
	}
	return nil
}

// ReadVolumeAndChangeStatus returns error if something went wrong
func ReadVolumeAndChangeStatus(k8sClient *k8s.KubeClient, volumeID string, newStatus string) error {
	var (
		v        = &volumecrd.Volume{}
		attempts = 10
		ctx      = context.WithValue(context.Background(), k8s.RequestUUID, volumeID)
	)

	if err := k8sClient.ReadCRWithAttempts(volumeID, v, attempts); err != nil {
		return err
	}

	// change status
	v.Spec.CSIStatus = newStatus
	if err := k8sClient.UpdateCRWithAttempts(ctx, v, attempts); err != nil {
		return err
	}
	return nil
}
