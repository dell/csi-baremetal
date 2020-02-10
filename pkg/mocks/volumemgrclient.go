package mocks

import (
	"context"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

type VolumeMgrClientMock struct {
	mock.Mock
}

func (m *VolumeMgrClientMock) GetLocalVolumes(ctx context.Context,
	req *api.VolumeRequest, opts ...grpc.CallOption) (*api.VolumeResponse, error) {
	args := m.Mock.Called(req)
	return args.Get(0).(*api.VolumeResponse), args.Error(1)
}

func (m *VolumeMgrClientMock) GetAvailableCapacity(ctx context.Context,
	req *api.AvailableCapacityRequest, opts ...grpc.CallOption) (*api.AvailableCapacityResponse, error) {
	args := m.Mock.Called(req)
	return args.Get(0).(*api.AvailableCapacityResponse), args.Error(1)
}

func (m *VolumeMgrClientMock) CreateLocalVolume(ctx context.Context,
	req *api.CreateLocalVolumeRequest, opts ...grpc.CallOption) (*api.CreateLocalVolumeResponse, error) {
	args := m.Mock.Called(req)
	return args.Get(0).(*api.CreateLocalVolumeResponse), args.Error(1)
}

func (m *VolumeMgrClientMock) DeleteLocalVolume(ctx context.Context,
	req *api.DeleteLocalVolumeRequest, opts ...grpc.CallOption) (*api.DeleteLocalVolumeResponse, error) {
	args := m.Mock.Called(req)
	return args.Get(0).(*api.DeleteLocalVolumeResponse), args.Error(1)
}
