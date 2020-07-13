//nolint:unparam
// Package halmgr provides HAL based implementation of DriveManager
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

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// NewHALManager is the constructor of HALManager
// Receives only logrus logger
// Returns an instance of HALManager
func NewHALManager(logger *logrus.Logger) *HALManager {
	return &HALManager{Log: logger.WithField("component", "HALManager")}
}

// HALManager struct that implements DriveManager interface using HAL and cgo
type HALManager struct {
	Log *logrus.Entry
}

// convertDriveHealth converts C.DriveHealth enum got from HAL to api.Health var
// Receives var of enum C.DriveHealth type
// Returns var of string type (GOOD, SUSPECT, BAD, UNKNOWN)
func (mgr *HALManager) convertDriveHealth(driveHealth C.DriveHealth) string {
	switch driveHealth {
	case C.HEALTH_GOOD:
		// If HAL C.DriveHealth enum is equal to "GOOD"
		return apiV1.HealthGood
	case C.HEALTH_SUSPECT:
		// If HAL C.DriveHealth enum is equal to "SUSPECT"
		return apiV1.HealthSuspect
	case C.HEALTH_FAILED:
		// If HAL C.DriveHealth enum is equal to "FAILED"
		return apiV1.HealthBad
	default:
		return apiV1.HealthUnknown
	}
}

// convertDriveType converts HAL enum StorageClass_t var to string drive type to fill api.Drive struct
// Receives var of enum StorageClass_t type
// Returns string var of drive type (HDD, SSD, NVMe)
func (mgr *HALManager) convertDriveType(storageClass C.StorageClass_t) string {
	switch storageClass {
	case C.HDD:
		return apiV1.DriveTypeHDD
	case C.SSD:
		return apiV1.DriveTypeSSD
	case C.NVME:
		return apiV1.DriveTypeNVMe
	default:
		mgr.Log.Errorf("Can't recognize type of the drive. Use HDD as default value")
		return apiV1.DriveTypeHDD
	}
}

// GetDrivesList returns slice of *api.Drive created from HAL C.HalDisk
// Returns slice of *api.Drives struct or error if something went wrong
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
			Size:         util.ToBytes(int64(drivesSliceHAL[i].capacity), util.GBYTE),
			Health:       mgr.convertDriveHealth(drivesSliceHAL[i].driveHealth),
			Type:         mgr.convertDriveType(drivesSliceHAL[i].storageClass),
			Path:         C.GoString(&drivesSliceHAL[i].path[0]),
			Slot:         C.GoString(&drivesSliceHAL[i].slotName[0]),
		}
		drivesSlice = append(drivesSlice, drive)
	}

	for i, drive := range drivesSlice {
		mgr.Log.WithField("HalDrive", drive).Info("Drive ", i)
	}

	return drivesSlice, nil
}
