package mocks

import (
	"context"
	"errors"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"google.golang.org/grpc"
)

// MockHWMgrClient is the implementation of HWManager interface to imitate success state
type MockHWMgrClient struct {
	drives []*api.Drive
}

// MockHWMgrClientFail is the implementation of HWManager interface to imitate failure state
type MockHWMgrClientFail struct {
}

// GetDrivesList is the simulation of failure during HWManager's GetDrivesList
// Returns nil DrivesResponse and non nil error
func (m MockHWMgrClientFail) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return nil, errors.New("hwmgr error")
}

// NewMockHWMgrClient returns new instance of MockHWMgrClient
// Receives slice of api.Drive which would be used in imitation of GetDrivesList
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

// GetDrivesList returns provided to MockHWMgrClient drives to imitate working of HWManager
func (m MockHWMgrClient) GetDrivesList(ctx context.Context, in *api.DrivesRequest, opts ...grpc.CallOption) (*api.DrivesResponse, error) {
	return &api.DrivesResponse{
		Disks: m.drives,
	}, nil
}
