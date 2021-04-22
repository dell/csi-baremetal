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

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"

	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// NewACReader returns instance of ACReader
func NewACReader(client *k8s.KubeClient, logger *logrus.Entry, cached bool) *ACReader {
	return &ACReader{
		client: client,
		logger: logger,
		cached: cached,
	}
}

// ACReader read AC from kubernetes API
type ACReader struct {
	client *k8s.KubeClient
	logger *logrus.Entry
	cached bool
	cache  []accrd.AvailableCapacity
}

// ReadCapacity returns AC list which was read from kubernetes API or from cache
func (acr *ACReader) ReadCapacity(ctx context.Context) ([]accrd.AvailableCapacity, error) {
	logger := util.AddCommonFields(ctx, acr.logger, "ACReader.ReadCapacity")
	if acr.cached && acr.cache != nil {
		logger.Tracef("Read AvailableCapacity from cache: %+v", acr.cache)
		return acr.cache, nil
	}
	acList := &accrd.AvailableCapacityList{}
	if err := acr.client.ReadList(ctx, acList); err != nil {
		logger.Errorf("failed to read AC list: %s", err.Error())
		return nil, err
	}
	logger.Tracef("Read AvailableCapacity: %+v", acList.Items)
	if acr.cached {
		acr.cache = acList.Items
	}
	return acList.Items, nil
}

// NewACRReader returns instance of ACReader
func NewACRReader(client *k8s.KubeClient, logger *logrus.Entry, cached bool) *ACRReader {
	return &ACRReader{
		client: client,
		logger: logger,
		cached: cached,
	}
}

// ACRReader read ACR from kubernetes API
type ACRReader struct {
	client *k8s.KubeClient
	logger *logrus.Entry
	cached bool
	cache  []acrcrd.AvailableCapacityReservation
}

// ReadReservations returns ACR list which was read from kubernetes API
func (acr *ACRReader) ReadReservations(ctx context.Context) ([]acrcrd.AvailableCapacityReservation, error) {
	logger := util.AddCommonFields(ctx, acr.logger, "ACRReader.ReadReservations")
	if acr.cached && acr.cache != nil {
		logger.Tracef("Read AvailableCapacityReservations from cache: %+v", acr.cache)
		return acr.cache, nil
	}
	acrList := &acrcrd.AvailableCapacityReservationList{}
	if err := acr.client.ReadList(ctx, acrList); err != nil {
		logger.Errorf("failed to read ACR list: %s", err.Error())
		return nil, err
	}
	logger.Tracef("Read AvailableCapacityReservations: %+v", acrList.Items)
	if acr.cached {
		acr.cache = acrList.Items
	}
	return acrList.Items, nil
}

// ReadReservation returns ACR which was read from kubernetes API
func (acr *ACRReader) ReadReservation(ctx context.Context, name string) (*acrcrd.AvailableCapacityReservation, error) {
	logger := util.AddCommonFields(ctx, acr.logger, "ACRReader.ReadReservations")

	// TODO do we need caching support for reservations - https://github.com/dell/csi-baremetal/issues/371
	/*if acr.cached && acr.cache != nil {
		logger.Tracef("Read AvailableCapacityReservations from cache: %+v", acr.cache)
		return acr.cache, nil
	}*/

	reservation := &acrcrd.AvailableCapacityReservation{}

	// ACR is cluster scoped
	if err := acr.client.ReadCR(ctx, name, "", reservation); err != nil {
		// don't print error message when not found since expected
		if !errors.IsNotFound(err) {
			logger.Errorf("failed to read ACR %s: %s", name, err.Error())
		}
		return reservation, err
	}

	return reservation, nil

	// TODO do we need caching support for reservations - https://github.com/dell/csi-baremetal/issues/371
	/*logger.Tracef("Read AvailableCapacityReservations: %+v", acrList.Items)
	if acr.cached {
		acr.cache = acrList.Items
	}
	return acrList.Items, nil*/
}

// NewUnreservedACReader returns instance of UnreservedACReader
func NewUnreservedACReader(logger *logrus.Entry, capReader CapacityReader,
	resReader ReservationReader) *UnreservedACReader {
	return &UnreservedACReader{
		capReader: capReader,
		resReader: resReader,
		logger:    logger,
	}
}

// UnreservedACReader capReader which returns ACs reserved in ACR
type UnreservedACReader struct {
	capReader CapacityReader
	resReader ReservationReader
	logger    *logrus.Entry
}

// ReadCapacity returns unreserved ACs
func (rar *UnreservedACReader) ReadCapacity(ctx context.Context) ([]accrd.AvailableCapacity, error) {
	logger := util.AddCommonFields(ctx, rar.logger, "UnreservedACReader.ReadCapacity")

	acrList, err := rar.resReader.ReadReservations(ctx)
	if err != nil {
		logger.Errorf("failed to read ACR list: %s", err.Error())
		return nil, err
	}

	acList, err := rar.capReader.ReadCapacity(ctx)
	if err != nil {
		logger.Errorf("failed to read AC list: %s", err.Error())
		return nil, err
	}

	reservationHelper := NewReservationFilter()
	reservedAC := reservationHelper.FilterByReservation(false, acList, acrList)
	logger.Tracef("Read AvailableCapacity: %+v", reservedAC)
	return reservedAC, nil
}
