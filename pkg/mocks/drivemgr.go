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
	"encoding/json"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

// SmartInfo is a structure to represent basic Smart metrics
type SmartInfo map[string]map[string]map[string]string

// MockDriveMgrClient is the implementation of DriveManager interface to imitate success state
type MockDriveMgrClient struct {
	drives    []*api.Drive
	smartInfo SmartInfo
}

// MockDriveMgrClientFail is the implementation of DriveManager interface to imitate failure state
type MockDriveMgrClientFail struct {
	Code codes.Code
}

// MockDriveMgrClientFailJSON is the implementation of Drive Manager interface to imitate invalid JSONs
type MockDriveMgrClientFailJSON struct {
	MockJSON string
}

// GetDrivesList is the simulation of failure during DriveManager's GetDrivesList
// Returns nil DrivesResponse and non nil error
func (m *MockDriveMgrClientFail) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return nil, errors.New("drivemgr error")
}

// Locate is a stub for Locate DriveManager's method
func (m *MockDriveMgrClientFail) Locate(ctx context.Context, in *api.DriveLocateRequest, opts ...grpc.CallOption) (*api.DriveLocateResponse, error) {
	return nil, errors.New("locate failed")
}

// LocateNode is a stub for LocateNode DriveManager's method
func (m *MockDriveMgrClientFail) LocateNode(ctx context.Context, in *api.NodeLocateRequest, opts ...grpc.CallOption) (*api.Empty, error) {
	return nil, errors.New("locate node failed")
}

// GetDriveSmartInfo is a stub for GetDriveSmartInfo DriveManager's method
func (m *MockDriveMgrClientFail) GetDriveSmartInfo(ctx context.Context, req *api.SmartInfoRequest, opts ...grpc.CallOption) (*api.SmartInfoResponse, error) {
	return nil, status.Errorf(m.Code, "method GetDriveSmartInfo in MockDriveMgrClient returns: %d", m.Code)
}

// GetAllDrivesSmartInfo is a stub for GetAllDrivesSmartInfo DriveManager's method
func (m *MockDriveMgrClientFail) GetAllDrivesSmartInfo(ctx context.Context, req *api.Empty, opts ...grpc.CallOption) (*api.SmartInfoResponse, error) {
	return nil, status.Errorf(m.Code, "method GetAllDrivesSmartInfo in MockDriveMgrClient returns: %d", m.Code)
}

// NewMockDriveMgrClient returns new instance of MockDriveMgrClient
// Receives slice of api.Drive which would be used in imitation of GetDrivesList
func NewMockDriveMgrClient(drives []*api.Drive, smartInfo SmartInfo) *MockDriveMgrClient {
	return &MockDriveMgrClient{
		drives:    drives,
		smartInfo: smartInfo,
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
func (m *MockDriveMgrClient) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return &api.DrivesResponse{
		Disks: m.drives,
	}, nil
}

// Locate is a stub for Locate DriveManager's method
func (m *MockDriveMgrClient) Locate(ctx context.Context, in *api.DriveLocateRequest, opts ...grpc.CallOption) (*api.DriveLocateResponse, error) {
	return &api.DriveLocateResponse{Status: apiV1.LocateStatusOn}, nil
}

// LocateNode is a stub for LocateNode DriveManager's method
func (m *MockDriveMgrClient) LocateNode(ctx context.Context, in *api.NodeLocateRequest, opts ...grpc.CallOption) (*api.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method LocateNode not implemented in MockDriveMgrClient")
}

// GetDriveSmartInfo is a stub for GetDriveSmartInfo DriveManager's method
func (m *MockDriveMgrClient) GetDriveSmartInfo(ctx context.Context, req *api.SmartInfoRequest, opts ...grpc.CallOption) (*api.SmartInfoResponse, error) {
	serialNumber := req.GetSerialNumber()
	smartInfo := m.smartInfo[serialNumber]

	if smartInfo != nil {
		smartInfoStr, _ := json.Marshal(smartInfo)
		return &api.SmartInfoResponse{
			SmartInfo: string(smartInfoStr),
		}, nil
	}
	return nil, status.Errorf(codes.NotFound, "failed to get smart info of drive %s: %v", serialNumber, "drive doesn't exist")
}

// GetAllDrivesSmartInfo is a stub for GetAllDrivesSmartInfo DriveManager's method
func (m *MockDriveMgrClient) GetAllDrivesSmartInfo(ctx context.Context, req *api.Empty, opts ...grpc.CallOption) (*api.SmartInfoResponse, error) {
	if m.smartInfo != nil {
		smartInfo, _ := json.Marshal(m.smartInfo)
		return &api.SmartInfoResponse{
			SmartInfo: string(smartInfo),
		}, nil
	}
	return nil, status.Errorf(codes.NotFound, "failed to get smart info of all drives: NotFound")
}

// GetDrivesList is the simulation of failure during DriveManager's GetDrivesList
// Returns nil DrivesResponse and non nil error
func (m *MockDriveMgrClientFailJSON) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return nil, errors.New("drivemgr error")
}

// Locate is a stub for Locate DriveManager's method
func (m *MockDriveMgrClientFailJSON) Locate(ctx context.Context, in *api.DriveLocateRequest, opts ...grpc.CallOption) (*api.DriveLocateResponse, error) {
	return nil, errors.New("locate failed")
}

// LocateNode is a stub for LocateNode DriveManager's method
func (m *MockDriveMgrClientFailJSON) LocateNode(ctx context.Context, in *api.NodeLocateRequest, opts ...grpc.CallOption) (*api.Empty, error) {
	return nil, errors.New("locate node failed")
}

// GetDriveSmartInfo is a stub for GetDriveSmartInfo DriveManager's method
func (m *MockDriveMgrClientFailJSON) GetDriveSmartInfo(ctx context.Context, req *api.SmartInfoRequest, opts ...grpc.CallOption) (*api.SmartInfoResponse, error) {
	return &api.SmartInfoResponse{
		SmartInfo: m.MockJSON,
	}, nil
}

// GetAllDrivesSmartInfo is a stub for GetAllDrivesSmartInfo DriveManager's method
func (m *MockDriveMgrClientFailJSON) GetAllDrivesSmartInfo(ctx context.Context, req *api.Empty, opts ...grpc.CallOption) (*api.SmartInfoResponse, error) {
	return &api.SmartInfoResponse{
		SmartInfo: m.MockJSON,
	}, nil
}
