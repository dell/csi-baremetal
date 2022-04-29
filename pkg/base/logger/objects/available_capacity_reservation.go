package objects

import (
	"fmt"

	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
)

type availableCapacityReservation struct{}

func (l *availableCapacityReservation) Log(object *acrcrd.AvailableCapacityReservation) (str string) {
	return fmt.Sprintf("ID: '%s', Name: '%s', Labels: %+v, Annotations: %+v, Spec: %+v",
		object.ObjectMeta.UID, object.ObjectMeta.Name,
		object.ObjectMeta.Labels, object.ObjectMeta.Annotations, object.Spec)
}

func newAvailableCapacityReservation() *availableCapacityReservation {
	return &availableCapacityReservation{}
}
