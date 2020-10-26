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
	"fmt"

	"github.com/sirupsen/logrus"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
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
	PlanVolumesPlacing(ctx context.Context, volumes []*genV1.Volume) (*VolumesPlacingPlan, error)
}

// CapacityManagerBuilder interface for capacity managers creation
type CapacityManagerBuilder interface {
	// GetCapacityManager returns CapacityManager
	GetCapacityManager(logger *logrus.Entry, capReader CapacityReader) CapacityPlaner
	// GetReservedCapacityManager returns ReservedCapacityManager
	GetReservedCapacityManager(logger *logrus.Entry,
		capReader CapacityReader, resReader ReservationReader) CapacityPlaner
}

// DefaultCapacityManagerBuilder is a builder for default CapacityManagers
type DefaultCapacityManagerBuilder struct{}

// GetCapacityManager returns default implementation of CapacityManager
func (dcmb *DefaultCapacityManagerBuilder) GetCapacityManager(
	logger *logrus.Entry, capReader CapacityReader) CapacityPlaner {
	return NewCapacityManager(logger, capReader)
}

// GetReservedCapacityManager returns default implementation of ReservedCapacityManager
func (dcmb *DefaultCapacityManagerBuilder) GetReservedCapacityManager(
	logger *logrus.Entry, capReader CapacityReader, resReader ReservationReader) CapacityPlaner {
	return NewReservedCapacityManager(logger, capReader, resReader)
}

// NewCapacityManager return new instance of CapacityManager
func NewCapacityManager(logger *logrus.Entry, capReader CapacityReader) *CapacityManager {
	return &CapacityManager{
		logger:    logger,
		capReader: capReader,
	}
}

// CapacityManager provides placing plan for volumes
type CapacityManager struct {
	logger    *logrus.Entry
	capReader CapacityReader

	// nodeID to nodeCapacity
	nodesCapacity map[string]*nodeCapacity
}

// PlanVolumesPlacing build placing plan for volumes
func (cm *CapacityManager) PlanVolumesPlacing(
	ctx context.Context, volumes []*genV1.Volume) (*VolumesPlacingPlan, error) {
	logger := util.AddCommonFields(ctx, cm.logger, "CapacityManager.PlanVolumesPlacing")
	err := cm.update(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update capacity data: %s", err.Error())
	}
	plan := VolumesPlanMap{}

	for node := range cm.nodesCapacity {
		volToACOnNode := cm.selectCapacityOnNode(ctx, node, volumes)
		if volToACOnNode == nil {
			continue
		}
		plan[node] = volToACOnNode
	}
	if len(plan) == 0 {
		logger.Info("Required capacity for volumes not found")
		return nil, nil
	}
	logger.Info("Capacity for all volumes found")
	return NewVolumesPlacingPlan(plan, cm.convertCapacityToMap()), nil
}

func (cm *CapacityManager) selectCapacityOnNode(ctx context.Context, node string, volumes []*genV1.Volume) VolToACMap {
	logger := util.AddCommonFields(ctx, cm.logger, "CapacityManager.selectCapacityOnNode")
	nodeCap := cm.nodesCapacity[node]

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

func (cm *CapacityManager) update(ctx context.Context) error {
	logger := util.AddCommonFields(ctx, cm.logger, "CapacityManager.update")
	cm.nodesCapacity = map[string]*nodeCapacity{}
	capacity, err := cm.capReader.ReadCapacity(ctx)
	if err != nil {
		logger.Errorf("Failed to read capacity: %s", err.Error())
		return err
	}
	for _, c := range capacity {
		c := c
		nodeID := c.Spec.NodeId
		logger.Tracef("register capacity %s on node %s", c.Name, nodeID)
		cm.registerNodeCapacity(c.Spec.NodeId, &c)
	}
	return nil
}

func (cm *CapacityManager) convertCapacityToMap() NodeCapacityMap {
	result := NodeCapacityMap{}
	for nodeID, capData := range cm.nodesCapacity {
		result[nodeID] = capData.capacity
	}
	return result
}

func (cm *CapacityManager) registerNodeCapacity(node string, capacity *accrd.AvailableCapacity) {
	if _, ok := cm.nodesCapacity[node]; !ok {
		cm.nodesCapacity[node] = &nodeCapacity{capacity: ACMap{}}
	}
	cm.nodesCapacity[node].registerAC(capacity)
}

// NewReservedCapacityManager returns new instance of ReservedCapacityManager
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
func (rcm *ReservedCapacityManager) PlanVolumesPlacing(
	ctx context.Context, volumes []*genV1.Volume) (*VolumesPlacingPlan, error) {
	logger := util.AddCommonFields(ctx, rcm.logger, "ReservedCapacityManager.PlanVolumesPlacing")
	if len(volumes) == 0 {
		return nil, nil
	}
	if len(volumes) > 1 {
		return nil, fmt.Errorf("plannning for multipile volumes not supported, volumes count: %d", len(volumes))
	}
	volume := volumes[0]
	err := rcm.update(ctx, volume)
	if err != nil {
		return nil, err
	}
	selectedACs := rcm.selectBestACForNodes()
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

func (rcm *ReservedCapacityManager) update(ctx context.Context, volume *genV1.Volume) error {
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
		return acr.Spec.StorageClass == volume.StorageClass && acr.Spec.Size == volume.Size
	})
	resFilter := NewReservationFilter()
	reservedACs := resFilter.FilterByReservation(true, acList, filteredACRs)
	rcm.nodeCapacityMap = buildNodeCapacityMap(reservedACs)
	rcm.acrMap, rcm.acNameToACRNamesMap = buildACRMaps(filteredACRs)

	return nil
}

// selectBestACForNode select best AC for volume on node
func (rcm *ReservedCapacityManager) selectBestACForNodes() NodeCapacityMap {
	selectedCapacityMap := NodeCapacityMap{}
	for node := range rcm.nodeCapacityMap {
		acForNode, _ := choseACFromOldestACR(rcm.nodeCapacityMap[node], rcm.acrMap, rcm.acNameToACRNamesMap)
		if acForNode == nil {
			continue
		}
		selectedCapacityMap[node] = ACMap{acForNode.Name: acForNode}
	}
	return selectedCapacityMap
}
