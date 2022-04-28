package healthserver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/health/grpc_health_v1"

	baselogger "github.com/dell/csi-baremetal/pkg/base/logger"
	"github.com/dell/csi-baremetal/pkg/scheduler/extender/healthserver/common"
)

var reader *mockYamlReader

func TestExtenderHealthServerCheck(t *testing.T) {
	t.Run("Check_Ready", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &common.ReadinessStatusList{
				Items: []common.ReadinessStatus{
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: true},
				},
			}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(statuses, nil)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, readyResponse, responce)
	})

	t.Run("Check_NotReady", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &common.ReadinessStatusList{
				Items: []common.ReadinessStatus{
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: false},
				},
			}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(statuses, nil)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.NotNil(t, err)
		assert.Equal(t, notReadyResponse, responce)
	})

	t.Run("Check_Ready_MultipleExtenders", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &common.ReadinessStatusList{
				Items: []common.ReadinessStatus{
					{NodeName: nodeName + "1", KubeScheduler: schedulerName, Restarted: true},
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: true},
					{NodeName: nodeName + "2", KubeScheduler: schedulerName, Restarted: true},
				},
			}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(statuses, nil)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, readyResponse, responce)
	})

	t.Run("Check_NotEnabled", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &common.ReadinessStatusList{
				Items: []common.ReadinessStatus{
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: true},
				},
			}
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		server.isPatchingEnabled = false

		reader.On("getStatuses").Return(statuses, nil)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Nil(t, err)
		assert.Equal(t, readyResponse, responce)
	})

	t.Run("Check_ReaderError", func(t *testing.T) {
		var (
			nodeName    = "node"
			returnedErr = errors.New("some_error")
		)

		server, err := prepareExtenderHealthServer(nodeName)
		assert.Nil(t, err)

		reader.On("getStatuses").Return(&common.ReadinessStatusList{}, returnedErr)

		responce, err := server.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		assert.Equal(t, returnedErr, err)
		assert.Equal(t, notReadyResponse, responce)
	})
}

func prepareExtenderHealthServer(nodeName string) (*ExtenderHealthServer, error) {
	logger, err := baselogger.InitLogger("", "DEBUG")
	if err != nil {
		return nil, err
	}

	reader = &mockYamlReader{}

	return &ExtenderHealthServer{
		logger:            logger,
		reader:            reader,
		nodeName:          nodeName,
		isPatchingEnabled: true,
	}, nil
}

type mockYamlReader struct {
	mock.Mock
}

func (m *mockYamlReader) getStatuses() (*common.ReadinessStatusList, error) {
	args := m.Mock.Called()

	return args.Get(0).(*common.ReadinessStatusList), args.Error(1)
}

func (m *mockYamlReader) isPathSet() bool {
	args := m.Mock.Called()

	return args.Bool(0)
}
