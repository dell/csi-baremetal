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
	var err error
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
