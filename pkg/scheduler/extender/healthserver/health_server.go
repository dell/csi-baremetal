package healthserver

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
)

// ExtenderHealthServer provides endpoint for extender readiness check
type ExtenderHealthServer struct {
	logger   *logrus.Logger
	reader   yamlReader
	nodeName string
}

// ReadinessStatus contains info about kube-scheduler restart for the related node
type ReadinessStatus struct {
	NodeName      string `yaml:"node_name"`
	KubeScheduler string `yaml:"kube_scheduler"`
	Restarted     bool   `yaml:"restarted"`
}

// ReadinessStatusList contains info about all kube-schedulers
type ReadinessStatusList struct {
	Items []ReadinessStatus `yaml:"nodes"`
}

type yamlReader interface {
	getStatuses() (*ReadinessStatusList, error)
	isPathSet() bool
}

type yamlReaderImpl struct {
	statusFilePath string
}

// NewExtenderHealthServer constructs ExtenderHealthServer for extender pod
func NewExtenderHealthServer(logger *logrus.Logger, statusFilePath, nodeName string) (*ExtenderHealthServer, error) {
	if nodeName == "" {
		return nil, errors.New("nodeName parameter is empty")
	}

	return &ExtenderHealthServer{
		logger: logger,
		reader: &yamlReaderImpl{
			statusFilePath: statusFilePath,
		},
		nodeName: nodeName,
	}, nil
}

const (
	ready    = 0
	notReady = 1
	notFound = 3
)

var (
	readyResponse    = &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}
	notReadyResponse = &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}
)

// Check does the health check and changes the status of the server based on drives cache size
func (e *ExtenderHealthServer) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	ll := e.logger.WithFields(logrus.Fields{
		"method": "Check",
	})

	if !e.reader.isPathSet() {
		ll.Debugf("Patcher is not enabled")
		return readyResponse, nil
	}

	readinessStatuses, err := e.reader.getStatuses()
	if err != nil {
		return notReadyResponse, err
	}

	var scheduler string
	isReady := notFound
	for _, nodeStatus := range readinessStatuses.Items {
		if nodeStatus.NodeName == e.nodeName {
			scheduler = nodeStatus.KubeScheduler
			if nodeStatus.Restarted {
				isReady = ready
				break
			} else {
				isReady = notReady
				break
			}
		}
	}

	if isReady == ready {
		return readyResponse, nil
	}

	if isReady == notReady {
		return notReadyResponse, fmt.Errorf("kube-scheduler %s is not restarted after patching", scheduler)
	}

	return notReadyResponse, fmt.Errorf("node %s is not found in extenders status list", e.nodeName)
}

// Watch is used by clients to receive updates when the svc status changes.
// Watch only dummy implemented just to satisfy the interface.
func (e *ExtenderHealthServer) Watch(*grpc_health_v1.HealthCheckRequest, grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

func (y *yamlReaderImpl) getStatuses() (*ReadinessStatusList, error) {
	readinessStatuses := &ReadinessStatusList{}

	yamlFile, err := ioutil.ReadFile(y.statusFilePath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, readinessStatuses)
	if err != nil {
		return nil, err
	}

	return readinessStatuses, nil
}

func (y *yamlReaderImpl) isPathSet() bool {
	return y.statusFilePath != ""
}
