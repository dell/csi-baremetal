package provisioners

import (
	"github.com/stretchr/testify/mock"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// MockProvisioner is a mock implementation of Provisioner interface
type MockProvisioner struct {
	mock.Mock
}

// GetMockProvisionerSuccess retruns MockProvisioner that perform each method without any error
// everytimePath is a path that will be returning for each "GetVolumePath" call
func GetMockProvisionerSuccess(everytimePath string) *MockProvisioner {
	mp := MockProvisioner{}
	mp.On("PrepareVolume", mock.Anything).Return(nil)
	mp.On("ReleaseVolume", mock.Anything).Return(nil)
	mp.On("GetVolumePath", mock.Anything).Return(everytimePath, nil)

	return &mp
}

// PrepareVolume is the mock implementation of PrepareVolume method from Provisioner interface
func (m *MockProvisioner) PrepareVolume(volume api.Volume) error {
	args := m.Mock.Called(volume)

	return args.Error(0)
}

// ReleaseVolume is the mock implementation of ReleaseVolume method from Provisioner interface
func (m *MockProvisioner) ReleaseVolume(volume api.Volume) error {
	args := m.Mock.Called(volume)

	return args.Error(0)
}

// GetVolumePath is the mock implementation of GetVolumePath method from Provisioner interface
func (m *MockProvisioner) GetVolumePath(volume api.Volume) (string, error) {
	args := m.Mock.Called(volume)

	return args.String(0), args.Error(1)
}
