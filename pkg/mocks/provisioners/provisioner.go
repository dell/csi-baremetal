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
