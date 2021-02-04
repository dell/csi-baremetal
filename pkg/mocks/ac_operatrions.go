/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mocks contains mock implementation of CSI methods for test purposes
package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
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

// RecreateACToLVGSC is the mock implementation of RecreateACToLVGSC method from AvailableCapacityOperations made for simulating
// recreation of list of ACs to LVG AC
// Returns error if user simulates error in tests or nil
func (a *ACOperationsMock) RecreateACToLVGSC(ctx context.Context, acName, sc string, acs ...accrd.AvailableCapacity) *accrd.AvailableCapacity {
	args := a.Mock.Called(ctx, acName, sc, acs)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*accrd.AvailableCapacity)
}
