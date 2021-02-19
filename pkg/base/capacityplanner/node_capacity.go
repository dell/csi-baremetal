/*
 * Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *   See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package capacityplanner

import (
	"math"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

type nodeCapacity struct {
	// AC cache
	capacity ACMap
	// store original versions of modified ACs
	origAC ACMap
}

// registerAC register AC in internal cache
func (nc *nodeCapacity) registerAC(ac *accrd.AvailableCapacity) {
	nc.capacity[ac.Name] = ac
}

func (nc *nodeCapacity) saveOriginalAC(ac *accrd.AvailableCapacity) {
	acInCache, ok := nc.capacity[ac.Name]
	if !ok {
		return
	}
	if nc.origAC == nil {
		nc.origAC = ACMap{}
	}
	// we need to save original AC only once
	if _, ok := nc.origAC[ac.Name]; !ok {
		nc.origAC[ac.Name] = acInCache.DeepCopy()
	}
}

func (nc *nodeCapacity) getOriginalAC(acName string) *accrd.AvailableCapacity {
	if ac, ok := nc.origAC[acName]; ok {
		return ac
	}
	if ac, ok := nc.capacity[acName]; ok {
		return ac
	}
	return nil
}

// removeAC remove AC from internal cache
func (nc *nodeCapacity) removeAC(ac *accrd.AvailableCapacity) {
	delete(nc.capacity, ac.Name)
}

// selectACForVolume select AC for volume
// will modify nodeCapacity AC cache
func (nc *nodeCapacity) selectACForVolume(vol *genV1.Volume) *accrd.AvailableCapacity {
	if util.IsStorageClassLVG(vol.StorageClass) {
		return nc.selectACForLVMVolume(vol)
	}
	return nc.selectACForFullDriveVolume(vol)
}

// selectACForFullDriveVolume selects AC for ANY SC or for other "full drive" storage classes.
func (nc *nodeCapacity) selectACForFullDriveVolume(vol *genV1.Volume) *accrd.AvailableCapacity {
	scToACMap := nc.getStorageClassToACMapping()

	if len(scToACMap[vol.StorageClass]) == 0 &&
		vol.StorageClass != v1.StorageClassAny {
		return nil
	}
	requiredSize := vol.GetSize()
	// filter out non relevant storage classes
	filteredMap := SCToACMap{}
	if vol.StorageClass == v1.StorageClassAny {
		for sc, acs := range scToACMap {
			// for any SC we need to check for non LVG only
			if !util.IsStorageClassLVG(sc) {
				// TODO Take into account drive technology for SC ANY https://github.com/dell/csi-baremetal/issues/231
				// map must be sorted HDD->SSD->NVMe
				filteredMap[sc] = acs
			}
		}
	} else {
		filteredMap[vol.StorageClass] = scToACMap[vol.StorageClass]
	}
	// try to find free capacity
	var foundAC *accrd.AvailableCapacity
	for _, acs := range filteredMap {
		foundAC = searchACWithClosestSize(acs, requiredSize, nil)
		if foundAC != nil {
			break
		}
	}
	// return if available capacity not found
	if foundAC == nil {
		return nil
	}
	nc.saveOriginalAC(foundAC)
	// mark AC as used
	nc.removeAC(foundAC)
	return nc.getOriginalAC(foundAC.Name)
}

// selectACForLVMVolume selects AC for Volume with LVM SC
// first we try to find AC with LVM AC, if not found full drive AC will be converted to LVM AC
func (nc *nodeCapacity) selectACForLVMVolume(vol *genV1.Volume) *accrd.AvailableCapacity {
	// extract drive technology - HDD,SSD, etc.
	subSC := util.GetSubStorageClass(vol.StorageClass)

	scToACMap := nc.getStorageClassToACMapping()
	if len(scToACMap[vol.StorageClass]) == 0 &&
		len(scToACMap[subSC]) == 0 {
		return nil
	}

	// we should round up volume size, it should be aligned with LVM PE size
	// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
	requiredSize := AlignSizeByPE(vol.GetSize())

	// try to find free capacity with StorageClass from volume creation request
	foundAC := searchACWithClosestSize(scToACMap[vol.StorageClass], requiredSize, nil)

	// for LVG SC try to reserve AC to create new LVG since no free space found on existing
	if foundAC == nil {
		// search AC in sub storage class
		foundAC = searchACWithClosestSize(scToACMap[subSC], requiredSize, SubtractLVMMetadataSize)
	}
	// return if available capacity not found
	if foundAC == nil {
		return nil
	}
	// we should return unmodified AC, but for planning the next volumes we should do a AC resource modification
	// here we save original AC for future use
	nc.saveOriginalAC(foundAC)

	if foundAC.Spec.StorageClass != vol.StorageClass { // sc relates to LVG or sc == ANY
		foundAC.Spec.StorageClass = vol.StorageClass // e.g. HDD -> HDDLVG
		foundAC.Spec.Size = SubtractLVMMetadataSize(foundAC.Spec.Size)
	}
	foundAC.Spec.Size -= requiredSize

	return nc.getOriginalAC(foundAC.Name)
}

func (nc *nodeCapacity) getStorageClassToACMapping() SCToACMap {
	result := SCToACMap{}
	for _, ac := range nc.capacity {
		if _, ok := result[ac.Spec.StorageClass]; !ok {
			result[ac.Spec.StorageClass] = map[string]*accrd.AvailableCapacity{}
		}
		result[ac.Spec.StorageClass][ac.Name] = ac
	}
	return result
}

func searchACWithClosestSize(acs ACMap, size int64, sizeRoundFunc func(int64) int64) *accrd.AvailableCapacity {
	var (
		maxSize  int64 = math.MaxInt64
		pickedAC *accrd.AvailableCapacity
	)

	for _, ac := range acs {
		acSize := ac.Spec.Size
		if sizeRoundFunc != nil {
			acSize = sizeRoundFunc(acSize)
		}
		if acSize >= size && acSize < maxSize {
			pickedAC = ac
			maxSize = acSize
		}
	}
	return pickedAC
}
