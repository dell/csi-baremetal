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

	v1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/metrics"
	"github.com/dell/csi-baremetal/pkg/metrics/common"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// NewReservationHelper returns new instance of ReservationHelper
func NewReservationHelper(logger *logrus.Entry, client *k8s.KubeClient,
	capReader CapacityReader) *ReservationHelper {
	return &ReservationHelper{
		logger:    logger,
		client:    client,
		capReader: capReader,
		metric:    common.ReservationDuration,
	}
}

// ReservationHelper provides methods to create and release reservation
type ReservationHelper struct {
	logger *logrus.Entry
	client *k8s.KubeClient

	capReader CapacityReader
	metric    metrics.Statistic
}

// UpdateReservation updates reservation CR
func (rh *ReservationHelper) UpdateReservation(ctx context.Context, placingPlan *VolumesPlacingPlan,
	nodes []string, reservation *acrcrd.AvailableCapacityReservation) error {
	defer rh.metric.EvaluateDurationForMethod("UpdateReservation")()
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.UpdateReservation")

	nameToCapacity := map[string][]*accrd.AvailableCapacity{}
	for volume, capacity := range placingPlan.GetACsForVolumes() {
		nameToCapacity[volume.Id] = capacity
	}

	for _, request := range reservation.Spec.ReservationRequests {
		acs := nameToCapacity[request.CapacityRequest.Name]
		request.Reservations = make([]string, len(acs))
		for i := 0; i < len(acs); i++ {
			request.Reservations[i] = acs[i].Name
		}
	}

	// update reserved nodes
	reservation.Spec.NodeRequests.Reserved = nodes
	// confirm reservation
	reservation.Spec.Status = v1.MatchReservationStatus(v1.ReservationConfirmed)
	if err := rh.client.UpdateCR(ctx, reservation); err != nil {
		logger.Errorf("Unable to update reservation %s: %v", reservation.Name, err)
		return err
	}

	return nil
}

// ReleaseReservation removes AC from ACR or ACR completely when one volume requested or left
func (rh *ReservationHelper) ReleaseReservation(ctx context.Context, reservation *acrcrd.AvailableCapacityReservation,
	requestNum int) error {
	// logging customization
	log := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.ReleaseReservation")
	// need to remove found reservation from the list
	orig := reservation.Spec.ReservationRequests
	size := len(orig)
	// if one volume request left delete whole reservation CR
	if size == 1 {
		// delete
		return rh.removeACR(ctx, reservation)
	}

	// remove reservation request
	copy(orig[requestNum:], orig[requestNum+1:])
	orig[size-1] = nil
	orig = orig[:size-1]

	reservation.Spec.ReservationRequests = orig
	// update
	if err := rh.client.UpdateCR(ctx, reservation); err != nil {
		log.Errorf("Unable to update reservation %s: %v", reservation.Name, err)
		return err
	}
	return nil
}

// Update do a force data update
/*func (rh *ReservationHelper) Update(ctx context.Context) error {
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.update")
	var err error
	rh.acList, err = rh.capReader.ReadCapacity(ctx)
	if err != nil {
		logger.Errorf("failed to read AC list: %s", err.Error())
		return err
	}
	rh.acrList, err = rh.resReader.ReadReservations(ctx)
	if err != nil {
		logger.Errorf("failed to read ACR list: %s", err.Error())
		return err
	}
	rh.acMap = buildACMap(rh.acList)
	rh.acrMap, rh.acNameToACR = buildACRMaps(rh.acrList)

	rh.updated = true

	return nil
}*/

func (rh *ReservationHelper) removeACR(ctx context.Context, acr *acrcrd.AvailableCapacityReservation) error {
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.removeACR")
	err := rh.client.DeleteCR(ctx, acr)
	if err == nil {
		logger.Infof("ACR %s removed", acr.Name)
		return nil
	}
	if k8serrors.IsNotFound(err) {
		logger.Infof("ACR %s already removed", acr.Name)
		return nil
	}
	logger.Errorf("Fail to remove ACR %s: %s", acr.Name, err.Error())
	return err
}

// NewReservationFilter returns new instance of ReservationFilter
func NewReservationFilter() *ReservationFilter {
	return &ReservationFilter{}
}

// ReservationFilter helper for working with ACR based reservations
type ReservationFilter struct{}

