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

// Locate invokes DriveManager's Locate method for manipulation drive's LED state
func (svc *DriveServiceServerImpl) Locate(ctx context.Context, in *api.DriveLocateRequest) (*api.DriveLocateResponse, error) {
	currentStatus, err := svc.mgr.Locate(in.GetDriveSerialNumber(), in.GetAction())
	if err != nil {
		svc.log.Errorf("Unable to locate device %s, action %d: %v", in.GetDriveSerialNumber(), in.GetAction(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &api.DriveLocateResponse{Status: currentStatus}, nil
}

// LocateNode invokes DriveManager's LocateNode method for manipulation node's LED state
func (svc *DriveServiceServerImpl) LocateNode(ctx context.Context, req *api.NodeLocateRequest) (*api.Empty, error) {
	err := svc.mgr.LocateNode(req.GetAction())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(api.Empty), nil
}

// GetDriveSmartInfo invokes DriveManager's GetDriveSmartInfo() and sends the response over gRPC
// Receives go context and SmartInfoRequest which contains Serial Number
// Returns SmartInfoResponse with smart info json string
func (svc *DriveServiceServerImpl) GetDriveSmartInfo(ctx context.Context, req *api.SmartInfoRequest) (*api.SmartInfoResponse, error) {
	smartInfo, err := svc.mgr.GetDriveSmartInfo(req.GetSerialNumber())
	if err != nil {
		svc.log.Errorf("DriveManager failed with error: %s", err.Error())
		return nil, err
	}
	return &api.SmartInfoResponse{
		SmartInfo: smartInfo,
	}, nil
}

// GetAllDrivesSmartInfo invokes DriveManager's GetAllDrivesSmartInfo() and sends the response over gRPC
// Receives go context and Empty message
// Returns SmartInfoResponse with smart info json string
func (svc *DriveServiceServerImpl) GetAllDrivesSmartInfo(ctx context.Context, req *api.Empty) (*api.SmartInfoResponse, error) {
	smartInfo, err := svc.mgr.GetAllDrivesSmartInfo()
	if err != nil {
		svc.log.Errorf("DriveManager failed with error: %s", err.Error())
		return nil, err
	}
	return &api.SmartInfoResponse{
		SmartInfo: smartInfo,
	}, nil
}
