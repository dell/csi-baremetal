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

package linuxutils

import (
	"github.com/stretchr/testify/mock"
)

// MockWrapPartition is a mock implementation of WrapPartition interface from partitionhelper package
type MockWrapPartition struct {
	mock.Mock
}

// DeviceHasPartitionTable is a mock implementations
func (m *MockWrapPartition) DeviceHasPartitionTable(device string) (bool, error) {
	args := m.Mock.Called(device)

	return args.Bool(0), args.Error(1)
}

// DeviceHasPartitions is a mock implementations
func (m *MockWrapPartition) DeviceHasPartitions(device string) (bool, error) {
	args := m.Mock.Called(device)

	return args.Bool(0), args.Error(1)
}

// IsPartitionExists is a mock implementations
func (m *MockWrapPartition) IsPartitionExists(device, partNum string) (exists bool, err error) {
	args := m.Mock.Called(device, partNum)

	return args.Bool(0), args.Error(1)
}

// GetPartitionTableType is a mock implementations
func (m *MockWrapPartition) GetPartitionTableType(device string) (ptType string, err error) {
	args := m.Mock.Called(device)

	return args.String(0), args.Error(1)
}

// CreatePartitionTable is a mock implementations
func (m *MockWrapPartition) CreatePartitionTable(device, partTableType string) (err error) {
	args := m.Mock.Called(device, partTableType)

	return args.Error(0)
}

// CreatePartition is a mock implementations
func (m *MockWrapPartition) CreatePartition(device, label, partUUID string) (err error) {
	args := m.Mock.Called(device, label, partUUID)

	return args.Error(0)
}

// DeletePartition is a mock implementations
func (m *MockWrapPartition) DeletePartition(device, partNum string) (err error) {
	args := m.Mock.Called(device, partNum)

	return args.Error(0)
}

// GetPartitionUUID is a mock implementations
func (m *MockWrapPartition) GetPartitionUUID(device, partNum string) (string, error) {
	args := m.Mock.Called(device, partNum)

	return args.String(0), args.Error(1)
}

// SyncPartitionTable is a mock implementations
func (m *MockWrapPartition) SyncPartitionTable(device string) error {
	args := m.Mock.Called(device)

	return args.Error(0)
}

// GetPartitionNameByUUID is a mock implementations
func (m *MockWrapPartition) GetPartitionNameByUUID(device, partUUID string) (string, error) {
	args := m.Mock.Called(device, partUUID)

	return args.String(0), args.Error(1)
}
