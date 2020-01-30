package base

import (
	"testing"
)

var testEndpoint string = "tcp://localhost:50051"

func TestNewClient(t *testing.T) {
	client, err := NewClient(nil, testEndpoint)
	if err != nil {
		t.FailNow()
	}
	if client.Creds != nil {
		t.Errorf("Creds should be nil")
	}
	if client.GRPCClient == nil {
		t.Errorf("gRPC client must be initialized but got nil")
	}
	if client.Endpoint != testEndpoint {
		t.Error("Endpoints are not equal")
	}
}

func TestClientClose(t *testing.T) {
	client, _ := NewClient(nil, testEndpoint)
	err := client.Close()
	if err != nil {
		t.Errorf("err should be nil, got %v", err)
	}
}
