package base

import (
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

var (
	testTcpEndpoint string         = "tcp://localhost:50051"
	testUdsEndpoint string         = "unix:///tmp/csi.sock"
	clientLogger    *logrus.Logger = logrus.New()
)

func TestNewClient(t *testing.T) {
	client, err := NewClient(nil, testTcpEndpoint, clientLogger)
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
	client, _ := NewClient(nil, testTcpEndpoint, clientLogger)
	err := client.Close()
	if err != nil {
		t.Errorf("err should be nil, got %v", err)
	}
}

func TestClient_GetEndpoint(t *testing.T) {
	c, _ := NewClient(nil, testTcpEndpoint, clientLogger)
	assert.Equal(t, "localhost:50051", c.GetEndpoint())

	c, _ = NewClient(nil, testUdsEndpoint, clientLogger)
	assert.Equal(t, "/tmp/csi.sock", c.GetEndpoint())
}
