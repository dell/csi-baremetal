package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

type VolumeOperationsMock struct {
	mock.Mock
}

func (vo *VolumeOperationsMock) CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error) {
	args := vo.Mock.Called(ctx, v)

	return args.Get(0).(*api.Volume), args.Error(1)
}

func (vo *VolumeOperationsMock) DeleteVolume(ctx context.Context, volumeID string) error {
	args := vo.Mock.Called(ctx, volumeID)

	return args.Error(0)
}

func (vo *VolumeOperationsMock) UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string) {

}

func (vo *VolumeOperationsMock) WaitStatus(ctx context.Context, volumeID string, statuses ...api.OperationalStatus) (bool, api.OperationalStatus) {
	args := vo.Mock.Called(ctx, volumeID, statuses)

	return args.Bool(0), args.Get(1).(api.OperationalStatus)
}

func (vo *VolumeOperationsMock) ReadVolumeAndChangeStatus(volumeID string, newStatus api.OperationalStatus) error {
	args := vo.Mock.Called(volumeID, newStatus)

	return args.Error(0)
}
