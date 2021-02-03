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

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// NewReservationHelper returns new instance of ReservationHelper
func NewReservationHelper(logger *logrus.Entry, client *k8s.KubeClient,
	capReader CapacityReader, resReader ReservationReader) *ReservationHelper {
	acMetrics := metrics.NewMetrics(prometheus.HistogramOpts{
		Name:    "ac_reservation_duration",
		Help:    "AvailableCapacity reservation duration",
		Buckets: prometheus.ExponentialBuckets(0.005, 1.5, 10),
	}, "method")
	if err := prometheus.Register(acMetrics.Collect()); err != nil {
		logger.WithField("component", "NewReservationHelper").
			Errorf("Failed to register metric: %v", err)
	}
	return &ReservationHelper{
		logger:    logger,
		client:    client,
		resReader: resReader,
		capReader: capReader,
		metric:    acMetrics,
	}
}

// ReservationHelper provides methods to create and release reservation
type ReservationHelper struct {
	logger  *logrus.Entry
	client  *k8s.KubeClient
	updated bool

	resReader ReservationReader
	capReader CapacityReader

	acList  []accrd.AvailableCapacity
	acrList []acrcrd.AvailableCapacityReservation

	acMap       ACMap
	acrMap      ACRMap
	acNameToACR ACNameToACRNamesMap

	metric metrics.Statistic
}

// CreateReservation create reservation
func (rh *ReservationHelper) CreateReservation(ctx context.Context, placingPlan *VolumesPlacingPlan) error {
	defer rh.metric.EvaluateDurationForMethod("CreateReservation")()
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.CreateReservation")

	volToAC := placingPlan.GetACsForVolumes()

	var (
		createErr   error
		createdACRs = make([]*acrcrd.AvailableCapacityReservation, 0, len(volToAC))
	)

	for v, acs := range volToAC {
		acsNames := make([]string, len(acs))
		for i := 0; i < len(acs); i++ {
			acsNames[i] = acs[i].Name
		}
		acrCR := rh.client.ConstructACRCR(genV1.AvailableCapacityReservation{
			Name:         uuid.New().String(),
			StorageClass: v.StorageClass,
			Size:         v.Size,
			Reservations: acsNames,
		})
		if createErr = rh.client.CreateCR(ctx, acrCR.Name, acrCR); createErr != nil {
			createErr = fmt.Errorf("unable to create ACR CR %v for volume %v: %v", acrCR.Spec, v, createErr)
			break
		}
		createdACRs = append(createdACRs, acrCR)
	}
	if createErr == nil {
		return nil
	}
	// try to remove all created ACRs
	// ctx can be canceled at this moment, so we will create new one
	ctx = context.Background()
	for _, acr := range createdACRs {
		if err := rh.client.DeleteCR(ctx, acr); err != nil {
			logger.Errorf("Unable to remove ACR %s: %v", acr.Name, err)
		}
	}
	return createErr
}

// ReleaseReservation removes ACR for AC
// if AC is in multiple ACRs, most suitable ACR will be remove, check choseACFromOldestACR function doc for details
// Also, if AC was converted to AC with another SC, for example HDD-> HDDLVG,
// we need to replace old AC with new in all ACRs
func (rh *ReservationHelper) ReleaseReservation(
	ctx context.Context, volume *genV1.Volume, ac, acReplacement *accrd.AvailableCapacity) error {
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.ReleaseReservation")
	if err := rh.updateIfRequired(ctx); err != nil {
		return err
	}
	// we should select ACR to remove from ACRs which have same size and SC as volume
	filteredACRMap, filteredACNameToACR := buildACRMaps(
		FilterACRList(rh.acrList, func(acr acrcrd.AvailableCapacityReservation) bool {
			return acr.Spec.StorageClass == volume.StorageClass && acr.Spec.Size == volume.Size
		}))
	_, acrToRemove := choseACFromOldestACR(ACMap{ac.Name: ac}, filteredACRMap, filteredACNameToACR)
	if acrToRemove == nil {
		logger.Infof("ACR holding AC %s not found. Skip deletion.", ac.Name)
		return nil
	}
	if err := rh.removeACR(ctx, acrToRemove); err != nil {
		return err
	}
	if ac == acReplacement {
		return nil
	}
	if err := rh.removeACFromACRs(ctx, acrToRemove.Name, ac); err != nil {
		return err
	}
	return nil
}

