package healthserver

import (
	"context"
	"errors"
	"testing"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	reader *mockYamlReader

	eReady    = &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}
	eNotReady = &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}
)

func TestExtenderHealthServerCheck(t *testing.T) {
	t.Run("Check_Ready", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &ReadinessStatusList{
				Items: []ReadinessStatus{
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: true},
				}}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(statuses, nil)
		reader.On("isPathSet").Return(true)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, eReady, responce)
	})

	t.Run("Check_NotReady", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &ReadinessStatusList{
				Items: []ReadinessStatus{
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: false},
				}}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(statuses, nil)
		reader.On("isPathSet").Return(true)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, eNotReady, responce)
	})

	t.Run("Check_Ready_MultipleExtenders", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &ReadinessStatusList{
				Items: []ReadinessStatus{
					{NodeName: nodeName + "1", KubeScheduler: schedulerName, Restarted: true},
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: true},
					{NodeName: nodeName + "2", KubeScheduler: schedulerName, Restarted: true},
				}}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(statuses, nil)
		reader.On("isPathSet").Return(true)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, eReady, responce)
	})

	t.Run("Check_NotEnabled", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &ReadinessStatusList{
				Items: []ReadinessStatus{
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: true},
				}}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(statuses, nil)
		reader.On("isPathSet").Return(false)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, eReady, responce)
	})

	t.Run("Check_ReaderError", func(t *testing.T) {
		var (
			nodeName    = "node"
			returnedErr = errors.New("some_error")
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(&ReadinessStatusList{}, returnedErr)
		reader.On("isPathSet").Return(true)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Equal(t, returnedErr, err)
		assert.Equal(t, eNotReady, responce)
	})
}

func prepareExtenderHealthServer(nodeName string) (*ExtenderHealthServer, error) {
	logger, err := base.InitLogger("", "DEBUG")
	if err != nil {
		return nil, err
	}

	reader = &mockYamlReader{}

	return &ExtenderHealthServer{
		logger:   logger,
		reader:   reader,
		nodeName: nodeName,
	}, nil
}

type mockYamlReader struct {
	mock.Mock
}

func (m *mockYamlReader) getStatuses() (*ReadinessStatusList, error) {
	args := m.Mock.Called()

	return args.Get(0).(*ReadinessStatusList), args.Error(1)
}

func (m *mockYamlReader) isPathSet() bool {
	args := m.Mock.Called()

	return args.Bool(0)
}
