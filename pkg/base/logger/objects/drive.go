package objects

import (
	"fmt"

	"github.com/dell/csi-baremetal/api/v1/drivecrd"
)

type drive struct{}

func (l *drive) Log(object *drivecrd.Drive) (str string) {
	return fmt.Sprintf("ID: '%s', Name: '%s', Labels: %+v, Annotations: %+v, Spec: %+v",
		object.ObjectMeta.UID, object.ObjectMeta.Name,
		object.ObjectMeta.Labels, object.ObjectMeta.Annotations, object.Spec)
}

func newDrive() *drive {
	return &drive{}
}
