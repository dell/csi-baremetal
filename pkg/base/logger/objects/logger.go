package objects

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

const (
	acKind     = "AvailableCapacity"
	acrKind    = "AvailableCapacityKind"
	driveKind  = "Drive"
	lvgKind    = "LogicalVolumeGroup"
	nodeKind   = "Node"
	volumeKind = "Volume"
)

// ObjectLogger is center logging object for CSI Driver crd objects
type ObjectLogger interface {
	Log(runtime.Object) string
}

type objectLogger struct {
	acLogger     *availableCapacity
	acrLogger    *availableCapacityReservation
	driveLogger  *drive
	lvgLogger    *logicalVolumeGroup
	nodeLogger   *node
	volumeLogger *volume
}

func (l *objectLogger) Log(object runtime.Object) string {
	switch kind := object.GetObjectKind().GroupVersionKind().Kind; kind {
	case acKind:
		return l.acLogger.Log(object.(*accrd.AvailableCapacity))
	case acrKind:
		return l.acrLogger.Log(object.(*acrcrd.AvailableCapacityReservation))
	case driveKind:
		return l.driveLogger.Log(object.(*drivecrd.Drive))
	case lvgKind:
		return l.lvgLogger.Log(object.(*lvgcrd.LogicalVolumeGroup))
	case nodeKind:
		return l.nodeLogger.Log(object.(*nodecrd.Node))
	case volumeKind:
		return l.volumeLogger.Log(object.(*volumecrd.Volume))
	}
	return fmt.Sprintf("%+v", object)
}

// NewObjectLogger is the constructor for ObjectLogger
func NewObjectLogger() ObjectLogger {
	return &objectLogger{
		acLogger:     newAvailableCapacity(),
		acrLogger:    newAvailableCapacityReservation(),
		driveLogger:  newDrive(),
		lvgLogger:    newLogicalVolumeGroup(),
		nodeLogger:   newNode(),
		volumeLogger: newVolume(),
	}
}
