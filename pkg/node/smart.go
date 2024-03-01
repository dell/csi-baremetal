/*
Copyright © 2040 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/go-faster/jx"
	api "github.com/dell/csi-baremetal/api/generated/v1"
	smart "github.com/dell/csi-baremetal/pkg/node/smart/generated"
)

// SmartService represents smart API handler
type SmartService struct {
	mux      sync.Mutex
	client   api.DriveServiceClient
	log      *logrus.Entry
}

// NewSmartService is the constructor for CmartService struct
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
	s.mux.Lock()
	defer s.mux.Unlock()

	smartInfoResponse, err := s.client.GetAllDrivesSmartInfo(context.Background(), &api.Empty{})
	if err != nil {
		s.log.Errorf("Drivemgr response failure: %v", err)
		return &smart.GetAllDrivesSmartInfoNotFound{}, nil
	}

	s.log.Debugf("Drivemgr response %v ", smartInfoResponse)
	response := smart.SmartMetrics(jx.Raw(smartInfoResponse.GetSmartInfo()))
	return &response, nil
}

// GetSmartInfo implements get-smart-info operation.
//
// Retrieve the disk information/metrics with the matching serial number.
//
// GET /smart/{serialNumber}
func (s *SmartService) GetSmartInfo(ctx context.Context, params smart.GetSmartInfoParams) (smart.GetSmartInfoRes, error) {
	s.mux.Lock()
	defer s.mux.Unlock()

	smartInfoResponse, err := s.client.GetSmartInfo(context.Background(), &api.SmartInfoRequest{SerialNumber: params.SerialNumber})
	if err != nil {
		s.log.Errorf("Drivemgr response failure: %v", err)
		return &smart.GetSmartInfoNotFound{}, nil
	}

	s.log.Debugf("Drivemgr response %v ", smartInfoResponse)
	response := smart.SmartMetrics(jx.Raw(smartInfoResponse.GetSmartInfo()))
	return &response, nil
}
