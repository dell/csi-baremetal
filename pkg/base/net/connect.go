package net

import (
	"net"
	"time"
)

const (
	numberOfRetries       = 4
	timeoutBetweenRetries = 500 * time.Millisecond
	tcp                   = "tcp"
)

// IsTCPPortOpen checks if TCP port is open
func IsTCPPortOpen(endpoint string) (bool, error) {
	return isPortOpen(tcp, endpoint)
}

// try to connect to provided endpoint with 4 attempts with 0.5 sec timeout
func isPortOpen(network, endpoint string) (bool, error) {
	// todo check network type and endpoint
	var err error = nil
	var conn net.Conn

	for i := 0; i < numberOfRetries; i++ {
		// check that port is open
		conn, err = net.DialTimeout(network, endpoint, time.Second)
		if err == nil && conn != nil {
			err = conn.Close()
			// don't handle error - just return it
			return true, err
		}
		// sleep before next try
		time.Sleep(timeoutBetweenRetries)
	}
	return false, err
}
