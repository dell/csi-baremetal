package util

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/health/grpc_health_v1"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	grpc "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks/rpc"
)

var testLogger = logrus.New()

func Test_SetupAndStartHealthCheckServer(t *testing.T) {
	healthServer := rpc.NewMockHealthServer()
	endpoint := fmt.Sprintf("tcp://%s:%d", base.DefaultHealthIP, base.DefaultHealthPort)
	go func() {
		err := SetupAndStartHealthCheckServer(healthServer, testLogger, endpoint)
		assert.Nil(t, err)
	}()
	time.Sleep(3 * time.Second)

	client, err := grpc.NewClient(nil, endpoint, testLogger)
	assert.Nil(t, err)

	healthClient := grpc_health_v1.NewHealthClient(client.GRPCClient)

	check, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	assert.Nil(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, check.Status)
}
