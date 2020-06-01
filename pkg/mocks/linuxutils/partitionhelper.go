package linuxutils

import (
	"github.com/stretchr/testify/mock"
)

// MockWrapPartition is a mock implementation of WrapPartition interface from partitionhelper package
type MockWrapPartition struct {
	mock.Mock
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
func (m *MockWrapPartition) CreatePartition(device, label string) (err error) {
	args := m.Mock.Called(device, label)

	return args.Error(0)
}

// DeletePartition is a mock implementations
func (m *MockWrapPartition) DeletePartition(device, partNum string) (err error) {
	args := m.Mock.Called(device, partNum)

	return args.Error(0)
}

// SetPartitionUUID is a mock implementations
func (m *MockWrapPartition) SetPartitionUUID(device, partNum, partUUID string) error {
	args := m.Mock.Called(device, partNum, partUUID)

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
