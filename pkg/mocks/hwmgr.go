package mocks

import (
	"context"
	"errors"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"google.golang.org/grpc"
)

// MockHWMgrClient implements HWManager interface
type MockHWMgrClient struct {
	drives []*api.Drive
}

// MockHWMgrClientFail returns error
type MockHWMgrClientFail struct {
}

func (m MockHWMgrClientFail) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return nil, errors.New("MockHWMgrClientFail: Error")
}

// NewMockHWMgrClient returns new instance of MockHWMgrClient
func NewMockHWMgrClient(drives []*api.Drive) *MockHWMgrClient {
	return &MockHWMgrClient{
		drives: drives,
	}
}

// SetDrives set drives for current MockHWMgrClient instance
func (m MockHWMgrClient) SetDrives(drives []*api.Drive) {
	m.drives = drives
}

// AddDrives extends drives slice
func (m MockHWMgrClient) AddDrives(drives ...*api.Drive) {
	m.drives = append(m.drives, drives...)
}

//GetDrivesList(ctx context.Context, in *DrivesRequest, opts ...grpc.CallOption) (*DrivesResponse, error)
// GetDrivesList return provided Drives
func (m MockHWMgrClient) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return &api.DrivesResponse{
		Disks: m.drives,
	}, nil
}
