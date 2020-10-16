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
	subSC := util.GetSubStorageClass(vol.StorageClass)
	isLVM := util.IsStorageClassLVG(vol.StorageClass)

	scM := nc.getStorageClassToACMapping()
	if len(scM[vol.StorageClass]) == 0 &&
		len(scM[subSC]) == 0 &&
		vol.StorageClass != v1.StorageClassAny {
		return nil
	}
	// make copy for temp transformations
	size := vol.GetSize()

	if isLVM {
		// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
		size = AlignSizeByPE(size)
	}
	var ac *accrd.AvailableCapacity
	ac = searchACWithClosestSize(scM[vol.StorageClass], size)
	if ac == nil {
		if isLVM {
			// for the new lvg we need some extra space
			size += LvgDefaultMetadataSize
			// search AC in sub storage class
			ac = searchACWithClosestSize(scM[subSC], size)
		} else if vol.StorageClass == v1.StorageClassAny {
			for _, acs := range scM {
				ac = searchACWithClosestSize(acs, size)
				if ac != nil {
					break
				}
			}
		}
	}
	if ac == nil {
		return nil
	}
	nc.saveOriginalAC(ac)
	if ac.Spec.StorageClass != vol.StorageClass { // sc relates to LVG or sc == ANY
		if util.IsStorageClassLVG(ac.Spec.StorageClass) || isLVM {
			if isLVM {
				ac.Spec.StorageClass = vol.StorageClass // e.g. HDD -> HDDLVG
			}
			ac.Spec.Size -= size
		} else {
			// sc == ANY && ac.Spec.StorageClass doesn't relate to LVG
			nc.removeAC(ac)
		}
	} else {
		if isLVM {
			ac.Spec.Size -= size
		} else {
			nc.removeAC(ac)
		}
	}
	return nc.getOriginalAC(ac.Name)
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
