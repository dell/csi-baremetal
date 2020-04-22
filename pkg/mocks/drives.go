package mocks

import api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

// HwMgrRespDrives are the drives for mock GetAllDrives call of HWManager
var HwMgrRespDrives = []*api.Drive{
	{
		SerialNumber: "hdd1",
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         1024 * 1024 * 1024 * 50,
	},
	{
		SerialNumber: "hdd2",
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         1024 * 1024 * 1024 * 150,
	},
}
