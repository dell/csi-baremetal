package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"github.com/dell/csi-baremetal.git/pkg/base/linuxutils/smartctl"
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
