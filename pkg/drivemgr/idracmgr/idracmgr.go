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

// Package idracmgr provides the iDRAC based implementation of DriveManager interface
package idracmgr

import "C"
import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

const (
	storageURL = "/redfish/v1/Systems/System.Embedded.1/Storage/"
	keyURL     = "@odata.id"
)

// IDRACManager is the struct that implements DriveManager interface using iDRAC inside
type IDRACManager struct {
	log      *logrus.Entry
	client   *http.Client
	ip       string
	user     string
	password string
}

// NewIDRACManager is the constructor of IDRACManager struct
// Receives logrus logger, timeout for HTTP client, user's credentials for iDRAC and iDRAC IP
// Returns an instance of IDRACManager
func NewIDRACManager(log *logrus.Logger, timeout time.Duration, user string, password string, ip string) *IDRACManager {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &IDRACManager{
		client:   &http.Client{Timeout: timeout, Transport: tr},
		ip:       ip,
		user:     user,
		password: password,
		log:      log.WithField("component", "IDRACManager"),
	}
}

// Storage contains urls of controller, enclosure etc, example @odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/NonRAID.Integrated.1-1
/*
...
Member: [{
	@odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/NonRAID.Integrated.1-1}
]
...
*/
type Storage struct {
	Member []map[string]string `json:"Members"`
}

// Controller contains urls of drives, example @odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/Drives/Disk.Bay.0:Enclosure.Internal.0-1:NonRAID.Integrated.1-1
/*
...
Drives: [{
 	@odata.id:/redfish/v1/Systems/System.Embedded.1/Storage/Drives/Disk.Bay.0:Enclosure.Internal.0-1:NonRAID.Integrated.1-1
}]
...
*/
type Controller struct {
	Drive []map[string]string `json:"Drives"`
}

// IDRACDrive contains info about drive got from iDRAC
type IDRACDrive struct {
	Status        map[string]string `json:"Status"`
	ID            string            `json:"Id"`
	SerialNumber  string            `json:"SerialNumber"`
	CapacityBytes int64             `json:"CapacityBytes"`
	MediaType     string            `json:"MediaType"`
	Manufacturer  string            `json:"Manufacturer"`
	Protocol      string            `json:"Protocol"`
	Model         string            `json:"Model"`
}

// GetDrivesList returns slice of *api.Drive created from iDRAC drives
// Returns slice of *api.Drives struct or error if something went wrong
func (mgr *IDRACManager) GetDrivesList() ([]*api.Drive, error) {
	controllerURL := mgr.getControllerURLs()
	if len(controllerURL) == 0 {
		return nil, errors.New("unable to inspect iDRAC controller")
	}
	drivesURL := make([]string, 0)
	for _, c := range controllerURL {
		drivesURL = append(drivesURL, mgr.getDrivesURLs(c)...)
	}
	if len(drivesURL) == 0 {
		return nil, errors.New("unable to inspect iDRAC drives")
	}
	drives := make([]*api.Drive, 0)
	for _, driveURL := range drivesURL {
		drive := mgr.getDrive(driveURL)
		if drive != nil {
			drives = append(drives, drive)
		}
	}
	return drives, nil
}

// Locate implements Locate method of DriveManager interface
func (mgr *IDRACManager) Locate(serialNumber string, action int32) (int32, error) {
	return -1, status.Error(codes.Unimplemented, "method Locate not implemented in IDRACManager")
}

// LocateNode implements LocateNode method of DriveManager interface
func (mgr *IDRACManager) LocateNode(action int32) error {
	// not implemented
	return status.Error(codes.Unimplemented, "method Locate not implemented in IDRACManager")
}

// GetSmartInfo implements GetSmartInfo method of DriveManager interface
func (mgr *IDRACManager) GetSmartInfo(serialNumber string) (string, error) {
	return "", status.Error(codes.Unimplemented, "method GetSmartInfo not implemented in BaseManager")
}

// GetAllDrivesSmartInfo implements GetAllDrivesSmartInfo method of DriveManager interface
func (mgr *IDRACManager) GetAllDrivesSmartInfo() (string, error) {
	// not implemented
	return "", status.Error(codes.Unimplemented, "method GetAllDrivesSmartInfo not implemented in BaseManager")
}

