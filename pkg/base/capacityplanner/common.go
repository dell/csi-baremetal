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

package capacityplanner

import "github.com/dell/csi-baremetal/pkg/base/util"

// AcSizeMinThresholdBytes means that if AC size becomes lower then AcSizeMinThresholdBytes that AC should be deleted
const AcSizeMinThresholdBytes = int64(util.MBYTE) // 1MB

// LvgDefaultMetadataSize is additional cost for new VG we should consider.
const LvgDefaultMetadataSize = int64(util.MBYTE) // 1MB

// DefaultPESize is the default extent size we should align with
// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
const DefaultPESize = 4 * int64(util.MBYTE)

// AlignSizeByPE make size aligned with default PE
// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
func AlignSizeByPE(size int64) int64 {
	var alignement int64
	reminder := size % DefaultPESize
	if reminder != 0 {
		alignement = DefaultPESize - reminder
	}
	return size + alignement
}
