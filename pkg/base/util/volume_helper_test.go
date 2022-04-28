/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"testing"

	"gotest.tools/assert"
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
