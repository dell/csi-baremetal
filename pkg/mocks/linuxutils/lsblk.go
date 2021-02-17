/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, version 2.0 (the "License");
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

// GetVersion is a ock implementation
func (m *MockWrapLsblk) GetVersion() lsblk.Version {
	return lsblk.Version{}
}
