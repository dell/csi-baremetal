package testutils

import (
	"context"
	"time"

	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/common"
)

// VolumeReconcileImitation looking for volume CR with name volId and sets it's status to newStatus
func VolumeReconcileImitation(svc common.VolumeOperations, volID string, newStatus string) {
	for {
		<-time.After(200 * time.Millisecond)
		err := svc.ReadVolumeAndChangeStatus(volID, newStatus)
		if err != nil {
			return
		}
	}
}

// AddAC create test AvailableCapacities
func AddAC(k8sClient *base.KubeClient, acs ...*accrd.AvailableCapacity) error {
	for _, ac := range acs {
		if err := k8sClient.CreateCR(context.Background(), ac.Name, ac); err != nil {
			return err
		}
	}
	return nil
}
