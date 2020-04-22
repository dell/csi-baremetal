package base

import (
	"net"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	tcp  string = "tcp"
	unix string = "unix"
)

// ServerRunner encapsulates logic for creating/starting/stopping gRPC server
type ServerRunner struct {
	GRPCServer *grpc.Server
	listener   net.Listener
	Creds      credentials.TransportCredentials
	Endpoint   string
	log        *logrus.Entry
}

// NewServerRunner returns ServerRunner object based on parameters that had provided
// Receives credentials for connection, connection endpoint (for example 'tcp://localhost:8888') and logrus logger
// Returns an instance of ServerRunner struct
func NewServerRunner(creds credentials.TransportCredentials, endpoint string, logger *logrus.Logger) *ServerRunner {
	sr := &ServerRunner{
		Creds:    creds,
		Endpoint: endpoint,
	}
	sr.SetLogger(logger)
	sr.init()
	e, socket := sr.GetEndpoint()
	logger.Infof("Create server for endpoint \"%s\" on \"%s\" socket", e, socket)
	return sr
}

// SetLogger sets logrus logger to ServerRunner struct
// Receives logrus logger
func (sr *ServerRunner) SetLogger(logger *logrus.Logger) {
	sr.log = logger.WithField("component", "ServerRunner")
}

// init initializes GRPCServer field of ServerRunner struct
func (sr *ServerRunner) init() {
	if sr.Creds != nil {
		sr.GRPCServer = grpc.NewServer(grpc.Creds(sr.Creds))
	} else {
		sr.GRPCServer = grpc.NewServer()
	}
}

// RunServer creates Listener and starts gRPC server on endpoint
// Receives error if error occurred during Listener creation or during GRPCServer.Serve
func (sr *ServerRunner) RunServer() error {
	var err error
	endpoint, socket := sr.GetEndpoint()
	if socket == unix {
		// try to remove
		_ = os.Remove(endpoint)
	}
	sr.listener, err = net.Listen(socket, endpoint)
	if err != nil {
		sr.log.Errorf("failed to create listener for endpoint %s: %v", endpoint, err)
		return err
	}
	sr.log.Infof("Starting gRPC server for endpoint %s and socket %s", endpoint, socket)
	return sr.GRPCServer.Serve(sr.listener)
}

// StopServer gracefully stops gRPC server and closes listener
func (sr *ServerRunner) StopServer() {
	sr.log.Info("Stopping server")
	sr.GRPCServer.GracefulStop()
}

// GetEndpoint returns endpoint representation
// Returns url.Path if Scheme is unix or url.Host otherwise
func (sr *ServerRunner) GetEndpoint() (string, string) {
	u, _ := url.Parse(sr.Endpoint)

	if u.Scheme == unix {
		return u.Path, unix
	}

	return u.Host, tcp
}
