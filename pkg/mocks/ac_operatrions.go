// Package mocks contains mock implementation of CSI methods for test purposes
package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
)

// ACOperationsMock is the mock implementation of AvailableCapacityOperations interface for test purposes
type ACOperationsMock struct {
	mock.Mock
}

// SearchAC is the mock implementation of SearchAC method from AvailableCapacityOperations made for simulating
// searching AvailableCapacity on a cluster.
// Returns a fake AvailableCapacity instance
func (a *ACOperationsMock) SearchAC(ctx context.Context, node string, requiredBytes int64, sc string) *accrd.AvailableCapacity {
	args := a.Mock.Called(ctx, node, requiredBytes, sc)

	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*accrd.AvailableCapacity)
}

// DeleteIfEmpty is the mock implementation of DeleteIfEmpty method from AvailableCapacityOperations made for simulating
// deletion of empty AvailableCapacity which Location is acLocation.
// Returns error if user simulates error in tests or nil
func (a *ACOperationsMock) DeleteIfEmpty(ctx context.Context, acLocation string) error {
	args := a.Mock.Called(ctx, acLocation)
	return args.Error(0)
}
