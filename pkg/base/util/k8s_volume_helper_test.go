package util

import (
	"gotest.tools/assert"
	"testing"
)

func Test_GetVolumeUUID(t *testing.T) {
	uuid := "84804065-9723-4954-a6ae-5e38769c9b2f"
	volumeID := "pvc-" + uuid

	test, err := GetVolumeUUID(volumeID)
	assert.Equal(t, uuid, test)
	assert.NilError(t, err)
}

func Test_GetEmptyVolumeID(t *testing.T) {
	volumeID := ""
	_, err := GetVolumeUUID(volumeID)
	assert.Error(t, err, "volume ID is empty")
}

func Test_GetEmptyVolumeUUID(t *testing.T) {
	volumeID := "pvc-"
	_, err := GetVolumeUUID(volumeID)
	assert.Error(t, err, "volume UUID is empty")
}
