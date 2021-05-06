package linuxutils

import "github.com/stretchr/testify/mock"

// MockWrapDataDiscover is a mock implementation of WrapDataDiscover interface from datadiscover package
type MockWrapDataDiscover struct {
	mock.Mock
}

// DiscoverData is a mock implementations
func (m *MockWrapDataDiscover) DiscoverData(device, serialNumber string) (bool, error) {
	args := m.Mock.Called(device, serialNumber)

	return args.Bool(0), args.Error(1)
}
