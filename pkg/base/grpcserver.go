package base

import (
	"fmt"
	"net"
	"net/url"

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
}

// NewServerRunner returns ServerRunner object based on parameters that had provided
func NewServerRunner(creds credentials.TransportCredentials, endpoint string) *ServerRunner {
	sr := &ServerRunner{
		Creds:    creds,
		Endpoint: endpoint,
	}
	sr.init()
	endp, socket := sr.GetEndpoint()
	logrus.Info("endpoint = ", endp)
	logrus.Info("socket = ", socket)
	return sr
}

// init creates Listener for ServerRunner and initialized GRPCServer
func (sr *ServerRunner) init() {
	if sr.Creds != nil {
		sr.GRPCServer = grpc.NewServer(grpc.Creds(sr.Creds))
	} else {
		sr.GRPCServer = grpc.NewServer()
	}
}

// RunServer starts gRPC server in gorutine
func (sr *ServerRunner) RunServer() error {
	var err error
	endpoint, socket := sr.GetEndpoint()
	sr.listener, err = net.Listen(socket, endpoint)
	if err != nil {
		logrus.Errorf("failed to create listener for endpoint %s: %v", endpoint, err)
		return err
	}
	fmt.Printf("Starting gRPC server for endpoint %s and socket %s", endpoint, socket)
	return sr.GRPCServer.Serve(sr.listener)
}

// StopServer gracefully stops gRPC server and closes listener
func (sr *ServerRunner) StopServer() {
	fmt.Println("Stopping server")
	sr.GRPCServer.GracefulStop()
}

// GetEndpoint returns endpoint representation based on hostTCP and port
func (sr *ServerRunner) GetEndpoint() (string, string) {
	u, _ := url.Parse(sr.Endpoint)

	if u.Scheme == unix {
		return u.Path, unix
	}

	return u.Host, tcp
}
