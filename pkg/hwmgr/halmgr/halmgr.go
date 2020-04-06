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
	"strings"
	"unsafe"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

func NewHALManager(logger *logrus.Logger) *HALManager {
	return &HALManager{Log: logger.WithField("component", "HALManager")}
}

type HALManager struct {
	Log *logrus.Entry
}

func (mgr *HALManager) convertDriveHealth(driveHealth C.DriveHealth) api.Health {
	switch driveHealth {
	case C.HEALTH_GOOD:
		// If HAL C.DriveHealth enum is equal to "GOOD"
		return api.Health_GOOD
	case C.HEALTH_SUSPECT:
		// If HAL C.DriveHealth enum is equal to "SUSPECT"
		return api.Health_SUSPECT
	case C.HEALTH_FAILED:
		// If HAL C.DriveHealth enum is equal to "FAILED"
		return api.Health_BAD
	default:
		return api.Health_UNKNOWN
	}
}

// Converts C_HAL enum StorageClass_t var to api.DriveType to fill api.Drive struct.
func (mgr *HALManager) convertDriveType(storageClass C.StorageClass_t) api.DriveType {
	switch storageClass {
	case C.HDD:
		return api.DriveType_HDD
	case C.SSD:
		return api.DriveType_SSD
	case C.NVME:
		return api.DriveType_NVMe
	default:
		mgr.Log.Errorf("Can't recognize type of the drive. Use HDD as default value")
		return api.DriveType_HDD
	}
}

func (mgr *HALManager) GetDrivesList() ([]*api.Drive, error) {
	var drivesHAL *C.HalDisk

	countHAL := C.int(0)
	res := C.getAllDrives(&drivesHAL, &countHAL) //nolint:gocritic

	if int(res.value) != 0 {
		errorMessage := C.GoString(&res.message[0])
		mgr.Log.Error("Hal failed with:", errorMessage)
		return nil, fmt.Errorf("hal failed with message %s", errorMessage)
	}

	defer C.free(unsafe.Pointer(drivesHAL))

	count := int(countHAL)

	mgr.Log.Infof("Found %d disks on the node", count)

	// Convert C-style array of C-style structs to go-slice of C-style structs
	drivesSliceHAL := (*[1 << 30]C.HalDisk)(unsafe.Pointer(drivesHAL))[:count:count]

	drivesSlice := make([]*api.Drive, 0, count)

	for i := 0; i < count; i++ {
		drive := &api.Drive{
			VID:          C.GoString(&drivesSliceHAL[i].vid[0]),
			PID:          C.GoString(&drivesSliceHAL[i].pid[0]),
			SerialNumber: strings.ToUpper(C.GoString(&drivesSliceHAL[i].serialNumber[0])),
			Size:         base.ToBytes(int64(drivesSliceHAL[i].capacity), base.GBYTE),
			Health:       mgr.convertDriveHealth(drivesSliceHAL[i].driveHealth),
			Type:         mgr.convertDriveType(drivesSliceHAL[i].storageClass),
			Path:         C.GoString(&drivesSliceHAL[i].path[0]),
		}
		drivesSlice = append(drivesSlice, drive)
	}

	for i, drive := range drivesSlice {
		mgr.Log.WithField("HalDrive", drive).Info("Drive ", i)
	}

	return drivesSlice, nil
}
