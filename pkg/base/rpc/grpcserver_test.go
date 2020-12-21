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
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	basenet "github.com/dell/csi-baremetal/pkg/base/net"
)

var (
	port         int32 = 4243
	socketType         = "tcp"
	address            = fmt.Sprintf("localhost:%d", port)
	endpoint           = fmt.Sprintf("%s://localhost:%d", socketType, port)
	nonSecureSR  *ServerRunner
	serverLogger = logrus.New()
)

func TestMain(m *testing.M) {
	nonSecureSR = NewServerRunner(nil, endpoint, false, serverLogger)
	code := m.Run()
	nonSecureSR.StopServer()
	os.Exit(code)
}

func TestNewServerRunner(t *testing.T) {
	if nonSecureSR.Creds != nil {
		t.Errorf("Creds should be nil")
	}

	if nonSecureSR.GRPCServer == nil {
		t.Errorf("gRPC server must be initialized but got nil")
	}
}

func TestServerRunner_RunServer(t *testing.T) {
	go func() {
		err2 := nonSecureSR.RunServer()
		if err2 != nil {
			t.Errorf("Server should started. Got error: %v", err2)
		}
	}()

	// Ensure that endpoint is accessible
	if ok, _ := basenet.IsTCPPortOpen(address); !ok {
		t.Errorf("TCP port %d should be opened", port)
	}

	// try to create server on same endpoint
	nonSecureSR2 := NewServerRunner(nil, endpoint, false, serverLogger)
	err := nonSecureSR2.RunServer()
	if err == nil {
		t.Errorf("Trying to create server for same endpoint. Error should appear but it doesn't.")
	} else if !strings.Contains(err.Error(), "address already in use") {
		t.Errorf("Got error %v. 'address already in use' should be in error message but it doesn't", err)
	}
}

func TestServerRunner_GetEndpoint(t *testing.T) {
	endpoint, socket := nonSecureSR.GetEndpoint()
	assert.Equal(t, address, endpoint)
	assert.Equal(t, socketType, socket)

	unixAddr := "unix:///tmp/csi.sock"
	unixSrv := NewServerRunner(nil, unixAddr, false, serverLogger)
	endpoint, socket = unixSrv.GetEndpoint()
	assert.Equal(t, "unix", socket)
	assert.Equal(t, "/tmp/csi.sock", endpoint)
}

func TestServerRunner_StopServer(t *testing.T) {
	// stop server
	nonSecureSR.StopServer()
}
