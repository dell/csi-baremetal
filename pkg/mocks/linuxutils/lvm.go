package linuxutils

import (
	"github.com/stretchr/testify/mock"
)

// MockWrapLVM is a mock implementation of WrapLVM interface from lvm package
type MockWrapLVM struct {
	mock.Mock
}

// PVCreate is a mock implementations
func (m *MockWrapLVM) PVCreate(dev string) error {
	args := m.Mock.Called(dev)

	return args.Error(0)
}

// PVRemove is a mock implementations
func (m *MockWrapLVM) PVRemove(name string) error {
	args := m.Mock.Called(name)

	return args.Error(0)
}

// VGCreate is a mock implementations
func (m *MockWrapLVM) VGCreate(name string, pvs ...string) error {
	args := m.Mock.Called(name, pvs)

	return args.Error(0)
}

// VGRemove is a mock implementations
func (m *MockWrapLVM) VGRemove(name string) error {
	args := m.Mock.Called(name)

	return args.Error(0)
}

// LVCreate is a mock implementations
func (m *MockWrapLVM) LVCreate(name, size, vgName string) error {
	args := m.Mock.Called(name, size, vgName)

	return args.Error(0)
}

// LVRemove is a mock implementations
func (m *MockWrapLVM) LVRemove(fullLVName string) error {
	args := m.Mock.Called(fullLVName)

	return args.Error(0)
}

// IsVGContainsLVs is a mock implementations
func (m *MockWrapLVM) IsVGContainsLVs(vgName string) bool {
	args := m.Mock.Called(vgName)

	return args.Bool(0)
}

// RemoveOrphanPVs is a mock implementations
func (m *MockWrapLVM) RemoveOrphanPVs() error {
	args := m.Mock.Called()

	return args.Error(0)
}

// FindVgNameByLvName is a mock implementations
func (m *MockWrapLVM) FindVgNameByLvName(lvName string) (string, error) {
	args := m.Mock.Called(lvName)

	return args.String(0), args.Error(1)
}

// GetVgFreeSpace is a mock implementations
func (m *MockWrapLVM) GetVgFreeSpace(vgName string) (int64, error) {
	args := m.Mock.Called(vgName)

	return args.Get(0).(int64), args.Error(1)
}

// IsMountPointInLVG is a mock implementations
func (m *MockWrapLVM) IsMountPointInLVG(mountpoint string) (bool, error) {
	args := m.Mock.Called(mountpoint)

	return args.Bool(0), args.Error(1)
}
