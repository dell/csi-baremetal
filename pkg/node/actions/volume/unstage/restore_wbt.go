package unstage

import (
	"context"
	"github.com/dell/csi-baremetal/pkg/node/actions/volume/unstage/errors"

	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/node"
)

const (
	wbtChangedVolumeAnnotation = "wbt-changed"
	wbtChangedVolumeKey        = "yes"
)

type restoreWBT struct {
	volume        *volumecrd.Volume
	volumeManager *node.VolumeManager
}

func (r *restoreWBT) Handle(_ context.Context) (err error) {
	if val, ok := r.volume.Annotations[wbtChangedVolumeAnnotation]; ok && val == wbtChangedVolumeKey {
		delete(r.volume.Annotations, wbtChangedVolumeAnnotation)
		if err = r.volumeManager.RestoreWbtValue(r.volume); err != nil {
			return errors.NewRestoreWBTError(err.Error())
		}
	}

	return
}

func NewRestoreWBT(volume *volumecrd.Volume, volumeManager *node.VolumeManager) Action {
	return &restoreWBT{
		volume:        volume,
		volumeManager: volumeManager,
	}
}
