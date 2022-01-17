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
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/dell/csi-baremetal/api/generated/v1/api"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

// VolumeOperationsMock is the mock implementation of VolumeOperations interface for test purposes.
// All of the mock methods based on stretchr/testify/mock.
type VolumeOperationsMock struct {
	mock.Mock
}

// CreateVolume is the mock implementation of CreateVolume method from VolumeOperations made for simulating
// creating of Volume CR on a cluster.
// Returns a fake api.Volume instance
func (vo *VolumeOperationsMock) CreateVolume(ctx context.Context, v api.Volume) (*api.Volume, error) {
	args := vo.Mock.Called(ctx, v)

	return args.Get(0).(*api.Volume), args.Error(1)
}

// DeleteVolume is the mock implementation of DeleteVolume method from VolumeOperations made for simulating
// deletion of Volume CR on a cluster.
// Returns error if user simulates error in tests or nil
func (vo *VolumeOperationsMock) DeleteVolume(ctx context.Context, volumeID string) error {
	args := vo.Mock.Called(ctx, volumeID)

	return args.Error(0)
}

// UpdateCRsAfterVolumeDeletion is the mock implementation of UpdateCRsAfterVolumeDeletion
func (vo *VolumeOperationsMock) UpdateCRsAfterVolumeDeletion(ctx context.Context, volumeID string) {

}

// WaitStatus is the mock implementation of WaitStatus. Simulates waiting of Volume to be reached one of provided
// statuses
func (vo *VolumeOperationsMock) WaitStatus(ctx context.Context, volumeID string, statuses ...string) error {
	args := vo.Mock.Called(ctx, volumeID, statuses)

	return args.Error(0)
}

// ExpandVolume is the mock implementation of ExpandVolume method from VolumeOperations made for simulating
// Receive golang context, volume CR, requiredBytes as int
// Return volume spec, error
func (vo *VolumeOperationsMock) ExpandVolume(ctx context.Context, volume *volumecrd.Volume, requiredBytes int64) error {
	args := vo.Mock.Called(ctx, volume, requiredBytes)
	return args.Error(0)
}

// UpdateCRsAfterVolumeExpansion is the mock implementation of UpdateCRsAfterVolumeExpansion method from VolumeOperations made for simulating
// Receive golang context, volume spec
// Return  error
func (vo *VolumeOperationsMock) UpdateCRsAfterVolumeExpansion(ctx context.Context, volID string, requiredBytes int64) {
	vo.Mock.Called(ctx, volID, requiredBytes)
}
