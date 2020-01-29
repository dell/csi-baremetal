package base

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

//Client encapsulates logic for new gRPC clint
type Client struct {
	GRPCClient *grpc.ClientConn
	Creds      credentials.TransportCredentials
	Host       string
	Port       int
}

//NewClient creates new Client object with host, port, creds and calls init function
func NewClient(creds credentials.TransportCredentials, host string, port int) (*Client, error) {
	client := &Client{
		Creds: creds,
		Host:  host,
		Port:  port,
	}
	err := client.initClient()
	if err != nil {
		return nil, err
	}
	return client, nil
}

//initClient defines ClientConn field in Client struct
func (c *Client) initClient() error {
	hostPort := fmt.Sprintf("%s:%d", c.Host, c.Port)
	var err error
	if c.Creds != nil {
		c.GRPCClient, err = grpc.Dial(hostPort, grpc.WithTransportCredentials(c.Creds))
	} else {
		c.GRPCClient, err = grpc.Dial(hostPort, grpc.WithInsecure())
	}
	if err != nil {
		return err
	}
	return nil
}

//Close function calls Close method in ClientConn
func (c *Client) Close() error {
	return c.GRPCClient.Close()
}
