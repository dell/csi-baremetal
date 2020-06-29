package rpc

import (
	"context"

	"google.golang.org/grpc/health/grpc_health_v1"
)

// MockHealthServer is a mock implementation for health check server
type MockHealthServer struct{}

// NewMockHealthServer is a constructor for MockHealthServer type
func NewMockHealthServer() *MockHealthServer {
	return &MockHealthServer{}
}

// Check is mock implementation for health check function
func (c *MockHealthServer) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

// Watch is mock implementation for health check function
func (c *MockHealthServer) Watch(*grpc_health_v1.HealthCheckRequest, grpc_health_v1.Health_WatchServer) error {
	return nil
}
