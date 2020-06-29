package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
)

// MockWrapLsblk is a mock implementation of WrapLsblk interface from lsblk package
type MockWrapLsblk struct {
	mock.Mock
}

// GetBlockDevices is a mock implementations
func (m *MockWrapLsblk) GetBlockDevices(device string) ([]lsblk.BlockDevice, error) {
	args := m.Mock.Called(device)

	return args.Get(0).([]lsblk.BlockDevice), args.Error(1)
}

// SearchDrivePath is a mock implementations
func (m *MockWrapLsblk) SearchDrivePath(drive *drivecrd.Drive) (string, error) {
	args := m.Mock.Called(drive)

	return args.String(0), args.Error(1)
}

// SetExecutor is a mock implementations
func (m *MockWrapLsblk) SetExecutor(e command.CmdExecutor) {
	m.Mock.Called(e)
}
