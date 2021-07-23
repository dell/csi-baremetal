package healthserver

import (
	"context"
	"errors"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"

	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

type ExtenderHealthServer struct {
	kubeClient     *k8s.KubeClient
	logger         *logrus.Logger
	statusFilePath string
	nodeName       string
}

type ReadinessStatus struct {
	NodeName      string `yaml:"node_name"`
	KubeScheduler string `yaml:"kube_scheduler"`
	Restarted     bool   `yaml:"restarted"`
}

type ReadinessStatusList struct {
	Items []ReadinessStatus `yaml:"nodes"`
}

func NewExtenderHealthServer(kubeClient *k8s.KubeClient, logger *logrus.Logger, statusFilePath, nodeName string) (*ExtenderHealthServer, error) {
	if nodeName == "" {
		return nil, errors.New("nodeName parameter is empty")
	}

	return &ExtenderHealthServer{
		kubeClient:     kubeClient,
		logger:         logger,
		statusFilePath: statusFilePath,
		nodeName:       nodeName,
	}, nil
}

// Check does the health check and changes the status of the server based on drives cache size
func (e *ExtenderHealthServer) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	ll := e.logger.WithFields(logrus.Fields{
		"method": "Check",
	})

	if e.statusFilePath == "" {
		ll.Debugf("Patcher is not enabled")
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	}

	yamlFile, err := ioutil.ReadFile(e.statusFilePath)
	if err != nil {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, err
	}

	readinessStatuses := &ReadinessStatusList{}
	ll.Debugf("%s", yamlFile)

	err = yaml.Unmarshal(yamlFile, readinessStatuses)
	if err != nil {
		ll.Debugf("%s", err)
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, err
	}

	for _, nodeStatus := range readinessStatuses.Items {
		if nodeStatus.NodeName == e.nodeName {
			if nodeStatus.Restarted {
				return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
			} else {
				return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
			}
		}
	}

	ll.Errorf("Node %s is not found in extenders status list", e.nodeName)
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
}

// Watch is used by clients to receive updates when the svc status changes.
// Watch only dummy implemented just to satisfy the interface.
func (e *ExtenderHealthServer) Watch(*grpc_health_v1.HealthCheckRequest, grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}
