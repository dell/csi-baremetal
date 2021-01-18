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
	// extract drive technology - HDD,SSD, etc.
	subSC := util.GetSubStorageClass(vol.StorageClass)
	// check if LVG SC
	isLVG := util.IsStorageClassLVG(vol.StorageClass)

	// build SC to Name->AC map and check for free capacity
	scToACMap := nc.getStorageClassToACMapping()
	if len(scToACMap[vol.StorageClass]) == 0 &&
		len(scToACMap[subSC]) == 0 &&
		vol.StorageClass != v1.StorageClassAny {
		return nil
	}

	// make copy for temp transformations
	size := vol.GetSize()
	if isLVG {
		// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
		size = AlignSizeByPE(size)
	}

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
		foundAC = searchACWithClosestSize(acs, size)
		if foundAC != nil {
			break
		}
	}
	// for LVG SC try to reserve AC to create new LVG since no free space found on existing
	if isLVG && foundAC == nil {
		// for the new lvg we need some extra space
		size += LvgDefaultMetadataSize
		// search AC in sub storage class
		foundAC = searchACWithClosestSize(scToACMap[subSC], size)
	}

	// return if available capacity not found
	if foundAC == nil {
		return nil
	}

	nc.saveOriginalAC(foundAC)
	if foundAC.Spec.StorageClass != vol.StorageClass { // sc relates to LVG or sc == ANY
		if util.IsStorageClassLVG(foundAC.Spec.StorageClass) || isLVG {
			if isLVG {
				foundAC.Spec.StorageClass = vol.StorageClass // e.g. HDD -> HDDLVG
			}
			foundAC.Spec.Size -= size
		} else {
			// sc == ANY && ac.Spec.StorageClass doesn't relate to LVG
			nc.removeAC(foundAC)
		}
	} else {
		if isLVG {
			foundAC.Spec.Size -= size
		} else {
			nc.removeAC(foundAC)
		}
	}
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

func searchACWithClosestSize(acs ACMap, size int64) *accrd.AvailableCapacity {
	var (
		maxSize  int64 = math.MaxInt64
		pickedAC *accrd.AvailableCapacity
	)

	for _, ac := range acs {
		if ac.Spec.Size >= size && ac.Spec.Size < maxSize {
			pickedAC = ac
			maxSize = ac.Spec.Size
		}
	}
	return pickedAC
}
