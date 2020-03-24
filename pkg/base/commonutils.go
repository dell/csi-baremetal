package base

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

// ConsistentRead returns content of the file and ensure that
// this content is actual (no one modify file during timeout)
// in case if there were not twice same read content - return error
func ConsistentRead(filename string, retry int, timeout time.Duration) ([]byte, error) {
	oldContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	ticker := time.NewTicker(timeout)
	for i := 0; i < retry; i++ {
		<-ticker.C
		newContent, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(oldContent, newContent) {
			ticker.Stop()
			return newContent, nil
		}
		// Files are different, continue reading
		oldContent = newContent
	}
	ticker.Stop()
	return nil, fmt.Errorf("could not get consistent content of %s after %d attempts", filename, retry)
}

// Converts string from k8s StorageClass's manifest to api.StorageClass const.
// If it is impossible than use api.StorageClass_ANY
func ConvertStorageClass(strSC string) api.StorageClass {
	sc, ok := api.StorageClass_value[strings.ToUpper(strSC)]
	if !ok {
		sc = int32(api.StorageClass_ANY)
	}
	return api.StorageClass(sc)
}

// Temporary function to fill availableCapacity StorageClass based on its DriveType
func ConvertDriveTypeToStorageClass(driveType api.DriveType) api.StorageClass {
	switch driveType {
	case api.DriveType_HDD:
		return api.StorageClass_HDD
	case api.DriveType_SSD:
		return api.StorageClass_SSD
	case api.DriveType_NVMe:
		return api.StorageClass_NVME
	default:
		return api.StorageClass_ANY
	}
}

// ContainsString return true if slice contains string str
func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// RemoveString removes string s from slice
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
