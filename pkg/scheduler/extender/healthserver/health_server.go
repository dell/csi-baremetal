package healthserver

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"

	"github.com/dell/csi-baremetal/pkg/scheduler/extender/healthserver/common"
)

// ExtenderHealthServer provides endpoint for extender readiness check
type ExtenderHealthServer struct {
	logger            *logrus.Logger
	reader            yamlReader
	nodeName          string
	isPatchingEnabled bool
}

type yamlReader interface {
	getStatuses() (*common.ReadinessStatusList, error)
}

type yamlReaderImpl struct {
	statusFilePath string
}

// NewExtenderHealthServer constructs ExtenderHealthServer for extender pod
func NewExtenderHealthServer(logger *logrus.Logger, isPatchingEnabled bool) (*ExtenderHealthServer, error) {
	nodeName := os.Getenv("KUBE_NODE_NAME")
	if nodeName == "" {
		return nil, errors.New("nodeName parameter is empty")
	}

	return &ExtenderHealthServer{
		logger: logger,
		reader: &yamlReaderImpl{
			statusFilePath: common.ExtenderConfigMapFullPath,
		},
		nodeName:          nodeName,
		isPatchingEnabled: isPatchingEnabled,
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

	if !e.isPatchingEnabled {
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

func (y *yamlReaderImpl) getStatuses() (*common.ReadinessStatusList, error) {
	readinessStatuses := &common.ReadinessStatusList{}

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
