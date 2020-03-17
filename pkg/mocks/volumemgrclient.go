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

func (m *VolumeMgrClientMock) GetAvailableCapacity(ctx context.Context,
	req *api.AvailableCapacityRequest, opts ...grpc.CallOption) (*api.AvailableCapacityResponse, error) {
	args := m.Mock.Called(req)
	return args.Get(0).(*api.AvailableCapacityResponse), args.Error(1)
}
