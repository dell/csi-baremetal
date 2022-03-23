package objects

import (
	"fmt"

	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
)

type logicalVolumeGroup struct{}

func (l *logicalVolumeGroup) Log(object *lvgcrd.LogicalVolumeGroup) (str string) {
	return fmt.Sprintf("ID: '%s', Name: '%s', Labels: %+v, Annotations: %+v, Spec: %+v",
		object.ObjectMeta.UID, object.ObjectMeta.Name,
		object.ObjectMeta.Labels, object.ObjectMeta.Annotations, object.Spec)
}

func newLogicalVolumeGroup() *logicalVolumeGroup {
	return &logicalVolumeGroup{}
}
