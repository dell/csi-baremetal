package objects

import (
	"fmt"

	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

type volume struct{}

func (l *volume) Log(object *volumecrd.Volume) (str string) {
	return fmt.Sprintf("ID: '%s', Name: '%s', Namespace: '%s', Labels: %+v, Annotations: %+v, Spec: %+v",
		object.ObjectMeta.UID, object.ObjectMeta.Name, object.ObjectMeta.Namespace,
		object.ObjectMeta.Labels, object.ObjectMeta.Annotations, object.Spec)
}

func newVolume() *volume {
	return &volume{}
}
