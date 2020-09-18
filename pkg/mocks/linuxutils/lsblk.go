package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
)

// MockWrapLsblk is a mock implementation of WrapLsblk interface from lsblk package
type MockWrapLsblk struct {
	mock.Mock
}

// GetBlockDevices is a mock implementations
func (m *MockWrapLsblk) GetBlockDevices(device string) ([]lsblk.BlockDevice, error) {
	args := m.Mock.Called(device)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]lsblk.BlockDevice), args.Error(1)
}

// SearchDrivePath is a mock implementations
func (m *MockWrapLsblk) SearchDrivePath(drive *drivecrd.Drive) (string, error) {
	args := m.Mock.Called(drive)

	return args.String(0), args.Error(1)
}
