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

package util

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/dell/csi-baremetal/pkg/base/rpc"
)

// SetupSIGTERMHandler try to shutdown service, when SIGTERM is caught
func SetupSIGTERMHandler(server *rpc.ServerRunner) {
	setupSignalHandler(syscall.SIGTERM)
	// We received an interrupt signal, shut down.
	server.StopServer()
}

// SetupSIGHUPHandler try to make cleanup, when SIGHUP is caught
func SetupSIGHUPHandler(cleanupFn func()) {
	setupSignalHandler(syscall.SIGHUP)
	// We received an SIGHUP signal, clean up.
	if cleanupFn != nil {
		cleanupFn()
	}
}

// setupSignalHandler set up channel for signal
func setupSignalHandler(sig syscall.Signal) {
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, sig)

	//Wait signal
	<-signalChan
}
