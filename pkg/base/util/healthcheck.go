package util

import (
	"github.com/sirupsen/logrus"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
)

// SetupAndStartHealthCheckServer starts gRPC server to handle Health checking requests
func SetupAndStartHealthCheckServer(c health.HealthServer, logger *logrus.Logger, endpoint string) error {
	healthServer := rpc.NewServerRunner(nil, endpoint, logger)
	// register Health checks
	logger.Info("Registering health check service")
	health.RegisterHealthServer(healthServer.GRPCServer, c)
	return healthServer.RunServer()
}
