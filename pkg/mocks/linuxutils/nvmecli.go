package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/nvmecli"
)

// MockWrapNvmecli is a mock implementation of WrapNvmecli interface from nvmee package
type MockWrapNvmecli struct {
	mock.Mock
}

// GetNVMDevices is a mock implementations
func (m *MockWrapNvmecli) GetNVMDevices() ([]nvmecli.NVMDevice, error) {
	args := m.Mock.Called()

	return args.Get(0).([]nvmecli.NVMDevice), args.Error(1)
}
