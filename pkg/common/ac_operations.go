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

// Package common is for common operations with CSI resources such as AvailableCapacity or Volume
package common

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// AvailableCapacityOperations is the interface for interact with AvailableCapacity CRs from Controller
type AvailableCapacityOperations interface {
	SearchAC(ctx context.Context, node string, requiredBytes int64, sc string) *accrd.AvailableCapacity
	DeleteIfEmpty(ctx context.Context, acLocation string) error
	RecreateACToLVGSC(ctx context.Context, sc string, acs ...accrd.AvailableCapacity) *accrd.AvailableCapacity
}

// AcSizeMinThresholdBytes means that if AC size becomes lower then AcSizeMinThresholdBytes that AC should be deleted
const AcSizeMinThresholdBytes = int64(util.MBYTE) // 1MB

// LvgDefaultMetadataSize is additional cost for new VG we should consider.
const LvgDefaultMetadataSize = int64(util.MBYTE) // 1MB

// DefaultPESize is the default extent size we should align with
// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
const DefaultPESize = 4 * int64(util.MBYTE)

// ACOperationsImpl is the basic implementation of AvailableCapacityOperations interface
type ACOperationsImpl struct {
	k8sClient *k8s.KubeClient
	log       *logrus.Entry
}

// NewACOperationsImpl is the constructor for ACOperationsImpl struct
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of ACOperationsImpl
func NewACOperationsImpl(k8sClient *k8s.KubeClient, l *logrus.Logger) *ACOperationsImpl {
	return &ACOperationsImpl{
		k8sClient: k8sClient,
		log:       l.WithField("component", "ACOperations"),
	}
}

// SearchAC searches appropriate available capacity and remove it's CR
// if SC is in LVM and there is no AC with such SC then LVG should be created based
// on non-LVM AC's and new AC should be created on point in LVG
// method shouldn't be used in separate goroutines without synchronizations.
// Receives golang context, node string which means the node where to find AC, required bytes for volume
// and storage class for created volume (For example HDD, HDDLVG, SSD, SSDLVG).
// Returns found AvailableCapacity CR instance
func (a *ACOperationsImpl) SearchAC(ctx context.Context,
	node string, requiredBytes int64, sc string) *accrd.AvailableCapacity {
	ll := a.log.WithFields(logrus.Fields{
		"method":        "SearchAC",
		"volumeID":      ctx.Value(k8s.RequestUUID),
		"requiredBytes": fmt.Sprintf("%.3fG", float64(requiredBytes)/float64(util.GBYTE)),
	})

	ll.Info("Search appropriate available ac")

	var (
		foundAC   *accrd.AvailableCapacity
		acList    = &accrd.AvailableCapacityList{}
		acNodeMap map[string][]*accrd.AvailableCapacity
	)

	err := a.k8sClient.ReadList(ctx, acList)
	if err != nil {
		ll.Errorf("Unable to read Available Capacity list, error: %v", err)
		return nil
	}

	acNodeMap = a.acNodeMapping(acList.Items)

	if node == "" {
		node = a.balanceAC(acNodeMap, requiredBytes, sc)
	}

	ll.Infof("Search AvailableCapacity on node %s with size not less %d bytes with storage class %s",
		node, requiredBytes, sc)

	if sc == apiV1.StorageClassAny {
		//First try to find AC with hdd, then AC with ssd, if we couldn't find AC with HDD or SSD, try to find AC with any StorageCLass
		for _, sc := range []string{apiV1.StorageClassHDD, apiV1.StorageClassSSD, ""} {
			foundAC = a.tryToFindAC(acNodeMap[node], sc, requiredBytes)
			if foundAC != nil {
				break
			}
		}
	} else {
		foundAC = a.tryToFindAC(acNodeMap[node], sc, requiredBytes)
	}

	if util.IsStorageClassLVG(sc) {
		if foundAC != nil {
			// check whether LVG being deleted or no
			lvgCR := &lvgcrd.LVG{}
			err := a.k8sClient.ReadCR(ctx, foundAC.Spec.Location, lvgCR)
			if err == nil && lvgCR.DeletionTimestamp.IsZero() {
				return foundAC
			}
			ll.Errorf("LVG %s that was chosen is being deleted", lvgCR.Name)
			return nil
		}
	}

	return foundAC
}

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

// DeleteIfEmpty search AC by it's location and remove if it size is less then threshold
// Receives golang context and AC Location as a string (For example Location could be Drive uuid in case of HDD SC)
// Returns error if something went wrong
func (a *ACOperationsImpl) DeleteIfEmpty(ctx context.Context, acLocation string) error {
	var acList = accrd.AvailableCapacityList{}
	_ = a.k8sClient.ReadList(ctx, &acList)

	for _, ac := range acList.Items {
		if ac.Spec.Location == acLocation {
			if ac.Spec.Size < AcSizeMinThresholdBytes {
				return a.k8sClient.DeleteCR(ctx, &ac)
			}
			return nil
		}
	}

	return fmt.Errorf("unable to find AC by location %s", acLocation)
}

