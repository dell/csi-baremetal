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
	"fmt"
	"net/url"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Client encapsulates logic for new gRPC clint
type Client struct {
	GRPCClient     *grpc.ClientConn
	Creds          credentials.TransportCredentials
	Endpoint       string
	log            *logrus.Entry
	metricsEnabled bool
}

// NewClient creates new Client object with hostTCP, port, creds and calls init function
// Receives credentials for connection, connection endpoint (for example 'tcp://localhost:8888') and logrus logger
// Returns an instance of Client struct or error if initClient() function failed
func NewClient(creds credentials.TransportCredentials, endpoint string, enableMetrics bool, logger *logrus.Logger) (*Client, error) {
	client := &Client{
		Creds:          creds,
		Endpoint:       endpoint,
		metricsEnabled: enableMetrics,
	}
	client.SetLogger(logger)
	err := client.initClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create client, error: %v", err)
	}
	return client, nil
}

// SetLogger sets logrus logger to Client struct
// Receives logrus logger
func (c *Client) SetLogger(logger *logrus.Logger) {
	c.log = logger.WithField("component", "Client")
}

// initClient defines ClientConn field in Client struct
// Returns error if client's endpoint is incorrect or grpc.Dial() failed
func (c *Client) initClient() error {
	endpoint, err := c.GetEndpoint()
	if err != nil {
		return err
	}

	c.log.Infof("Initialize client for endpoint \"%s\"", endpoint)

	opts := make([]grpc.DialOption, 0, 1)
	if c.Creds != nil {
		opts = append(opts, grpc.WithTransportCredentials(c.Creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if c.metricsEnabled {
		metricsOpts := []grpc.DialOption{grpc.WithUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor), grpc.WithStreamInterceptor(grpc_prometheus.StreamClientInterceptor)}
		opts = append(opts, metricsOpts...)
	}

	c.GRPCClient, err = grpc.Dial(endpoint, opts...)
	if err != nil {
		return err
	}
	return nil
}

// Close function calls Close method in ClientConn
// Returns error if something went wrong
func (c *Client) Close() error {
	return c.GRPCClient.Close()
}

// GetEndpoint returns endpoint representation
// Returns url.Path if Scheme is unix or url.Host otherwise
func (c *Client) GetEndpoint() (string, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return "", err
	}

	if u.Scheme == unix {
		return u.Path, nil
	}

	return u.Host, nil
}
