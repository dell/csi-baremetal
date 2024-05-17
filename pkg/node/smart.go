/*
Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.

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

// Package node contains implementation of CSI Node component
package node

import (
	"context"
	"encoding/json"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	smart "github.com/dell/csi-baremetal/api/smart/generated"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SmartService represents smart API handler
type SmartService struct {
	client api.DriveServiceClient
	log    *logrus.Entry
}

// NewSmartService is the constructor for SmartService struct
// Receives query path prefix and handles incomming HTTP requests
// Returns an instance of SmartService
func NewSmartService(client api.DriveServiceClient, logger *logrus.Logger) *SmartService {
	s := &SmartService{
		client: client,
		log:    logger.WithField("component", "SmartService"),
	}
	return s
}

// GetAllDrivesSmartInfo implements get-all-drives-smart-info operation.
//
// Retrieve all discovered disks information/metrics.
//
// GET /smart
func (s *SmartService) GetAllDrivesSmartInfo(ctx context.Context) (smart.GetAllDrivesSmartInfoRes, error) {
	smartInfoResponse, err := s.client.GetAllDrivesSmartInfo(context.Background(), &api.Empty{})
	if err != nil {
		s.log.Errorf("Drivemgr response failure: %v", err)

		if code, ok := status.FromError(err); ok {
			switch code.Code() {
			case codes.NotFound:
				return &smart.GetAllDrivesSmartInfoNotFound{}, nil
			case codes.Unimplemented:
				return &smart.GetAllDrivesSmartInfoBadRequest{}, nil
			default:
				s.log.Errorf("Unsupported GRPC code: %d", code.Code())
			}
		}
		return &smart.GetAllDrivesSmartInfoInternalServerError{}, nil
	}

	s.log.Debugf("Drivemgr response %v ", smartInfoResponse)

	if !s.isValidSmartInfoJSON(smartInfoResponse) {
		return &smart.GetAllDrivesSmartInfoInternalServerError{}, nil
	}

	var smartInfo smart.OptString
	smartInfo.SetTo(smartInfoResponse.GetSmartInfo())
	response := smart.SmartMetrics{SmartInfo: smartInfo}
	return &response, nil
}

// GetDriveSmartInfo implements get-drive-smart-info operation.
//
// Retrieve the disk information/metrics with the matching serial number.
//
// GET /smart/{serialNumber}
func (s *SmartService) GetDriveSmartInfo(ctx context.Context, params smart.GetDriveSmartInfoParams) (smart.GetDriveSmartInfoRes, error) {
	smartInfoResponse, err := s.client.GetDriveSmartInfo(context.Background(), &api.SmartInfoRequest{SerialNumber: params.SerialNumber})
	if err != nil {
		s.log.Errorf("Drivemgr response failure: %v", err)

		if code, ok := status.FromError(err); ok {
			switch code.Code() {
			case codes.NotFound:
				return &smart.GetDriveSmartInfoNotFound{}, nil
			case codes.Unimplemented:
				return &smart.GetDriveSmartInfoBadRequest{}, nil
			default:
				s.log.Errorf("Unsupported GRPC code: %d", code.Code())
			}
		}
		return &smart.GetDriveSmartInfoInternalServerError{}, nil
	}

	s.log.Debugf("Drivemgr response %v ", smartInfoResponse)

	if !s.isValidSmartInfoJSON(smartInfoResponse) {
		return &smart.GetDriveSmartInfoInternalServerError{}, nil
	}

	var smartInfo smart.OptString
	smartInfo.SetTo(smartInfoResponse.GetSmartInfo())
	response := smart.SmartMetrics{SmartInfo: smartInfo}
	return &response, nil
}

// isValidSmartInfoJSON checks if the given SmartInfoResponse contains a valid JSON string.
//
// Parameters:
// - smartInfoResponse: a pointer to an api.SmartInfoResponse object.
//
// Return:
// - bool: true if the response contains a valid JSON string, false otherwise.
func (s *SmartService) isValidSmartInfoJSON(smartInfoResponse *api.SmartInfoResponse) bool {
	response := smartInfoResponse.GetSmartInfo()
	if !json.Valid([]byte(response)) {
		s.log.Errorf("Invalid Smart Info JSON response from drivemgr: %v", response)
		return false
	}
	return true
}
