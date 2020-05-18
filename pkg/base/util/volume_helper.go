// Package util contains common utilities
package util

import (
	"errors"
	"strings"
)

const prefix = "pvc-"

// GetVolumeUUID extracts UUID from volume ID: pvc-<UUID>
// Method will remove prefix `pvc-` and return UUID
func GetVolumeUUID(volumeID string) (string, error) {
	// check that volume ID is correct
	if volumeID == "" {
		return "", errors.New("volume ID is empty")
	}

	// trim prefix
	uuid := strings.TrimPrefix(volumeID, prefix)
	// return error if volume UUID is empty
	if uuid == "" {
		return "", errors.New("volume UUID is empty")
	}
	// is PV UUID RFC 4122 compatible?
	return uuid, nil
}
