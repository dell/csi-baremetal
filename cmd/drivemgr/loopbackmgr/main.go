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

package main

import (
	"flag"
	"fmt"

	"github.com/fsnotify/fsnotify"

	dmsetup "github.com/dell/csi-baremetal/cmd/drivemgr"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/drivemgr/loopbackmgr"
)

var (
	endpoint = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "DriveManager Endpoint")
	logPath  = flag.String("logpath", "", "log path for DriveManager")
	logLevel = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
)

func main() {
	flag.Parse()

	logger, err := base.InitLogger(*logPath, *logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	// Server is insecure for now because credentials are nil
	serverRunner := rpc.NewServerRunner(nil, *endpoint, logger)

	e := &command.Executor{}
	e.SetLogger(logger)

	// creates a new file watcher for config
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatalf("Failed to create fs watcher: %v", err)
	}
	//nolint:errcheck
	defer watcher.Close()

	driveMgr := loopbackmgr.NewLoopBackManager(e, logger)

	go driveMgr.UpdateOnConfigChange(watcher)
	dmsetup.SetupAndRunDriveMgr(driveMgr, serverRunner, driveMgr.CleanupLoopDevices, logger)
}
