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

package drivemgr

import (
	"context"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

// DriveServiceServerImpl is the implementation of gRPC server that gives possibility to invoke DriveManager's methods
// remotely
type DriveServiceServerImpl struct {
	mgr DriveManager
	log *logrus.Entry
}

// NewDriveServer is the constructor for DriveServiceServerImpl struct
// Receives logrus logger and implementation of DriveManager as parameters
// Returns an instance of DriveServiceServerImpl
func NewDriveServer(logger *logrus.Logger, manager DriveManager) DriveServiceServerImpl {
	driveService := DriveServiceServerImpl{
		log: logger.WithField("component", "DriveServiceServerImpl"),
		mgr: manager,
	}
	return driveService
}

// GetDrivesList invokes DriveManager's GetDrivesList() and sends the response over gRPC
// Receives go context and DrivesRequest which contains node id
// Returns DrivesResponse with slice of api.Drives structs
func (svc *DriveServiceServerImpl) GetDrivesList(ctx context.Context, req *api.DrivesRequest) (*api.DrivesResponse, error) {
	drives, err := svc.mgr.GetDrivesList()
	if err != nil {
		svc.log.Errorf("DriveManager failed with error: %s", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	// All drives are ONLINE by default
	for _, drive := range drives {
		drive.NodeId = req.NodeId
		if drive.Status == "" {
			drive.Status = apiV1.DriveStatusOnline
		}
	}
	return &api.DrivesResponse{
		Disks: drives,
	}, nil
}

func (svc *DriveServiceServerImpl) Locate(ctx context.Context, in *api.DriveLocateRequest) (*api.DriveLocateResponse, error) {
	err := svc.mgr.Locate(in.GetDriveSerialNumber(), in.GetAction())
	if err != nil {
		svc.log.Errorf("Unable to locate device %s, action %s: %v", in.GetDriveSerialNumber(), in.GetAction(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &api.DriveLocateResponse{}, nil
}
