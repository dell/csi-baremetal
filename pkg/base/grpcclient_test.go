package base

import (
	"testing"
)

type TestConfig struct {
	Host string
	Port string
}

var testConfig = &TestConfig{
	Host: "localhost",
	Port: "50051",
}

func TestNewClient(t *testing.T) {
	client, err := NewClient(nil, testConfig.Host, testConfig.Port)
	if err != nil {
		t.FailNow()
	}
	if client.Creds != nil {
		t.Errorf("Creds should be nil")
	}
	if client.GRPCClient == nil {
		t.Errorf("gRPC client must be initialized but got nil")
	}
	if client.Host != testConfig.Host {
		t.Error("Hosts are not equal")
	}
	if client.Port != testConfig.Port {
		t.Error("Ports are not equal")
	}
}
