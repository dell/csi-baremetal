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
	"context"
	"sort"

	v1 "github.com/dell/csi-baremetal/api/v1"

	"github.com/sirupsen/logrus"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	baseerr "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// ACMap AC.Name to AC mapping
type ACMap map[string]*accrd.AvailableCapacity

// ACRMap ACR.Name to ACR mapping
type ACRMap map[string]*acrcrd.AvailableCapacityReservation

// SCToACMap SC name to ACMap mapping
type SCToACMap map[string]ACMap

// VolToACMap volume to AC mapping
type VolToACMap map[*genV1.Volume]*accrd.AvailableCapacity

// VolToACListMap volume to AC list mapping
type VolToACListMap map[*genV1.Volume][]*accrd.AvailableCapacity

// VolumesPlanMap NodeID to VolToACMap mapping
type VolumesPlanMap map[string]VolToACMap

// NodeCapacityMap NodeID to ACMap mapping
type NodeCapacityMap map[string]ACMap

// ACNameToACRNamesMap AC name to ACR names mapping
type ACNameToACRNamesMap map[string][]string

// CapacityReader methods to read available capacity
type CapacityReader interface {
	// ReadCapacity read capacity
	ReadCapacity(ctx context.Context) ([]accrd.AvailableCapacity, error)
}

// ReservationReader methods to read capacity reservations
type ReservationReader interface {
	// ReadReservations read capacity reservations
	ReadReservations(ctx context.Context) ([]acrcrd.AvailableCapacityReservation, error)
}

// CapacityPlaner describes interface for volumes placing planing
type CapacityPlaner interface {
	// PlanVolumesPlacing plan volumes placing on nodes
	PlanVolumesPlacing(ctx context.Context, volumes []*genV1.Volume, nodes []string) (*VolumesPlacingPlan, error)
}

// CapacityManagerBuilder interface for capacity managers creation
type CapacityManagerBuilder interface {
	// GetCapacityManager returns CapacityManager
	GetCapacityManager(logger *logrus.Entry,
		capReader CapacityReader, resReader ReservationReader) CapacityPlaner
	// TODO: Need to refactor, reservations for standalone pvc is not working - https://github.com/dell/csi-baremetal/issues/371
	/*	// GetReservedCapacityManager returns ReservedCapacityManager
		GetReservedCapacityManager(logger *logrus.Entry,
			capReader CapacityReader, resReader ReservationReader) CapacityPlaner*/
}

// DefaultCapacityManagerBuilder is a builder for default CapacityManagers
type DefaultCapacityManagerBuilder struct {
	SequentialLVGReservation bool
}

// GetCapacityManager returns default implementation of CapacityManager
func (dcmb *DefaultCapacityManagerBuilder) GetCapacityManager(
	logger *logrus.Entry, capReader CapacityReader, resReader ReservationReader) CapacityPlaner {
	return NewCapacityManager(logger, capReader, resReader, dcmb.SequentialLVGReservation)
}

// TODO: Need to refactor, reservations for standalone pvc is not working - https://github.com/dell/csi-baremetal/issues/371
/*// GetReservedCapacityManager returns default implementation of ReservedCapacityManager
func (dcmb *DefaultCapacityManagerBuilder) GetReservedCapacityManager(
	logger *logrus.Entry, capReader CapacityReader, resReader ReservationReader) CapacityPlaner {
	return NewReservedCapacityManager(logger, capReader, resReader)
}*/

// NewCapacityManager return new instance of CapacityManager
func NewCapacityManager(logger *logrus.Entry, capReader CapacityReader,
	resReader ReservationReader, sequentialLVGReservation bool) *CapacityManager {
	return &CapacityManager{
		logger:                   logger,
		capReader:                capReader,
		resReader:                resReader,
		sequentialLVGReservation: sequentialLVGReservation,
	}
}

// CapacityManager provides placing plan for volumes
type CapacityManager struct {
	logger    *logrus.Entry
	capReader CapacityReader
	resReader ReservationReader

	// nodeID to nodeCapacity
	nodesCapacity map[string]*nodeCapacity

	// keep ACR with LVG REQUESTED until another ACR is RESERVED
	sequentialLVGReservation bool
}

