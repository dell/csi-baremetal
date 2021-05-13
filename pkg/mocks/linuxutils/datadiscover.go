package linuxutils

import (
	"github.com/stretchr/testify/mock"

	"github.com/dell/csi-baremetal/pkg/base/linuxutils/datadiscover/types"
)

// MockWrapDataDiscover is a mock implementation of WrapDataDiscover interface from datadiscover package
type MockWrapDataDiscover struct {
	mock.Mock
}

// DiscoverData is a mock implementations
func (m *MockWrapDataDiscover) DiscoverData(device, serialNumber string) (*types.DiscoverResult, error) {
	args := m.Mock.Called(device, serialNumber)

	return args.Get(0).(*types.DiscoverResult), args.Error(1)
}
