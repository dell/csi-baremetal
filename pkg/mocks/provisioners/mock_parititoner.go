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

package provisioners

import (
	"github.com/stretchr/testify/mock"

	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
	"github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
)

// MockPartitionOps is a mock implementation of WrapPartition interface
type MockPartitionOps struct {
	mocklu.MockWrapPartition
	mock.Mock
}

// PreparePartition is a mock implementation
func (m *MockPartitionOps) PreparePartition(p utilwrappers.Partition) (*utilwrappers.Partition, error) {
	args := m.Mock.Called(p)

	return args.Get(0).(*utilwrappers.Partition), args.Error(1)
}

// ReleasePartition is a mock implementation
func (m *MockPartitionOps) ReleasePartition(p utilwrappers.Partition) error {
	args := m.Mock.Called(p)

	return args.Error(0)
}

// SearchPartName is a mock implementation
func (m *MockPartitionOps) SearchPartName(device, partUUID string) (string, error) {
	args := m.Mock.Called(device, partUUID)

	return args.String(0), args.Error(1)
}
