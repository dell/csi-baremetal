package base

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

// ConsistentRead returns content of the file and ensure that this content is actual (no one modify file during timeout)
// Receives absolute path to the file as filename, amount of retries to read and timeout of the operation
// Returns read file or error in case if there were not twice same read content
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

// ConvertStorageClass converts string from k8s StorageClass's manifest to api.StorageClass
// If it is impossible then use api.StorageClass_ANY
// Receives string name of StorageClass
// Returns var of api.StorageClass type
func ConvertStorageClass(strSC string) api.StorageClass {
	sc, ok := api.StorageClass_value[strings.ToUpper(strSC)]
	if !ok {
		sc = int32(api.StorageClass_ANY)
	}
	return api.StorageClass(sc)
}

// ConvertDriveTypeToStorageClass converts type of a drive to AvailableCapacity StorageClass
// Receives driveType var of api.DriveType type
// Returns var of api.StorageClass type
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
// Receives slice of strings and string to find
// Returns true if contains or false if not
func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// RemoveString removes string s from slice
// Receives slice of strings and string to remove
// Returns slice without mentioned string
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
