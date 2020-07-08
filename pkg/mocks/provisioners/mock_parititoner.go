package provisioners

import (
	"github.com/stretchr/testify/mock"

	mocklu "github.com/dell/csi-baremetal.git/pkg/mocks/linuxutils"
	"github.com/dell/csi-baremetal.git/pkg/node/provisioners/utilwrappers"
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
func (m *MockPartitionOps) SearchPartName(device, partUUID string) string {
	args := m.Mock.Called(device, partUUID)

	return args.String(0)
}