// PlanVolumesPlacing build placing plan for volumes
func (cm *CapacityManager) PlanVolumesPlacing(ctx context.Context, volumes []*genV1.Volume, nodes []string) (*VolumesPlacingPlan, error) {
	logger := util.AddCommonFields(ctx, cm.logger, "CapacityManager.PlanVolumesPlacing")

	acList, err := cm.capReader.ReadCapacity(ctx)
	if err != nil {
		logger.Errorf("failed to read AC list: %s", err.Error())
		return nil, err
	}
	acrList, err := cm.resReader.ReadReservations(ctx)
	if err != nil {
		logger.Errorf("failed to read ACR list: %s", err.Error())
		return nil, err
	}

	if cm.sequentialLVGReservation {
		if cm.isLVGCapacityReserved(ctx, volumes, acrList) {
			return nil, baseerr.ErrorRejectReservationRequest
		}
	}

	// update node capacity
	cm.nodesCapacity = map[string]*nodeCapacity{}
	for _, node := range nodes {
		cm.nodesCapacity[node] = newNodeCapacity(node, acList, acrList)
	}
	logger.Debugf("Node_capacity: %+v", cm.nodesCapacity)

	plan := VolumesPlanMap{}

	// sort capacity requests (LVG first)
	sort.Slice(volumes, func(i, j int) bool {
		if util.IsStorageClassLVG(volumes[i].StorageClass) && !util.IsStorageClassLVG(volumes[j].StorageClass) {
			return true
		}
		if !util.IsStorageClassLVG(volumes[i].StorageClass) && util.IsStorageClassLVG(volumes[j].StorageClass) {
			return false
		}

		return volumes[i].Size < volumes[j].Size
	})

	// select ACs on each node for volumes
	for _, node := range nodes {
		volToACOnNode := cm.selectCapacityOnNode(ctx, node, volumes)
		if volToACOnNode == nil {
			continue
		}
		plan[node] = volToACOnNode
	}
	logger.Debugf("Placing_plan: %+v", plan)

	if len(plan) == 0 {
		logger.Warning("Required capacity for volumes not found")
		return nil, nil
	}
	logger.Info("Capacity for all volumes found")
	return NewVolumesPlacingPlan(plan, cm.convertCapacityToMap()), nil
}

func (cm *CapacityManager) selectCapacityOnNode(ctx context.Context, node string, volumes []*genV1.Volume) VolToACMap {
	logger := util.AddCommonFields(ctx, cm.logger, "CapacityManager.selectCapacityOnNode")
	nodeCap := cm.nodesCapacity[node]
	// capacity might not exists
	if nodeCap == nil {
		logger.Tracef("No AC found on node %s", node)
		return nil
	}

	result := VolToACMap{}

	for _, vol := range volumes {
		ac := nodeCap.selectACForVolume(vol)
		if ac == nil {
			logger.Tracef("AC for vol: %s not found on node %s", vol.Id, node)
			return nil
		}
		logger.Tracef("AC %v selected for vol: %s found on node %s", ac, vol.Id, node)
		result[vol] = ac
	}
	logger.Debugf("AC for all volumes found on node %s", node)
	return result
}

// check for existing ACR in RESERVED state with the same LVG SC
// need to skip new reservation for LVG requests to avoid usage extra non-LVG AC for LVG
func (cm *CapacityManager) isLVGCapacityReserved(ctx context.Context, volumes []*genV1.Volume, acrs []acrcrd.AvailableCapacityReservation) bool {
	logger := util.AddCommonFields(ctx, cm.logger, "CapacityManager.isLVGCapacityReserved")

	if !cm.sequentialLVGReservation {
		return false
	}

	var lvgVolumes []*genV1.Volume

	// find capacity requests based on LVG
	for _, vol := range volumes {
		if util.IsStorageClassLVG(vol.StorageClass) {
			lvgVolumes = append(lvgVolumes, vol)
		}
	}
	if len(lvgVolumes) == 0 {
		return false
	}

	// check if other ACRs in RESERVED state has requests with the same LVG SC
	for _, acr := range acrs {
		if acr.Spec.Status != v1.ReservationConfirmed {
			continue
		}
		for _, resRequest := range acr.Spec.ReservationRequests {
			for _, vol := range lvgVolumes {
				if vol.StorageClass == resRequest.CapacityRequest.StorageClass {
					logger.Debugf("ACR %s has LVG volume %s. Should retry reservation proccesing", acr.Name, resRequest.CapacityRequest.Name)
					return true
				}
			}
		}
	}

	return false
}

func (cm *CapacityManager) convertCapacityToMap() NodeCapacityMap {
	result := NodeCapacityMap{}
	for nodeID, capData := range cm.nodesCapacity {
		result[nodeID] = capData.acs
	}
	return result
}

