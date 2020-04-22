package sc

import (
	"github.com/stretchr/testify/mock"
)

// ImplementerMock is the mock implementation of StorageClassImplementer interface for test purposes
type ImplementerMock struct {
	mock.Mock
}

// CreateFileSystem mocks creating of specified file system on the provided device
// Receives file system as a var of FileSystem type and path of the device as a string
// Returns error if user simulates error in tests
func (m *ImplementerMock) CreateFileSystem(fsType FileSystem, device string) error {
	args := m.Mock.Called(fsType, device)
	return args.Error(0)
}

// DeleteFileSystem mocks deletion of file system from the provided device
// Receives file path of the device as a string
// Returns error if user simulates error in tests
func (m *ImplementerMock) DeleteFileSystem(device string) error {
	args := m.Mock.Called(device)
	return args.Error(0)
}

// CreateTargetPath mocks creation of specified path
// Receives directory path to create as a string
// Returns error if user simulates error in tests
func (m *ImplementerMock) CreateTargetPath(path string) error {
	args := m.Mock.Called(path)
	return args.Error(0)
}

// DeleteTargetPath mocks deletion of specified path
// Receives directory path to delete as a string
// Returns error if user simulates error in tests
func (m *ImplementerMock) DeleteTargetPath(path string) error {
	args := m.Mock.Called(path)
	return args.Error(0)
}

// IsMounted mocks checking if the partition of device mounted
// Receives partition path as a string
// Returns bool that simulates mount status or error if user simulates error in tests
func (m *ImplementerMock) IsMounted(device string) (bool, error) {
	args := m.Mock.Called(device)
	return args.Bool(0), args.Error(1)
}

// Mount mocks mounting of source path to the destination directory
// Receives source path and destination dir and also opts parameters that are used for mount command for example --bind
// Returns error if user simulates error in tests
func (m *ImplementerMock) Mount(src, dir string, opts ...string) error {
	args := m.Mock.Called(src, dir, opts)
	return args.Error(0)
}

// Unmount mocks unmounting of device from the specified path
// Receives path where the device is mounted
// Returns error if user simulates error in tests
func (m *ImplementerMock) Unmount(path string) error {
	args := m.Mock.Called(path)
	return args.Error(0)
}

// IsMountPoint mocks checking if the specified path is mount point
// Receives path that should be checked
// Returns bool that simulates if the path is the mount point or error if user simulates error in tests
func (m *ImplementerMock) IsMountPoint(path string) (bool, error) {
	args := m.Mock.Called(path)
	return args.Bool(0), args.Error(1)
}

// PrepareVolume mocks preparing of a volume in NodePublish() call
// Receives device that the volume should be based on and a targetPath where the device should be mounted
// Returns simulation of rollBacked status and error if user simulates error in tests
func (m *ImplementerMock) PrepareVolume(device, targetPath string) (bool, error) {
	args := m.Mock.Called(device, targetPath)
	return args.Bool(0), args.Error(1)
}
