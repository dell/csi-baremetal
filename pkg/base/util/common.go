package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	api "github.com/dell/csi-baremetal/api/v1"
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

// ConvertStorageClass converts string from k8s StorageClass's manifest to CSI Storage Class string
// If it is impossible then use api.StorageClassAny
// Receives string name of StorageClass
// Returns string of CSI StorageClass
func ConvertStorageClass(strSC string) string {
	sc := strings.ToUpper(strSC)
	if sc == api.StorageClassHDD || sc == api.StorageClassSSD || sc == api.StorageClassNVMe ||
		sc == api.StorageClassHDDLVG || sc == api.StorageClassSSDLVG {
		return sc
	}

	return api.StorageClassAny
}

// ConvertDriveTypeToStorageClass converts type of a drive to AvailableCapacity StorageClass
// Receives driveType var of string type
// Returns string of Available Capacity StorageClass
func ConvertDriveTypeToStorageClass(driveType string) string {
	switch driveType {
	case api.DriveTypeHDD:
		return api.StorageClassHDD
	case api.DriveTypeSSD:
		return api.StorageClassSSD
	case api.DriveTypeNVMe:
		return api.StorageClassNVMe
	default:
		return api.StorageClassAny
	}
}

// GetSubStorageClass return appropriate underlying storage class for
// storage classes that are based on LVM, or empty string
func GetSubStorageClass(sc string) string {
	if sc == api.StorageClassHDDLVG {
		return api.StorageClassHDD
	} else if sc == api.StorageClassSSDLVG {
		return api.StorageClassSSD
	}
	return ""
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
