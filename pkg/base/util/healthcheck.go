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
	"github.com/sirupsen/logrus"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/dell/csi-baremetal/pkg/base/rpc"
)

// SetupAndStartHealthCheckServer starts gRPC server to handle Health checking requests
func SetupAndStartHealthCheckServer(c health.HealthServer, logger *logrus.Logger, endpoint string) error {
	healthServer := rpc.NewServerRunner(nil, endpoint, logger)
	// register Health checks
	logger.Info("Registering health check service")
	health.RegisterHealthServer(healthServer.GRPCServer, c)
	return healthServer.RunServer()
}