// acNodeMapping constructs map with key - nodeID(hostname), value - AC CRs based on Spec.NodeID field of AC
// Receives slice of AvailableCapacity custom resources
// Returns map of AvailableCapacities where key is nodeID
func (a *ACOperationsImpl) acNodeMapping(acs []accrd.AvailableCapacity) map[string][]*accrd.AvailableCapacity {
	var (
		acNodeMap = make(map[string][]*accrd.AvailableCapacity)
		node      string
	)

	for _, ac := range acs {
		node = ac.Spec.NodeId
		if _, ok := acNodeMap[node]; !ok {
			acNodeMap[node] = make([]*accrd.AvailableCapacity, 0)
		}
		acTmp := ac
		acNodeMap[node] = append(acNodeMap[node], &acTmp)
	}
	return acNodeMap
}

// balanceAC looks for a node with appropriate AC and choose node with maximum AC, return node
// Receives acNodeMap gathered from acNodeMapping method, size requested by a volume, and appropriate storage class
// Returns the most unloaded node according to input parameters
func (a *ACOperationsImpl) balanceAC(acNodeMap map[string][]*accrd.AvailableCapacity,
	size int64, sc string) (node string) {
	maxLen := 0
	for nodeID, acs := range acNodeMap {
		if len(acs) > maxLen {
			// ensure that there is at least one AC with size not less than requiredBytes and with the same SC
			for _, ac := range acs {
				if ac.Spec.Size >= size && (ac.Spec.StorageClass == sc || sc == apiV1.StorageClassAny) {
					node = nodeID
					maxLen = len(acs)
					break
				}
			}
		}
	}

	return node
}

// RecreateACToLVGSC creates LVG(based on ACs) creates AC based on that LVG and set sise of provided ACs to 0.
// Receives newSC as string (e.g. HDDLVG) and AvailableCapacities where LVG should be based
// Returns created AC or nil
func (a *ACOperationsImpl) RecreateACToLVGSC(ctx context.Context, newSC string, acs ...accrd.AvailableCapacity) *accrd.AvailableCapacity {
	ll := a.log.WithFields(logrus.Fields{
		"method":   "RecreateACToLVGSC",
		"volumeID": ctx.Value(k8s.RequestUUID),
	})

	if len(acs) == 0 {
		return nil
	}

	ll.Debugf("Recreating ACs %v with SC %s to SC %s", acs[0], acs[0].Spec.StorageClass, newSC)

	lvgLocations := make([]string, len(acs))
	var lvgSize int64
	for i, ac := range acs {
		lvgLocations[i] = ac.Spec.Location
		lvgSize += ac.Spec.Size
	}

	var (
		err    error
		name   = uuid.New().String()
		apiLVG = api.LogicalVolumeGroup{
			Node:      acs[0].Spec.NodeId, // all ACs are from the same node
			Name:      name,
			Locations: lvgLocations,
			Size:      lvgSize,
			Status:    apiV1.Creating,
		}
	)

	// set size ACs to 0 to avoid allocations
	for _, ac := range acs {
		ac.Spec.Size = 0
		// nolint: scopelint
		if err = a.k8sClient.UpdateCR(ctx, &ac); err != nil {
			ll.Errorf("Unable to update AC %v, error: %v.", ac, err)
		}
	}

	// create LVG CR based on ACs
	lvg := a.k8sClient.ConstructLVGCR(name, apiLVG)
	if err = a.k8sClient.CreateCR(ctx, name, lvg); err != nil {
		ll.Errorf("Unable to create LVG CR: %v", err)
		return nil
	}
	ll.Infof("LVG %v was created.", apiLVG)

	// create new AC
	newACCRName := uuid.New().String()
	newACCR := a.k8sClient.ConstructACCR(newACCRName, api.AvailableCapacity{
		Location:     lvg.Name,
		NodeId:       acs[0].Spec.NodeId,
		StorageClass: newSC,
		Size:         lvgSize,
	})
	if err = a.k8sClient.CreateCR(ctx, newACCRName, newACCR); err != nil {
		ll.Errorf("Unable to create AC %v, error: %v", newACCRName, err)
		return nil
	}

	ll.Infof("AC was created: %v", newACCR)
	return newACCR
}

//tryToFindAC is used to find proper AvailableCapacity based on provided storageClass and requiredBytes
//If storageClass = "" then we search for AvailableCapacity with any storageClass
func (a *ACOperationsImpl) tryToFindAC(acs []*accrd.AvailableCapacity, storageClass string, requiredBytes int64) *accrd.AvailableCapacity {
	var (
		allocatedCapacity int64 = math.MaxInt64
		foundAC           *accrd.AvailableCapacity
		driveUUID         string
	)
	for _, ac := range acs {
		// Available capacity with system disk location won't be allocated
		if driveUUID == "" {
			driveUUID = a.k8sClient.GetSystemDriveUUID(context.Background(), ac.Spec.NodeId)
			if driveUUID == "" {
				driveUUID = base.SystemDriveAsLocation
			}
		}
		if storageClass != "" {
			if ac.Spec.Size < allocatedCapacity && ac.Spec.Size >= requiredBytes && ac.Spec.StorageClass == storageClass && ac.Spec.Location != driveUUID {
				foundAC = ac
				allocatedCapacity = ac.Spec.Size
			}
		} else {
			if ac.Spec.Size < allocatedCapacity && ac.Spec.Size >= requiredBytes && ac.Spec.Location != driveUUID {
				foundAC = ac
				allocatedCapacity = ac.Spec.Size
			}
		}
	}
	return foundAC
}
