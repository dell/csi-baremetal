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

package mocks

import (
	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

// DriveMgrRespDrives are the drives for mock GetAllDrives call of DriveManager
var DriveMgrRespDrives = []*api.Drive{
	{
		SerialNumber: "hdd1",
		Health:       apiV1.MatchHealthStatus(apiV1.HealthGood),
		Type:         apiV1.MatchDriveType(apiV1.DriveTypeHDD),
		Size:         1024 * 1024 * 1024 * 50,
	},
	{
		SerialNumber: "hdd2",
		Health:       apiV1.MatchHealthStatus(apiV1.HealthGood),
		Type:         apiV1.MatchDriveType(apiV1.DriveTypeHDD),
		Size:         1024 * 1024 * 1024 * 150,
	},
}
