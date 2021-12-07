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

package provisioners

import (
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
)

// MockFsOpts is a mock implementation of FSOperation interface from volumeprovisioner package
type MockFsOpts struct {
	mocklu.MockWrapFS
}

// PrepareAndPerformMount is a mock implementation
func (m *MockFsOpts) PrepareAndPerformMount(src, dst string, bindMount, dstIsDir bool, mountOptions ...string) error {
	args := m.Mock.Called(src, dst, bindMount, dstIsDir)

	return args.Error(0)
}

// MountFakeTmpfs is a mock implementation
func (m *MockFsOpts) MountFakeTmpfs(volumeID, path string) error {
	args := m.Mock.Called(volumeID, path)

	return args.Error(0)
}

// UnmountWithCheck is a mock implementation
func (m *MockFsOpts) UnmountWithCheck(path string) error {
	args := m.Mock.Called(path)

	return args.Error(0)
}

// CreateFSIfNotExist is a mock implementation
func (m *MockFsOpts) CreateFSIfNotExist(fsType fs.FileSystem, device string) error {
	args := m.Mock.Called(fsType, device)

	return args.Error(0)
}
