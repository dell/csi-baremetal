package base

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	host        = "localhost"
	port        = int32(4243)
	nonSecureSR *ServerRunner
)

func TestMain(m *testing.M) {
	nonSecureSR = NewServerRunner(nil, host, port)
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
	if !isTCPPortOpen(nonSecureSR.GetEndpoint()) {
		t.Errorf("TCP port %d should be opened", port)
	}

	// try to create server on same endpoint
	nonSecureSR2 := NewServerRunner(nil, host, port)
	err := nonSecureSR2.RunServer()
	if err == nil {
		t.Errorf("Trying to create server for same endpoint. Error should appear but it doesn't.")
	} else if !strings.Contains(err.Error(), "address already in use") {
		t.Errorf("Got error %v. 'address already in use' should be in error message but it doesn't", err)
	}
}

func TestServerRunner_GetEndpoint(t *testing.T) {
	expectedEndpoint := fmt.Sprintf("%s:%d", host, port)
	currEndpoint := nonSecureSR.GetEndpoint()
	if expectedEndpoint != currEndpoint {
		t.Errorf("Got endpoint %s, expected %s", currEndpoint, expectedEndpoint)
	}
}

func TestServerRunner_StopServer(t *testing.T) {
	// stop server
	nonSecureSR.StopServer()
}

// try to connect to provided endpoint with 4 attempts with 0.5 sec timeout
func isTCPPortOpen(e string) bool {
	for i := 0; i < 4; i++ {
		// check that port is open
		conn, err := net.DialTimeout("tcp", e, time.Second)
		if err == nil && conn != nil {
			conn.Close()
			return true
		} else {
			time.Sleep(time.Millisecond * 500)
		}
	}
	return false
}
