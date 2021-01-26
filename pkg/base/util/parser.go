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
	"regexp"
	"strings"

	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
)

// GetOSNameAndVersion receives string with the OS information in th following format:
// "<OS name> <OS version> <Extra information>". For example, "Ubuntu 18.04.4 LTS"
// returns os name with the lower case and major and minor version. For example, "ubuntu", "18.04"
func GetOSNameAndVersion(osInfo string) (name, version string, err error) {
	// check input parameter
	if len(osInfo) == 0 {
		return "", "", errTypes.ErrorEmptyParameter
	}

	// extract OS name
	name = regexp.MustCompile(`^[A-Za-z]+`).FindString(osInfo)
	if len(name) == 0 {
		return "", "", errTypes.ErrorFailedParsing
	}

	// extract OS version
	//version = regexp.MustCompile("[0-9]+\\.[0-9]+").FindString(osInfo)
	version = regexp.MustCompile(`[0-9]+\.[0-9]+`).FindString(osInfo)
	if len(version) == 0 {
		return "", "", errTypes.ErrorFailedParsing
	}

	return strings.ToLower(name), version, nil
}
