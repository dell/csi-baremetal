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

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/rpc"
)

// SignalHandler is a structure which contains methods for signal handling
type SignalHandler struct {
	log *logrus.Entry
}

// NewSignalHandler is a constructor for SignalHandler
func NewSignalHandler(logger *logrus.Logger) *SignalHandler {
	return &SignalHandler{log: logger.WithField("component", "SignalHandler")}
}

// SetupSIGTERMHandler tries to shutdown service, when SIGTERM is caught
func (sh *SignalHandler) SetupSIGTERMHandler(server *rpc.ServerRunner) {
	sh.setupSignalHandler(syscall.SIGTERM)
	// We received an interrupt signal, shut down.
	server.StopServer()
}

// SetupSIGHUPHandler tries to make cleanup, when SIGHUP is caught
func (sh *SignalHandler) SetupSIGHUPHandler(cleanupFn func()) {
	sh.setupSignalHandler(syscall.SIGHUP)
	// We received an SIGHUP signal, clean up.
	if cleanupFn != nil {
		cleanupFn()
	}
}

// setupSignalHandler sets up channel for signal
func (sh *SignalHandler) setupSignalHandler(sig syscall.Signal) {
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, sig)

	//Wait signal
	<-signalChan

	sh.log.WithField("method", "setupSignalHandler").Debugf("Got %v signal", sig)
}
