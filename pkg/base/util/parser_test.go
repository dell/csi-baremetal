/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
	"gotest.tools/assert"
	"testing"
)

func TestGetOSNameAndVersion(t *testing.T) {
	name, version, err := GetOSNameAndVersion("")
	assert.Equal(t, errTypes.ErrorEmptyParameter, err)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("Wrong OS")
	assert.Equal(t, err, errTypes.ErrorFailedParsing)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("12.04")
	assert.Equal(t, err, errTypes.ErrorFailedParsing)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("Ubuntu 18.04.4 LTS")
	assert.Equal(t, err, nil)
	assert.Equal(t, name, "ubuntu")
	assert.Equal(t, version, "18.04")

	name, version, err = GetOSNameAndVersion("Ubuntu 19.10")
	assert.Equal(t, err, nil)
	assert.Equal(t, name, "ubuntu")
	assert.Equal(t, version, "19.10")
}