// ExtendReservations allows to add additional AC to ACRs which hold parent AC
func (rh *ReservationHelper) ExtendReservations(ctx context.Context,
	parentAC *accrd.AvailableCapacity, additionalAC string) error {
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.ExtendReservations")
	if err := rh.updateIfRequired(ctx); err != nil {
		return err
	}

	// list of ACRs names which hold parent AC
	acrNamesToCheck := rh.acNameToACR[parentAC.Name]

	acrToCheck := make([]*acrcrd.AvailableCapacityReservation, 0, len(acrNamesToCheck))
	for _, name := range acrNamesToCheck {
		acr := rh.acrMap[name]
		if acr == nil {
			logger.Warningf("unknown AC Name in ACR.Spec.Reservations, posible bug")
			continue
		}
		acrToCheck = append(acrToCheck, acr)
	}

	var acrToUpdate []*acrcrd.AvailableCapacityReservation

	for _, acr := range acrToCheck {
		alreadyExist := false
		for _, res := range acr.Spec.Reservations {
			if res == additionalAC {
				alreadyExist = true
				break
			}
		}
		if !alreadyExist {
			acr.Spec.Reservations = append(acr.Spec.Reservations, additionalAC)
			acrToUpdate = append(acrToUpdate, acr)
		}
	}

	for _, acr := range acrToUpdate {
		if err := rh.client.Update(ctx, acr); err != nil {
			logger.Infof("Fail to update ACR %s: %s", acr.Name, err.Error())
			return err
		}
	}
	return nil
}

func (rh *ReservationHelper) updateIfRequired(ctx context.Context) error {
	if rh.updated {
		return nil
	}
	return rh.Update(ctx)
}

// Update do a force data update
func (rh *ReservationHelper) Update(ctx context.Context) error {
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
}

func (rh *ReservationHelper) removeACR(ctx context.Context, acr *acrcrd.AvailableCapacityReservation) error {
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.removeACR")
	err := rh.client.Delete(ctx, acr)
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

func (rh *ReservationHelper) removeACFromACRs(ctx context.Context, removedACR string, ac *accrd.AvailableCapacity) error {
	logger := util.AddCommonFields(ctx, rh.logger, "ReservationHelper.removeACFromACRs")

	acrToCheck, ok := rh.acNameToACR[ac.Name]
	if !ok {
		logger.Infof("Can't find ACRs for AC %s", ac.Name)
		return nil
	}

	var acrToUpdate []*acrcrd.AvailableCapacityReservation

	for _, acrName := range acrToCheck {
		// ignore already removed ACR
		if acrName == removedACR {
			continue
		}
		acr, ok := rh.acrMap[acrName]
		if !ok {
			continue
		}
		removed := false
		resLen := len(acr.Spec.Reservations)
		for i := 0; i < resLen; i++ {
			if acr.Spec.Reservations[i] == ac.Name {
				acr.Spec.Reservations[i] = acr.Spec.Reservations[len(acr.Spec.Reservations)-1]
				acr.Spec.Reservations = acr.Spec.Reservations[:len(acr.Spec.Reservations)-1]
				i--
				resLen--
				removed = true
			}
		}
		if removed {
			acrToUpdate = append(acrToUpdate, acr)
		}
	}
	for _, acr := range acrToUpdate {
		err := rh.client.Update(ctx, acr)
		if err != nil {
			logger.Infof("Fail to update ACR %s: %s", acr.Name, err.Error())
			return err
		}
		logger.Infof("ACR %s updated", acr.Name)
	}

	return nil
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
		for _, acName := range acr.Spec.Reservations {
			acMap[acName] = struct{}{}
		}
	}
	return acMap
}

// choseACFromOldestACR chose AC from oldest ACR
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
		for _, acName := range acr.Spec.Reservations {
			if _, ok := acNameToACRNamesMap[acName]; !ok {
				acNameToACRNamesMap[acName] = []string{}
			}
			acNameToACRNamesMap[acName] = append(acNameToACRNamesMap[acName], acr.Name)
		}
	}
	return acrMAP, acNameToACRNamesMap
}
