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
	"github.com/stretchr/testify/mock"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
)

// CapacityReaderMock is a mock implementation of CapacityReader interface for test purposes
type CapacityReaderMock struct {
	mock.Mock
}

// ReadCapacity is a mock implementation of CapacityReader
func (cr *CapacityReaderMock) ReadCapacity(ctx context.Context) ([]accrd.AvailableCapacity, error) {
	args := cr.Mock.Called(ctx)
	var ret []accrd.AvailableCapacity
	if args.Get(0) != nil {
		ret = args.Get(0).([]accrd.AvailableCapacity)
	}
	return ret, args.Error(1)
}

// ReservationReaderMock is the mock implementation of ReservationReader interface for test purposes
type ReservationReaderMock struct {
	mock.Mock
}

// ReadReservations is a mock implementation of ReadReservations
func (rrm *ReservationReaderMock) ReadReservations(ctx context.Context) ([]acrcrd.AvailableCapacityReservation, error) {
	args := rrm.Mock.Called(ctx)
	var ret []acrcrd.AvailableCapacityReservation
	if args.Get(0) != nil {
		ret = args.Get(0).([]acrcrd.AvailableCapacityReservation)
	}
	return ret, args.Error(1)
}

// PlannerMock is a mock implementation of CapacityManager
type PlannerMock struct {
	mock.Mock
}

// PlanVolumesPlacing mock implementation of PlanVolumesPlacing
func (cr *PlannerMock) PlanVolumesPlacing(ctx context.Context, volumes []*genV1.Volume, _ []string) (*VolumesPlacingPlan, error) {
	args := cr.Mock.Called(ctx, volumes)

	volIDToVol := make(map[string]*genV1.Volume, len(volumes))
	for _, vol := range volumes {
		volIDToVol[vol.Id] = vol
	}
	var ret *VolumesPlacingPlan
	if args.Get(0) != nil {
		ret = args.Get(0).(*VolumesPlacingPlan)
		for _, volPlan := range ret.plan {
			for vol, ac := range volPlan {
				ac := ac
				replace, ok := volIDToVol[vol.Id]
				if ok {
					delete(volPlan, vol)
					volPlan[replace] = ac
				}
			}
		}
	}
	return ret, args.Error(1)
}

// MockCapacityManagerBuilder is a builder for CapacityManagers which return mocked versions of managers
type MockCapacityManagerBuilder struct {
	Manager CapacityPlaner
}

// GetCapacityManager returns mock implementation of CapacityManager
func (mcb *MockCapacityManagerBuilder) GetCapacityManager(_ *logrus.Entry, _ CapacityReader,
	_ ReservationReader,
) CapacityPlaner {
	return mcb.Manager
}

// GetReservedCapacityManager returns mock implementation of ReservedCapacityManager
func (mcb *MockCapacityManagerBuilder) GetReservedCapacityManager(_ *logrus.Entry,
	_ CapacityReader, _ ReservationReader,
) CapacityPlaner {
	return mcb.Manager
}

func getCapReaderMock(acList []*accrd.AvailableCapacity, err error) *CapacityReaderMock {
	acListV := make([]accrd.AvailableCapacity, len(acList))
	for i := 0; i < len(acList); i++ {
		acListV[i] = *acList[i]
	}
	capReaderMock := &CapacityReaderMock{}
	capReaderMock.On("ReadCapacity", mock.Anything).Return(
		acListV, err)
	return capReaderMock
}

func getResReaderMock(acrList []*acrcrd.AvailableCapacityReservation, err error) *ReservationReaderMock {
	acrListV := make([]acrcrd.AvailableCapacityReservation, len(acrList))
	for i := 0; i < len(acrList); i++ {
		acrListV[i] = *acrList[i]
	}
	resReaderMock := &ReservationReaderMock{}
	resReaderMock.On("ReadReservations", mock.Anything).Return(
		acrListV, err)
	return resReaderMock
}
