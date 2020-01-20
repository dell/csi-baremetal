package hwmgr

import api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

type HWManager interface {
	GetDrivesList() ([]*api.Drive, error)
}
