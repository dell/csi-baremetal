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

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/api/generated/v1/api"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

// AvailableCapacityOperations is the interface for interact with AvailableCapacity CRs from Controller
type AvailableCapacityOperations interface {
	RecreateACToLVGSC(ctx context.Context, sc string, acs ...accrd.AvailableCapacity) *accrd.AvailableCapacity
}

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

// RecreateACToLVGSC creates new LVG using locations from provided ACs.
// Concerts first AC to LVG SC and set size of remaining to 0
// Receives newSC as string (e.g. HDDLVG) and AvailableCapacities where LVG should be based
// Returns created AC or nil
func (a *ACOperationsImpl) RecreateACToLVGSC(ctx context.Context, newSC string,
	acs ...accrd.AvailableCapacity) *accrd.AvailableCapacity {
	ll := a.log.WithFields(logrus.Fields{
		"method":   "RecreateACToLVGSC",
		"volumeID": ctx.Value(base.RequestUUID),
	})

	if len(acs) == 0 {
		return nil
	}

	ll.Debugf("Recreating ACs %v with SC %s to SC %s", acs[0], acs[0].Spec.StorageClass, newSC)

	lvgLocations := make([]string, len(acs))
	var lvgSize int64
	for i, ac := range acs {
		lvgLocations[i] = ac.Spec.Location
		lvgSize += capacityplanner.SubtractLVMMetadataSize(ac.Spec.Size)
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
			Health:    apiV1.HealthGood,
		}
	)

	// create LVG CR based on ACs
	lvg := a.k8sClient.ConstructLVGCR(name, apiLVG)
	if err = a.k8sClient.CreateCR(ctx, name, lvg); err != nil {
		ll.Errorf("Unable to create LVG CR: %v", err)
		return nil
	}
	ll.Infof("LVG %v was created.", apiLVG)

	// convert first AC to LVG type
	updatedAC := &acs[0]
	updatedAC.Spec.Size = lvgSize
	updatedAC.Spec.Location = lvg.Name
	updatedAC.Spec.StorageClass = newSC
	if err = a.k8sClient.UpdateCR(ctx, updatedAC); err != nil {
		ll.Errorf("Unable to update AC %v, error: %v.", updatedAC, err)
		return nil
	}

	// set size of remaining ACs to 0
	/*for _, ac := range acs[:1] {
		ac.Spec.Size = 0
		// nolint: scopelint
		if err = a.k8sClient.UpdateCR(ctx, &ac); err != nil {
			ll.Errorf("Unable to update AC %v, error: %v.", ac, err)
		}
	}*/

	// get recent version
	// TODO - refactor this code https://github.com/dell/csi-baremetal/issues/371
	if err = a.k8sClient.ReadCR(ctx, updatedAC.Name, "", updatedAC); err != nil {
		ll.Errorf("Unable to read latest AC version %v, error: %v.", updatedAC, err)
		return nil
	}

	ll.Infof("AC was updated: %v", updatedAC)
	return updatedAC
}
