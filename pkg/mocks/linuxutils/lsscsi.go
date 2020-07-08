package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"github.com/dell/csi-baremetal.git/pkg/base/linuxutils/lsscsi"
)

// MockWrapLsscsi is a mock implementation of WrapLsscsi interface from lsscsi package
type MockWrapLsscsi struct {
	mock.Mock
}

// GetSCSIDevices is a mock implementations
func (m *MockWrapLsscsi) GetSCSIDevices() ([]*lsscsi.SCSIDevice, error) {
	args := m.Mock.Called()

	return args.Get(0).([]*lsscsi.SCSIDevice), args.Error(1)
}
