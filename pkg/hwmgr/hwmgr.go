package hwmgr

import "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

type HWManager interface {
	GetDrivesList() ([]*v1api.Drive, error)
}
