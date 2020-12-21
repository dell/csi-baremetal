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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	testTcpEndpoint  string         = "tcp://localhost:50051"
	testUdsEndpoint  string         = "unix:///tmp/csi.sock"
	testFailEndpoint string         = "dsf:// df df :sdf"
	clientLogger     *logrus.Logger = logrus.New()
)

func TestNewClient_Success(t *testing.T) {
	client, err := NewClient(nil, testTcpEndpoint,false, clientLogger)
	assert.Nil(t, err)
	assert.Nil(t, client.Creds)
	assert.NotNil(t, client.GRPCClient)
	assert.Equal(t, testTcpEndpoint, client.Endpoint)
}

func TestNewClient_Fail(t *testing.T) {
	client, err := NewClient(nil, testFailEndpoint,false, clientLogger)
	assert.NotNil(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unable to create client")
}

func TestClientClose(t *testing.T) {
	client, _ := NewClient(nil, testTcpEndpoint,false, clientLogger)
	err := client.Close()
	assert.Nil(t, err)
}

func TestClient_GetEndpoint(t *testing.T) {
	c, _ := NewClient(nil, testTcpEndpoint,false, clientLogger)
	actual, err := c.GetEndpoint()
	assert.Nil(t, err)
	assert.Equal(t, "localhost:50051", actual)

	c, _ = NewClient(nil, testUdsEndpoint,false, clientLogger)
	actual, err = c.GetEndpoint()
	assert.Nil(t, err)
	assert.Equal(t, "/tmp/csi.sock", actual)
}
