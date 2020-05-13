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
	client, err := NewClient(nil, testTcpEndpoint, clientLogger)
	assert.Nil(t, err)
	assert.Nil(t, client.Creds)
	assert.NotNil(t, client.GRPCClient)
	assert.Equal(t, testTcpEndpoint, client.Endpoint)
}

func TestNewClient_Fail(t *testing.T) {
	client, err := NewClient(nil, testFailEndpoint, clientLogger)
	assert.NotNil(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unable to create client")
}

func TestClientClose(t *testing.T) {
	client, _ := NewClient(nil, testTcpEndpoint, clientLogger)
	err := client.Close()
	assert.Nil(t, err)
}

func TestClient_GetEndpoint(t *testing.T) {
	c, _ := NewClient(nil, testTcpEndpoint, clientLogger)
	actual, err := c.GetEndpoint()
	assert.Nil(t, err)
	assert.Equal(t, "localhost:50051", actual)

	c, _ = NewClient(nil, testUdsEndpoint, clientLogger)
	actual, err = c.GetEndpoint()
	assert.Nil(t, err)
	assert.Equal(t, "/tmp/csi.sock", actual)
}
