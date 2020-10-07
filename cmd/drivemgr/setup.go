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

// Package dmsetup has method for drivemgr initialization and startup
package dmsetup

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/drivemgr"
)

// SetupAndRunDriveMgr setups and start/stop particular drive manager
func SetupAndRunDriveMgr(d drivemgr.DriveManager, sr *rpc.ServerRunner, cleanupFn func(), logger *logrus.Logger) {
	logger.Info("Start DriveManager")

	driveServiceServer := drivemgr.NewDriveServer(logger, d)

	api.RegisterDriveServiceServer(sr.GRPCServer, &driveServiceServer)

	go util.SetupSIGHUPHandler(cleanupFn, logger)

	if err := sr.RunServer(); err != nil && err != grpc.ErrServerStopped {
		logger.Fatalf("Failed to serve on %s. Error: %v", sr.Endpoint, err)
	}

}
