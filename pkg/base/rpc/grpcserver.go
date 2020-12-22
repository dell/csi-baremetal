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

package rpc

import (
	"net"
	"net/url"
	"os"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
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
	GRPCServer     *grpc.Server
	listener       net.Listener
	Creds          credentials.TransportCredentials
	Endpoint       string
	log            *logrus.Entry
	metricsEnabled bool
}

// NewServerRunner returns ServerRunner object based on parameters that had provided
// Receives credentials for connection, connection endpoint (for example 'tcp://localhost:8888') and logrus logger
// Returns an instance of ServerRunner struct
func NewServerRunner(creds credentials.TransportCredentials, endpoint string, enableMetrics bool, logger *logrus.Logger) *ServerRunner {
	sr := &ServerRunner{
		Creds:          creds,
		Endpoint:       endpoint,
		metricsEnabled: enableMetrics,
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
	opts := make([]grpc.ServerOption, 0)
	if sr.Creds != nil {
		opts = append(opts, grpc.Creds(sr.Creds))
	}

	if sr.metricsEnabled {
		opts = append(opts, grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor), grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor))
	}
	sr.GRPCServer = grpc.NewServer(opts...)
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
