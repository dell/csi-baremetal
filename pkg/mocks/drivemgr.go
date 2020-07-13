package mocks

import (
	"context"
	"errors"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"google.golang.org/grpc"
)

// MockDriveMgrClient is the implementation of DriveManager interface to imitate success state
type MockDriveMgrClient struct {
	drives []*api.Drive
}

// MockDriveMgrClientFail is the implementation of DriveManager interface to imitate failure state
type MockDriveMgrClientFail struct {
}

// GetDrivesList is the simulation of failure during DriveManager's GetDrivesList
// Returns nil DrivesResponse and non nil error
func (m MockDriveMgrClientFail) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return nil, errors.New("drivemgr error")
}

// NewMockDriveMgrClient returns new instance of MockDriveMgrClient
// Receives slice of api.Drive which would be used in imitation of GetDrivesList
func NewMockDriveMgrClient(drives []*api.Drive) *MockDriveMgrClient {
	return &MockDriveMgrClient{
		drives: drives,
	}
}

// SetDrives set drives for current MockDriveMgrClient instance
func (m MockDriveMgrClient) SetDrives(drives []*api.Drive) {
	m.drives = drives
}

// AddDrives extends drives slice
func (m MockDriveMgrClient) AddDrives(drives ...*api.Drive) {
	m.drives = append(m.drives, drives...)
}

// GetDrivesList returns provided to MockDriveMgrClient drives to imitate working of DriveManager
func (m MockDriveMgrClient) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return &api.DrivesResponse{
		Disks: m.drives,
	}, nil
}
