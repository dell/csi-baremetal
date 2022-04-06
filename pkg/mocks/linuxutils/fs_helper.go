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

	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
)

// MockWrapFS is a mock implementation of WrapFS interface from fs package
type MockWrapFS struct {
	mock.Mock
}

// GetFSType is a mock implementations
func (m *MockWrapFS) GetFSType(device string) (string, error) {
	args := m.Mock.Called(device)

	return args.String(0), args.Error(1)
}

// GetFSUUID is a mock implementations
func (m *MockWrapFS) GetFSUUID(device string) (string, error) {
	args := m.Mock.Called(device)

	return args.String(0), args.Error(1)
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

// MkFile is a mock implementations
func (m *MockWrapFS) MkFile(src string) error {
	args := m.Mock.Called(src)

	return args.Error(0)
}

// RmDir is a mock implementations
func (m *MockWrapFS) RmDir(src string) error {
	args := m.Mock.Called(src)

	return args.Error(0)
}

// CreateFS is a mock implementations
func (m *MockWrapFS) CreateFS(fsType fs.FileSystem, device, uuid string) error {
	args := m.Mock.Called(fsType, device, uuid)

	return args.Error(0)
}

// WipeFS is a mock implementations
func (m *MockWrapFS) WipeFS(device string) error {
	args := m.Mock.Called(device)

	return args.Error(0)
}

// IsMounted is a mock implementations
func (m *MockWrapFS) IsMounted(src string) (bool, error) {
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
