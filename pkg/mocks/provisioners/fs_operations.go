package provisioners

import mocklu "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks/linuxutils"

// MockFsOpts is a mock implementation of FSOperation interface from volumeprovisioner package
type MockFsOpts struct {
	mocklu.MockWrapFS
}

// PrepareAndPerformMount is a mock implementation
func (m *MockFsOpts) PrepareAndPerformMount(src, dst string, bindMount bool) error {
	args := m.Mock.Called(src, dst, bindMount)

	return args.Error(0)
}
