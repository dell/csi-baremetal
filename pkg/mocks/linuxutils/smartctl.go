package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/smartctl"
)

// MockWrapSmartctl is a mock implementation of WrapSmartctl interface from smartctl package
type MockWrapSmartctl struct {
	mock.Mock
}

// GetDriveInfoByPath is a mock implementations
func (m *MockWrapSmartctl) GetDriveInfoByPath(path string) (*smartctl.DeviceSMARTInfo, error) {
	args := m.Mock.Called(path)

	return args.Get(0).(*smartctl.DeviceSMARTInfo), args.Error(1)
}
