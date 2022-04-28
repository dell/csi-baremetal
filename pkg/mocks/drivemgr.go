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
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// MockDriveMgrClient is the implementation of DriveManager interface to imitate success state
type MockDriveMgrClient struct {
	drives []*api.Drive
}

// MockDriveMgrClientFail is the implementation of DriveManager interface to imitate failure state
type MockDriveMgrClientFail struct{}

// GetDrivesList is the simulation of failure during DriveManager's GetDrivesList
// Returns nil DrivesResponse and non nil error
func (m *MockDriveMgrClientFail) GetDrivesList(_ context.Context, _ *api.DrivesRequest, _ ...grpc.CallOption) (*api.DrivesResponse, error) {
	return nil, errors.New("drivemgr error")
}

// Locate is a stub for Locate DriveManager's method
func (m *MockDriveMgrClientFail) Locate(_ context.Context, _ *api.DriveLocateRequest, _ ...grpc.CallOption) (*api.DriveLocateResponse, error) {
	return nil, errors.New("locate failed")
}

// LocateNode is a stub for LocateNode DriveManager's method
func (m *MockDriveMgrClientFail) LocateNode(_ context.Context, _ *api.NodeLocateRequest, _ ...grpc.CallOption) (*api.Empty, error) {
	return nil, errors.New("locate node failed")
}

// NewMockDriveMgrClient returns new instance of MockDriveMgrClient
// Receives slice of api.Drive which would be used in imitation of GetDrivesList
func NewMockDriveMgrClient(drives []*api.Drive) *MockDriveMgrClient {
	return &MockDriveMgrClient{
		drives: drives,
	}
}

// SetDrives set drives for current MockDriveMgrClient instance
func (m *MockDriveMgrClient) SetDrives(drives []*api.Drive) {
	m.drives = drives
}

// AddDrives extends drives slice
func (m *MockDriveMgrClient) AddDrives(drives ...*api.Drive) {
	m.drives = append(m.drives, drives...)
}

// GetDrivesList returns provided to MockDriveMgrClient drives to imitate working of DriveManager
func (m *MockDriveMgrClient) GetDrivesList(_ context.Context, _ *api.DrivesRequest, _ ...grpc.CallOption) (*api.DrivesResponse, error) {
	return &api.DrivesResponse{
		Disks: m.drives,
	}, nil
}

// Locate is a stub for Locate DriveManager's method
func (m *MockDriveMgrClient) Locate(_ context.Context, _ *api.DriveLocateRequest, _ ...grpc.CallOption) (*api.DriveLocateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Locate not implemented in MockDriveMgrClient")
}

// LocateNode is a stub for LocateNode DriveManager's method
func (m *MockDriveMgrClient) LocateNode(_ context.Context, _ *api.NodeLocateRequest, _ ...grpc.CallOption) (*api.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method LocateNode not implemented in MockDriveMgrClient")
}
