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
	"sort"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

type reservedCapacity struct {
	Size         int64
	StorageClass string
}

type reservedACs map[string]*reservedCapacity

type scToACOrder map[string][]string

type nodeCapacity struct {
	node        string
	acs         ACMap
	acsOrder    scToACOrder
	reservedACs reservedACs
}

func newNodeCapacity(node string, acs []accrd.AvailableCapacity, acrs []acrcrd.AvailableCapacityReservation) *nodeCapacity {
	// Check for no capacity on node
	if len(acs) == 0 {
		return nil
	}

	// Chose AC only for the related node
	acs = FilterACList(acs, func(ac accrd.AvailableCapacity) bool {
		return ac.Spec.NodeId == node
	})

	// Sort AC to have the persistent order for each reservation
	sort.Slice(acs, func(i, j int) bool {
		// By size (the smallest first)
		if acs[i].Spec.Size < acs[j].Spec.Size {
			return true
		}
		if acs[i].Spec.Size > acs[j].Spec.Size {
			return false
		}

		// By name
		return acs[i].Name < acs[j].Name
	})

	acsOrder := scToACOrder{}
	for _, ac := range acs {
		if acsOrder[ac.Spec.StorageClass] == nil {
			acsOrder[ac.Spec.StorageClass] = []string{}
		}
		acsOrder[ac.Spec.StorageClass] = append(acsOrder[ac.Spec.StorageClass], ac.Name)
	}

	// ANY SC should select ACs with the order - HDD->SSD->NVMe
	acsOrder[v1.StorageClassAny] = append(acsOrder[v1.StorageClassAny], acsOrder[v1.StorageClassHDD]...)
	acsOrder[v1.StorageClassAny] = append(acsOrder[v1.StorageClassAny], acsOrder[v1.StorageClassSSD]...)
	acsOrder[v1.StorageClassAny] = append(acsOrder[v1.StorageClassAny], acsOrder[v1.StorageClassNVMe]...)

	// LVG SCs should select non-LVG ACs after LVG
	acsOrder[v1.StorageClassHDDLVG] = append(acsOrder[v1.StorageClassHDDLVG], acsOrder[v1.StorageClassHDD]...)
	acsOrder[v1.StorageClassSSDLVG] = append(acsOrder[v1.StorageClassSSDLVG], acsOrder[v1.StorageClassSSD]...)
	acsOrder[v1.StorageClassNVMeLVG] = append(acsOrder[v1.StorageClassNVMeLVG], acsOrder[v1.StorageClassNVMe]...)

	acMap := ACMap{}
	for i, ac := range acs {
		acMap[ac.Name] = &acs[i]
	}

	reservedACs := reservedACs{}
	for _, acr := range acrs {
		for _, request := range acr.Spec.ReservationRequests {
			reservedCapacity := &reservedCapacity{
				Size:         request.CapacityRequest.Size,
				StorageClass: request.CapacityRequest.StorageClass,
			}
			// Add reservation from ACR or update existed one if it repeats more than one time
			for _, reservation := range request.Reservations {
				existed, ok := reservedACs[reservation]
				if !ok {
					reservedACs[reservation] = reservedCapacity
				} else {
					existed.Size += reservedCapacity.Size
				}
			}
		}
	}

	return &nodeCapacity{
		node:        node,
		acs:         acMap,
		acsOrder:    acsOrder,
		reservedACs: reservedACs,
	}
}

func (nc *nodeCapacity) selectACForVolume(vol *genV1.Volume) *accrd.AvailableCapacity {
	requiredSize := vol.GetSize()
	if util.IsStorageClassLVG(vol.StorageClass) {
		// we should round up volume size, it should be aligned with LVM PE size
		// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
		requiredSize = AlignSizeByPE(vol.GetSize())
	}

	for _, ac := range nc.acsOrder[vol.StorageClass] {
		if requiredSize <= nc.acs[ac].Spec.Size {
			// check if AC is reserved
			reservation, ok := nc.reservedACs[ac]

			// reserve AC, if it is not found in reservations
			if !ok {
				foundAC := nc.acs[ac]
				nc.reservedACs[foundAC.Name] = &reservedCapacity{
					Size:         vol.Size,
					StorageClass: vol.StorageClass,
				}
				return foundAC
			}

			// skip AC, if required SC is non-LVG
			if !util.IsStorageClassLVG(vol.StorageClass) {
				continue
			}

			// skip AC, if AC was reserved for non-LVG
			if !util.IsStorageClassLVG(reservation.StorageClass) {
				continue
			}

			// select AC, if it has enough capacity
			if reservation.Size+requiredSize <= nc.acs[ac].Spec.Size {
				foundAC := nc.acs[ac]
				nc.reservedACs[foundAC.Name].Size += requiredSize
				return foundAC
			}
		}
	}

	return nil
}