// TODO: Need to refactor, reservations for standalone pvc is not working - https://github.com/dell/csi-baremetal/issues/371
/*// NewReservedCapacityManager returns new instance of ReservedCapacityManager
func NewReservedCapacityManager(
	logger *logrus.Entry, capReader CapacityReader, resReader ReservationReader) *ReservedCapacityManager {
	return &ReservedCapacityManager{
		logger:    logger,
		capReader: capReader,
		resReader: resReader,
	}
}

// ReservedCapacityManager provides placing plan when ACR based reservation enabled
type ReservedCapacityManager struct {
	logger    *logrus.Entry
	capReader CapacityReader
	resReader ReservationReader

	nodeCapacityMap     NodeCapacityMap
	acrMap              ACRMap
	acNameToACRNamesMap ACNameToACRNamesMap
}

// PlanVolumesPlacing build placing plan for reserved volumes
func (rcm *ReservedCapacityManager) PlanVolumesPlacing(ctx context.Context, volumes []*genV1.Volume, nodes []string) (*VolumesPlacingPlan, error) {
	logger := util.AddCommonFields(ctx, rcm.logger, "ReservedCapacityManager.PlanVolumesPlacing")
	// TODO reserve resources on requested nodes only - https://github.com/dell/csi-baremetal/issues/370
	_ = nodes
	if len(volumes) == 0 {
		return nil, nil
	}
	if len(volumes) > 1 {
		return nil, fmt.Errorf("plannning for multipile volumes not supported, volumes count: %d", len(volumes))
	}
	volume := volumes[0]
	// TODO refactor update logic here - https://github.com/dell/csi-baremetal/issues/371
	err := rcm.update(ctx, nil)
	if err != nil {
		return nil, err
	}
	selectedACs := rcm.selectBestACForNodes(ctx)
	if len(selectedACs) == 0 {
		logger.Info("Required capacity for volumes not found")
		return nil, nil
	}
	plan := VolumesPlanMap{}
	for node, acMap := range selectedACs {
		// we should have single value in acMap
		for _, ac := range acMap {
			plan[node] = VolToACMap{volume: ac}
		}
	}
	logger.Info("Capacity for all volumes found")
	return NewVolumesPlacingPlan(plan, rcm.nodeCapacityMap), nil
}

func (rcm *ReservedCapacityManager) update(ctx context.Context, info *util.VolumeInfo) error {
	logger := util.AddCommonFields(ctx, rcm.logger, "CapacityManager.update")
	rcm.nodeCapacityMap = NodeCapacityMap{}
	acList, err := rcm.capReader.ReadCapacity(ctx)
	if err != nil {
		logger.Errorf("failed to read AC list: %s", err.Error())
		return err
	}
	acrList, err := rcm.resReader.ReadReservations(ctx)
	if err != nil {
		logger.Errorf("failed to read ACR list: %s", err.Error())
		return err
	}
	filteredACRs := FilterACRList(acrList, func(acr acrcrd.AvailableCapacityReservation) bool {
		if acr.Namespace == info.Namespace {
			for _, request := range acr.Spec.ReservationRequests {
				if request.CapacityRequest.Name == info.Name {
					return true
				}
			}
		}
		return false
	})
	resFilter := NewReservationFilter()
	reservedACs := resFilter.FilterByReservation(true, acList, filteredACRs)
	rcm.nodeCapacityMap = buildNodeCapacityMap(reservedACs)
	rcm.acrMap, rcm.acNameToACRNamesMap = buildACRMaps(filteredACRs)

	return nil
}

// selectBestACForNode select best AC for volume on node
func (rcm *ReservedCapacityManager) selectBestACForNodes(ctx context.Context) NodeCapacityMap {
	logger := util.AddCommonFields(ctx, rcm.logger, "CapacityManager.selectBestACForNodes")
	selectedCapacityMap := NodeCapacityMap{}
	for node := range rcm.nodeCapacityMap {
		acForNode, _ := choseACFromOldestACR(rcm.nodeCapacityMap[node], rcm.acrMap, rcm.acNameToACRNamesMap)
		if acForNode == nil {
			continue
		}
		if acForNode.Spec.Size == 0 {
			logger.Warning("AvailableCapacity with zero size was selected. AvailableCapacity will be ignored.")
			continue
		}
		selectedCapacityMap[node] = ACMap{acForNode.Name: acForNode}
	}
	return selectedCapacityMap
}*/
