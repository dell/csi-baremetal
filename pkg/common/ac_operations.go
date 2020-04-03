package common

import (
	"context"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
)

type AvailableCapacityOperations interface {
	SearchAC(ctx context.Context, node string, requiredBytes int64, sc api.StorageClass) *accrd.AvailableCapacity
	UpdateACSizeOrDelete(ac *accrd.AvailableCapacity, bytes int64) error
}
