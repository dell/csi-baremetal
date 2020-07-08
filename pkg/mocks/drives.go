package mocks

import (
	api "github.com/dell/csi-baremetal.git/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal.git/api/v1"
)

// DriveMgrRespDrives are the drives for mock GetAllDrives call of DriveManager
var DriveMgrRespDrives = []*api.Drive{
	{
		SerialNumber: "hdd1",
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024 * 1024 * 50,
	},
	{
		SerialNumber: "hdd2",
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024 * 1024 * 150,
	},
}
