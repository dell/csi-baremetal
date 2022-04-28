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
	"testing"

	"gotest.tools/assert"

	"github.com/dell/csi-baremetal/pkg/base/baseerr"
)

func TestGetOSNameAndVersion(t *testing.T) {
	name, version, err := GetOSNameAndVersion("")
	assert.Equal(t, baseerr.ErrorEmptyParameter, err)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("Wrong OS")
	assert.Equal(t, err, baseerr.ErrorFailedParsing)
	assert.Equal(t, name, "")
	assert.Equal(t, version, "")

	name, version, err = GetOSNameAndVersion("12.04")
	assert.Equal(t, err, baseerr.ErrorFailedParsing)
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

	// OpenShift has the following output for OS Image
	name, version, err = GetOSNameAndVersion("Red Hat Enterprise Linux CoreOS 46.82.202101301821-0 (Ootpa)")
	assert.Equal(t, err, nil)
	assert.Equal(t, name, "red")
	assert.Equal(t, version, "46.82")
}

func TestGetKernelVersion(t *testing.T) {
	version, err := GetKernelVersion("")
	assert.Equal(t, baseerr.ErrorEmptyParameter, err)
	assert.Equal(t, version, "")

	version, err = GetKernelVersion("bla-bla")
	assert.Equal(t, baseerr.ErrorFailedParsing, err)
	assert.Equal(t, version, "")

	// ubuntu 19
	kernel := "5.4"
	version, err = GetKernelVersion(kernel + ".0-66-generic")
	assert.Equal(t, err, nil)
	assert.Equal(t, version, kernel)

	// ubuntu 18
	kernel = "4.15"
	version, err = GetKernelVersion(kernel + ".0-76-generic")
	assert.Equal(t, err, nil)
	assert.Equal(t, version, kernel)

	// rhel coreos 4.6
	kernel = "4.18"
	version, err = GetKernelVersion(kernel + ".0-193.41.1.el8_2.x86_64")
	assert.Equal(t, err, nil)
	assert.Equal(t, version, kernel)
}
