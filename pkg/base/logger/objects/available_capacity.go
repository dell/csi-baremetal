package objects

import (
	"fmt"

	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
)

type availableCapacity struct{}

func (l *availableCapacity) Log(object *accrd.AvailableCapacity) (str string) {
	return fmt.Sprintf("Labels: %+v, Annotations: %+v, Spec: %+v",
		object.ObjectMeta.Labels, object.ObjectMeta.Annotations, object.Spec)
}

func newAvailableCapacity() *availableCapacity {
	return &availableCapacity{}
}
