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

package mocks

import (
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/stretchr/testify/mock"
)

// MockCRHelper is the mock implementation of CRHelper interface for test purposes
type MockCRHelper struct {
	mock.Mock
}

// GetVolumeByID is the mock implementation of GetVolumeByID method from CRHelper made for simulating
// returning some errors for get Volume CR by ID
// Returns nil for volume and error if user simulates error in tests or nil
func (cs *MockCRHelper) GetVolumeByID(volID string) (*volumecrd.Volume, error) {
	args := cs.Mock.Called(volID)
	return nil, args.Error(0)
}
