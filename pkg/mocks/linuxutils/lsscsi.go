package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsscsi"
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