// getControllerURLs returns slice of all controllers url in Storage
func (mgr *IDRACManager) getControllerURLs() []string {
	endpoint := fmt.Sprintf("https://%s%s", mgr.ip, storageURL)
	controllerURLs := make([]string, 0)
	response, err := mgr.doRequest(endpoint)
	if err != nil {
		mgr.log.Errorf("Couldn't create storage request %s to IDRAC: %v", endpoint, err)
		return controllerURLs
	}
	if response != nil {
		defer func() {
			if err != response.Body.Close() {
				mgr.log.Errorf("Fail to close connection url: %s, err: %v", endpoint, err)
			}
		}()
	}
	var storage Storage
	if err := json.NewDecoder(response.Body).Decode(&storage); err != nil {
		mgr.log.Errorf("Fail to convert to storage struct, %v", err)
		return controllerURLs
	}
	for _, member := range storage.Member {
		// getting url of all controllers or enclosure in storage
		controllerURLs = append(controllerURLs, fmt.Sprintf("https://%s%s", mgr.ip, member[keyURL]))
	}
	return controllerURLs
}

// getDrivesURLs returns slice of all drive url in Controller with controllerURL
func (mgr *IDRACManager) getDrivesURLs(controllerURL string) []string {
	driveURLs := make([]string, 0)
	// get information about controllers using url from Storage
	response, err := mgr.doRequest(controllerURL)
	if response != nil {
		defer func() {
			if err != response.Body.Close() {
				mgr.log.Errorf("Fail to close connection url: %s, err: %v", controllerURL, err)
			}
		}()
	}
	if err != nil {
		mgr.log.Errorf("Couldn't create controller request %s to IDRAC: %v", controllerURL, err)
		return driveURLs
	}
	var controller Controller
	if err := json.NewDecoder(response.Body).Decode(&controller); err != nil {
		mgr.log.Errorf("Fail to convert to controller struct, %v", err)
		return driveURLs
	}
	for _, odata := range controller.Drive {
		// getting url of all drive in controller or enclosure
		driveURLs = append(driveURLs, fmt.Sprintf("https://%s%s", mgr.ip, odata[keyURL]))
	}
	return driveURLs
}

// getDrive returns api.Drive with information from IDRAC drive with url driveURL
func (mgr *IDRACManager) getDrive(driveURL string) *api.Drive {
	response, err := mgr.doRequest(driveURL)
	if err != nil {
		mgr.log.Errorf("Couldn't create drive %s request to IDRAC: %v", driveURL, err)
		return nil
	}
	if response != nil {
		defer func() {
			if err != response.Body.Close() {
				mgr.log.Errorf("Fail to close connection, url: %s, err: %v", driveURL, err)
			}
		}()
	}
	var drive IDRACDrive
	if err := json.NewDecoder(response.Body).Decode(&drive); err != nil {
		mgr.log.Errorf("Fail to convert to IDRACDrive struct, err: %v", err)
		return nil
	}
	var diskType string
	if drive.Protocol == "NVMe" {
		diskType = apiV1.DriveTypeNVMe
	} else {
		diskType = convertMediaType(drive.MediaType)
	}
	apiDrive := &api.Drive{
		VID:          drive.Manufacturer,
		PID:          drive.Model,
		SerialNumber: drive.SerialNumber,
		Health:       convertDriveHealth(drive.Status["Health"]),
		Type:         diskType,
		Size:         drive.CapacityBytes,
		Status:       apiV1.DriveStatusOnline,
	}
	return apiDrive
}

// doRequest performs HTTP GET request on provided url
// Receives url to request
// Returns *http.Response or error if something went wrong
func (mgr *IDRACManager) doRequest(url string) (*http.Response, error) {
	mgr.log.Info("Connecting to IDRAC with url ", url)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(mgr.user, mgr.password)
	request.Header.Add("Accept", "application/json")
	response, err := mgr.client.Do(request)
	if err != nil {
		return nil, err
	}
	return response, err
}

// convertDriveHealth converts iDRAC drives's health string to apiV1 Health string
// Receives iDRAC drives's health string
// Returns string variable (GOOD, BAD, UNKNOWN)
func convertDriveHealth(health string) string {
	switch health {
	case "OK":
		return apiV1.HealthGood
	// shouldn't it be SUSPECT?
	case "Warning":
		return apiV1.HealthBad
	case "Critical":
		return apiV1.HealthBad
	default:
		return apiV1.HealthUnknown
	}
}

// convertMediaType converts iDRAC drive's media type to drive type string var
// Receives iDRAC drive's media type
// Returns string variable of drive type (HDD, SSD)
func convertMediaType(mediaType string) string {
	switch mediaType {
	case "HDD":
		return apiV1.DriveTypeHDD
	case "SSD":
		return apiV1.DriveTypeSSD
	default:
		return apiV1.DriveTypeHDD
	}
}