// FilterByReservation returns AC which are reserved if reserved == true, or not reserved otherwise
func (rf *ReservationFilter) FilterByReservation(reserved bool, acs []accrd.AvailableCapacity,
	acrs []acrcrd.AvailableCapacityReservation) []accrd.AvailableCapacity {
	acInACR := buildACInACRMap(acrs)
	return FilterACList(acs, func(ac accrd.AvailableCapacity) bool {
		_, acIsReserved := acInACR[ac.Name]
		if reserved {
			// we looking for reserved ACs
			return acIsReserved
		}
		// we looking for unreserved ACs
		return !acIsReserved
	})
}

// FilterACRList filter for ACR list
func FilterACRList(
	acrs []acrcrd.AvailableCapacityReservation,
	filter func(acr acrcrd.AvailableCapacityReservation) bool) []acrcrd.AvailableCapacityReservation {
	var result []acrcrd.AvailableCapacityReservation
	for _, acr := range acrs {
		if filter(acr) {
			result = append(result, acr)
		}
	}
	return result
}

// FilterACList filter for AC list
func FilterACList(
	acs []accrd.AvailableCapacity, filter func(ac accrd.AvailableCapacity) bool) []accrd.AvailableCapacity {
	var result []accrd.AvailableCapacity
	for _, ac := range acs {
		if filter(ac) {
			result = append(result, ac)
		}
	}
	return result
}

// buildACInACRMap build map with AC names which included at least in one ACR
func buildACInACRMap(acrs []acrcrd.AvailableCapacityReservation) map[string]struct{} {
	acMap := map[string]struct{}{}
	for _, acr := range acrs {
		for _, request := range acr.Spec.ReservationRequests {
			for _, acName := range request.Reservations {
				acMap[acName] = struct{}{}
			}
		}
	}
	return acMap
}

// TODO reserve resources on requested nodes only - https://github.com/dell/csi-baremetal/issues/370
/*// choseACFromOldestACR chose AC from oldest ACR
func choseACFromOldestACR(acMap ACMap, acrMAP ACRMap, acToACRs ACNameToACRNamesMap) (
	*accrd.AvailableCapacity, *acrcrd.AvailableCapacityReservation) {
	var (
		oldest  metaV1.Time
		bestAC  *accrd.AvailableCapacity
		bestACR *acrcrd.AvailableCapacityReservation
	)

	for acName, ac := range acMap {
		acrNames, ok := acToACRs[acName]
		if !ok {
			continue
		}
		for _, acrName := range acrNames {
			acr, ok := acrMAP[acrName]
			if !ok {
				continue
			}
			acrCreationTimestamp := acr.GetCreationTimestamp()
			if acrCreationTimestamp.IsZero() {
				continue
			}
			if oldest.IsZero() || acrCreationTimestamp.Before(&oldest) {
				oldest = acrCreationTimestamp
				bestAC = ac
				bestACR = acr
			}
		}
	}
	return bestAC, bestACR
}

// buildNodeCapacityMap convert internal node capacity struct to Map with exported type
func buildNodeCapacityMap(acs []accrd.AvailableCapacity) NodeCapacityMap {
	capMap := NodeCapacityMap{}
	for _, ac := range acs {
		ac := ac
		if _, ok := capMap[ac.Spec.NodeId]; !ok {
			capMap[ac.Spec.NodeId] = ACMap{}
		}
		capMap[ac.Spec.NodeId][ac.Name] = &ac
	}
	return capMap
}

func buildACMap(acs []accrd.AvailableCapacity) ACMap {
	acMap := ACMap{}
	for _, ac := range acs {
		ac := ac
		acMap[ac.Name] = &ac
	}
	return acMap
}

func buildACRMaps(acrs []acrcrd.AvailableCapacityReservation) (ACRMap, ACNameToACRNamesMap) {
	acrMAP := ACRMap{}
	acNameToACRNamesMap := ACNameToACRNamesMap{}
	for _, acr := range acrs {
		acr := acr
		acrMAP[acr.Name] = &acr
		for _, request := range acr.Spec.ReservationRequests {
			for _, acName := range request.Reservations {
				if _, ok := acNameToACRNamesMap[acName]; !ok {
					acNameToACRNamesMap[acName] = []string{}
				}
				acNameToACRNamesMap[acName] = append(acNameToACRNamesMap[acName], acr.Name)
			}
		}
	}
	return acrMAP, acNameToACRNamesMap
}*/
