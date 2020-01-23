package node

import (
	"context"

	volumemanager "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

type VolumeManager struct{}

//GetLocalVolumes request return array of volumes on node
func (v VolumeManager) GetLocalVolumes(context.Context, *volumemanager.VolumeRequest) (*volumemanager.VolumeResponse, error) {
	volumes := make([]*volumemanager.Volume, 0)
	return &volumemanager.VolumeResponse{Volume: volumes}, nil
}

//GetAvailableCapacity request return array of free capacity on node
func (v VolumeManager) GetAvailableCapacity(context.Context, *volumemanager.AvailableCapacityRequest) (*volumemanager.AvailableCapacityResponse, error) {
	capacities := make([]*volumemanager.AvailableCapacity, 0)
	return &volumemanager.AvailableCapacityResponse{AvailableCapacity: capacities}, nil
}

// depending on SC and parameters in CreateVolumeRequest()
// here we should use different SC implementations for creating required volumes
// the same principle we can use in Controller Server or read from a CRD instance
