package base

import (
	"testing"

	"gotest.tools/assert"
)

var (
	testTcpEndpoint string = "tcp://localhost:50051"
	testUdsEndpoint string = "unix:///tmp/csi.sock"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient(nil, testTcpEndpoint)
	if err != nil {
		t.FailNow()
	}
	if client.Creds != nil {
		t.Errorf("Creds should be nil")
	}
	if client.GRPCClient == nil {
		t.Errorf("gRPC client must be initialized but got nil")
	}
	if client.Endpoint != testTcpEndpoint {
		t.Error("Endpoints are not equal")
	}
}

func TestClientClose(t *testing.T) {
	client, _ := NewClient(nil, testTcpEndpoint)
	err := client.Close()
	if err != nil {
		t.Errorf("err should be nil, got %v", err)
	}
}

func TestClient_GetEndpoint(t *testing.T) {
	c, _ := NewClient(nil, testTcpEndpoint)
	assert.Equal(t, "localhost:50051", c.GetEndpoint())

	c, _ = NewClient(nil, testUdsEndpoint)
	assert.Equal(t, "/tmp/csi.sock", c.GetEndpoint())
}
