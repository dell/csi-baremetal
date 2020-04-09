package hwmgr

import api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

/*
List of supported implementations
*/
const (
	DEFAULT string = "HAL"
	REDFISH string = "IDRAC"
	TEST    string = "LOOPBACK"
)

type HWManager interface {
	// get list of drives
	GetDrivesList() ([]*api.Drive, error)
}
