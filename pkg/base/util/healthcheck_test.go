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

package util

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/dell/csi-baremetal/pkg/base"
	grpc "github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/mocks/rpc"
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

	client, err := grpc.NewClient(nil, endpoint, false, testLogger)
	assert.Nil(t, err)

	healthClient := grpc_health_v1.NewHealthClient(client.GRPCClient)

	check, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	assert.Nil(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, check.Status)
}
