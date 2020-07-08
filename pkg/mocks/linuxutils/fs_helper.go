package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"github.com/dell/csi-baremetal.git/pkg/base/linuxutils/fs"
)

// MockWrapFS is a mock implementation of WrapFS interface from fs package
type MockWrapFS struct {
	mock.Mock
}

// GetFSSpace is a mock implementations
func (m *MockWrapFS) GetFSSpace(src string) (int64, error) {
	args := m.Mock.Called(src)

	return args.Get(0).(int64), args.Error(1)
}

// MkDir is a mock implementations
func (m *MockWrapFS) MkDir(src string) error {
	args := m.Mock.Called(src)

	return args.Error(0)
}

// RmDir is a mock implementations
func (m *MockWrapFS) RmDir(src string) error {
	args := m.Mock.Called(src)

	return args.Error(0)
}

// CreateFS is a mock implementations
func (m *MockWrapFS) CreateFS(fsType fs.FileSystem, device string) error {
	args := m.Mock.Called(fsType, device)

	return args.Error(0)
}

// WipeFS is a mock implementations
func (m *MockWrapFS) WipeFS(device string) error {
	args := m.Mock.Called(device)

	return args.Error(0)
}

// GetFSType is a mock implementations
func (m *MockWrapFS) GetFSType(device string) (fs.FileSystem, error) {
	args := m.Mock.Called(device)

	return args.Get(0).(fs.FileSystem), args.Error(1)
}

// IsMounted is a mock implementations
func (m *MockWrapFS) IsMounted(src string) (bool, error) {
	args := m.Mock.Called(src)

	return args.Bool(0), args.Error(1)
}

// IsMountPoint is a mock implementations
func (m *MockWrapFS) IsMountPoint(src string) (bool, error) {
	args := m.Mock.Called(src)

	return args.Bool(0), args.Error(1)
}

// FindMountPoint is a mock implementations
func (m *MockWrapFS) FindMountPoint(target string) (string, error) {
	args := m.Mock.Called(target)

	return args.String(0), args.Error(1)
}

// Mount is a mock implementations
func (m *MockWrapFS) Mount(src, dst string, opts ...string) error {
	args := m.Mock.Called(src, dst, opts)

	return args.Error(0)
}

// Unmount is a mock implementations
func (m *MockWrapFS) Unmount(src string) error {
	args := m.Mock.Called(src)

	return args.Error(0)
}
