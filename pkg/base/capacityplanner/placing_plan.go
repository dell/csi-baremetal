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

import (
	"sort"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
)

// NewVolumesPlacingPlan returns new instance of VolumesPlacingPlan
func NewVolumesPlacingPlan(volMap VolumesPlanMap, capacityMap NodeCapacityMap) *VolumesPlacingPlan {
	return &VolumesPlacingPlan{
		plan:     volMap,
		capacity: capacityMap,
	}
}

// VolumesPlacingPlan hold information about volumes placing on nodes
type VolumesPlacingPlan struct {
	// plan holds mapping between nodeID and VolToACMap
	plan VolumesPlanMap
	// capacity holds mapping between nodeID and ACMap
	capacity NodeCapacityMap
}

// GetVolumesToACMapping returns volumes to AC mapping for node
func (vpp *VolumesPlacingPlan) GetVolumesToACMapping(node string) VolToACMap {
	if node == "" {
		return nil
	}
	plan, ok := vpp.plan[node]
	if !ok {
		return nil
	}
	return plan
}

// SelectNode returns less loaded node which has required capacity to create volume
func (vpp *VolumesPlacingPlan) SelectNode() string {
	suitableNodes := make([]string, 0, len(vpp.plan))
	for node := range vpp.plan {
		suitableNodes = append(suitableNodes, node)
	}
	if len(suitableNodes) == 0 {
		return ""
	}
	sort.Slice(suitableNodes, func(i, j int) bool {
		return len(vpp.capacity[suitableNodes[i]]) > len(vpp.capacity[suitableNodes[j]])
	})
	return suitableNodes[0]
}

// GetACForVolume returns AC selected for volume on node
func (vpp *VolumesPlacingPlan) GetACForVolume(node string, volume *genV1.Volume) *accrd.AvailableCapacity {
	volToACMapping := vpp.GetVolumesToACMapping(node)
	if volToACMapping == nil {
		return nil
	}
	ac, ok := volToACMapping[volume]
	if !ok {
		return nil
	}
	return ac
}

// GetACsForVolumes returns mapping between volume and AC list
// AC list consist of suitable ACs on all nodes
func (vpp *VolumesPlacingPlan) GetACsForVolumes() VolToACListMap {
	volToACListMap := VolToACListMap{}
	for _, volToACMap := range vpp.plan {
		for vol, ac := range volToACMap {
			volToACListMap[vol] = append(volToACListMap[vol], ac)
		}
	}
	return volToACListMap
}
