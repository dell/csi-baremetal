package volume

import (
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/node"
	"github.com/dell/csi-baremetal/pkg/node/actions"
	"github.com/dell/csi-baremetal/pkg/node/actions/volume/unstage"
	"github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
)

type Factory interface {
	CreateUnstageVolumeActions(volume *volumecrd.Volume, stagingTargetPath string) actions.Actions
}

type factory struct {
	volumeManager *node.VolumeManager
	fsOps         utilwrappers.FSOperations
}

func (f *factory) CreateUnstageVolumeActions(volume *volumecrd.Volume, stagingTargetPath string) actions.Actions {
	return actions.Actions{
		unstage.NewUnmountVolume(volume, stagingTargetPath, f.fsOps),
		unstage.NewRestoreWBT(volume, f.volumeManager),
	}
}

func NewVolumeFactory(volumeManager *node.VolumeManager, fsOps utilwrappers.FSOperations) Factory {
	return &factory{
		volumeManager: volumeManager,
		fsOps:         fsOps,
	}
}
