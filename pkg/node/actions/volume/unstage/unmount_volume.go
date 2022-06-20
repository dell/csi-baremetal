package unstage

import (
	"context"
	"github.com/dell/csi-baremetal/pkg/node/actions/volume/unstage/errors"

	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
)

const (
	fakeAttachVolumeAnnotation = "fake-attach"
	fakeAttachVolumeKey        = "yes"
)

type unmountVolume struct {
	volume            *volumecrd.Volume
	stagingTargetPath string
	fsOps             utilwrappers.FSOperations
}

func (r *unmountVolume) Handle(_ context.Context) (err error) {
	if r.volume.Annotations[fakeAttachVolumeAnnotation] != fakeAttachVolumeKey {
		if err = r.fsOps.UnmountWithCheck(r.stagingTargetPath); err == nil {
			err = r.fsOps.RmDir(r.stagingTargetPath)
		}
		if err != nil {
			return errors.NewUnmountVolumeError(err.Error())
		}
	}

	return
}

func NewUnmountVolume(volume *volumecrd.Volume, stagingTargetPath string, fsOps utilwrappers.FSOperations) Action {
	return &unmountVolume{
		volume:            volume,
		stagingTargetPath: stagingTargetPath,
		fsOps:             fsOps,
	}
}
