package base

import (
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ServerRunner encapsulates logic for creating/starting/stopping gRPC server
type ServerRunner struct {
	GRPCServer *grpc.Server
	listener   net.Listener
	Creds      credentials.TransportCredentials
	Host       string
	Port       int32
}

// NewServerRunner returns ServerRunner object based on parameters that had provided
func NewServerRunner(creds credentials.TransportCredentials, host string, port int32) *ServerRunner {
	sr := &ServerRunner{
		Creds: creds,
		Host:  host,
		Port:  port,
	}
	sr.init()
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
	endpoint := sr.GetEndpoint()
	sr.listener, err = net.Listen("tcp", endpoint)
	if err != nil {
		logrus.Errorf("failed to create listener for endpoint %s: %v", endpoint, err)
		return err
	}
	// start new server
	fmt.Printf("Starting gRPC server for endpoint %s", sr.GetEndpoint())
	return sr.GRPCServer.Serve(sr.listener)
}

// StopServer gracefully stops gRPC server and closes listener
func (sr *ServerRunner) StopServer() {
	fmt.Println("Stopping server")
	sr.GRPCServer.GracefulStop()
}

// GetEndpoint returns endpoint representation based on host and port
func (sr *ServerRunner) GetEndpoint() string {
	return fmt.Sprintf("%s:%d", sr.Host, sr.Port)
}
