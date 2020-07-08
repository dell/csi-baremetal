package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	api "github.com/dell/csi-baremetal.git/api/generated/v1"
)

// VolumeOperationsMock is the mock implementation of VolumeOperations interface for test purposes.
// All of the mock methods based on stretchr/testify/mock.
type VolumeOperationsMock struct {
	mock.Mock
}

// CreateVolume is the mock implementation of CreateVolume method from VolumeOperations made for simulating
// creating of Volume CR on a cluster.
// Returns a fake api.Volume instance
func (vo *VolumeOperationsMock) CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error) {
	args := vo.Mock.Called(ctx, v)

	return args.Get(0).(*api.Volume), args.Error(1)
}

// DeleteVolume is the mock implementation of DeleteVolume method from VolumeOperations made for simulating
// deletion of Volume CR on a cluster.
// Returns error if user simulates error in tests or nil
func (vo *VolumeOperationsMock) DeleteVolume(ctx context.Context, volumeID string) error {
	args := vo.Mock.Called(ctx, volumeID)

	return args.Error(0)
}

// UpdateCRsAfterVolumeDeletion is the mock implementation of UpdateCRsAfterVolumeDeletion
func (vo *VolumeOperationsMock) UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string) {

}

// WaitStatus is the mock implementation of WaitStatus. Simulates waiting of Volume to be reached one of provided
// statuses
func (vo *VolumeOperationsMock) WaitStatus(ctx context.Context, volumeID string, statuses ...string) error {
	args := vo.Mock.Called(ctx, volumeID, statuses)

	return args.Error(0)
}

// ReadVolumeAndChangeStatus is the mock implementation of ReadVolumeAndChangeStatus. Simulates updating of Volume CR
// with newStatus
func (vo *VolumeOperationsMock) ReadVolumeAndChangeStatus(volumeID string, newStatus string) error {
	args := vo.Mock.Called(volumeID, newStatus)

	return args.Error(0)
}
