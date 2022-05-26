/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package linuxutils

import (
	"github.com/stretchr/testify/mock"
)

// MockWrapLVM is a mock implementation of WrapLVM interface from lvm package
type MockWrapLVM struct {
	mock.Mock
}

// ExpandLV is a mock implementations
func (m *MockWrapLVM) ExpandLV(lvName string, requiredSize int64) error {
	args := m.Mock.Called(lvName, requiredSize)

	return args.Error(0)
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

// VGScan is a mock implementation
func (m *MockWrapLVM) VGScan(name string) (bool, error) {
	args := m.Mock.Called(name)

	return args.Bool(0), args.Error(1)
}

// VGReactivate is a mock implementation
func (m *MockWrapLVM) VGReactivate(name string) error {
	args := m.Mock.Called(name)

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

// GetVgFreeSpace is a mock implementations
func (m *MockWrapLVM) GetVgFreeSpace(vgName string) (int64, error) {
	args := m.Mock.Called(vgName)

	return args.Get(0).(int64), args.Error(1)
}

// GetLVsInVG is a mock implementations
func (m *MockWrapLVM) GetLVsInVG(vgName string) ([]string, error) {
	args := m.Mock.Called(vgName)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]string), args.Error(1)
}

// GetAllPVs is a mock implementations
func (m *MockWrapLVM) GetAllPVs() ([]string, error) {
	args := m.Mock.Called()

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]string), args.Error(1)
}

// GetVGNameByPVName is a mock implementations
func (m *MockWrapLVM) GetVGNameByPVName(pvName string) (string, error) {
	args := m.Mock.Called(pvName)

	return args.String(0), args.Error(1)
}
