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
	// get list of drives
	GetDrivesList() ([]*api.Drive, error)
}
