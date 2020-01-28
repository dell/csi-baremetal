//nolint:unparam
package halmgr

// #cgo LDFLAGS: -L/opt/emc/hal/lib64 -lhalHelper -lviprhal
/*
	#include "/opt/emc/hal/include/chal/hal-helper.hxx"
	#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

type HALManager struct{}

func (mgr *HALManager) convertDriveHealth(driveHealth C.DriveHealth) api.Health {
	switch driveHealth {
	case C.HEALTH_GOOD:
		// If HAL C.DriveHealth enum is equal to "GOOD"
		return api.Health_GOOD
	case C.HEALTH_SUSPECT:
		// If HAL C.DriveHealth enum is equal to "SUSPECT"
		return api.Health_BAD
	case C.HEALTH_FAILED:
		// If HAL C.DriveHealth enum is equal to "FAILED"
		return api.Health_BAD
	default:
		return api.Health_UNKNOWN
	}
}

func (mgr *HALManager) GetDrivesList() ([]*api.Drive, error) {
	var drivesHAL *C.HalDisk

	countHAL := C.int(0)
	res := C.getAllDrives(&drivesHAL, &countHAL) //nolint:gocritic

	if int(res.value) != 0 {
		errorMessage := C.GoString(&res.message[0])
		logrus.Error("Hal failed with:", errorMessage)
		return nil, fmt.Errorf("hal failed with message %s", errorMessage)
	}

	defer C.free(unsafe.Pointer(drivesHAL))

	count := int(countHAL)

	logrus.Infof("Found %d disks on the node", count)

	// Convert C-style array of C-style structs to go-slice of C-style structs
	drivesSliceHAL := (*[1 << 30]C.HalDisk)(unsafe.Pointer(drivesHAL))[:count:count]

	drivesSlice := make([]*api.Drive, 0, count)

	for i := 0; i < count; i++ {
		drive := &api.Drive{
			VID:          C.GoString(&drivesSliceHAL[i].vid[0]),
			PID:          C.GoString(&drivesSliceHAL[i].pid[0]),
			SerialNumber: C.GoString(&drivesSliceHAL[i].serialNumber[0]),
			Size:         base.ToBytes(uint64(drivesSliceHAL[i].capacity), base.GBYTE),
			Health:       mgr.convertDriveHealth(drivesSliceHAL[i].driveHealth),
		}
		drivesSlice = append(drivesSlice, drive)
	}

	for i, drive := range drivesSlice {
		logrus.WithField("HalDrive", drive).Info("Drive ", i)
	}

	return drivesSlice, nil
}
