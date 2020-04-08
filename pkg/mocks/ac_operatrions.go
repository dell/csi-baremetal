package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
)

type ACOperationsMock struct {
	mock.Mock
}

func (a *ACOperationsMock) SearchAC(ctx context.Context, node string, requiredBytes int64, sc api.StorageClass) *accrd.AvailableCapacity {
	args := a.Mock.Called(ctx, node, requiredBytes, sc)

	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*accrd.AvailableCapacity)
}

func (a *ACOperationsMock) DeleteIfEmpty(ctx context.Context, acLocation string) error {
	args := a.Mock.Called(ctx, acLocation)
	return args.Error(0)
}
