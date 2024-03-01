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

// Package drivemgr contains a code for managers of storage hardware such as drives
package drivemgr

import api "github.com/dell/csi-baremetal/api/generated/v1"

// DriveManager is the interface for managers that provide information about drives on a node
type DriveManager interface {
	// GetDrivesList gets list of drives
	GetDrivesList() ([]*api.Drive, error)
	// Locate manipulates of drive's led state, receive drive serial number and type of action
	// returns current led status or error
	Locate(serialNumber string, action int32) (currentStatus int32, err error)
	// LocateNode manipulates of node's led state, which should be synced with drive's led
	LocateNode(action int32) error
	// GetSmartInfo gets smart info for specific drive with given serialNumber
	GetSmartInfo(serialNumber string) (string, error)
	// GetAllDrivesSmartInfo gets smart info for all drives on given node
	GetAllDrivesSmartInfo() (string, error)
}
