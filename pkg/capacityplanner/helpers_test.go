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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

func TestReservationHelper_CreateReservation(t *testing.T) {
	logger := testLogger.WithField("component", "test")
	ctx := context.Background()
	rh := createReservationHelper(t, logger, nil, nil)
	err := rh.CreateReservation(ctx, getSimpleVolumePlacingPlan())
	assert.Nil(t, err)
	// check reservations exist
	acrList := &acrcrd.AvailableCapacityReservationList{}
	err = rh.client.ReadList(ctx, acrList)
	if err != nil {
		t.FailNow()
	}
	assert.Len(t, acrList.Items, 1)
	assert.Len(t, acrList.Items[0].Spec.Reservations, 2)
}

func createReservationHelper(t *testing.T, logger *logrus.Entry,
	capReader CapacityReader, resReader ReservationReader) *ReservationHelper {
	k, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)
	return NewReservationHelper(logger,
		k, capReader, resReader)
}

func getSimpleVolumePlacingPlan() *VolumesPlacingPlan {
	testVol := getTestVol("", testSmallSize, apiV1.StorageClassAny)
	testAC1 := getTestAC(testNode1, testSmallSize, apiV1.StorageClassAny)
	testAC2 := getTestAC(testNode2, testLargeSize, apiV1.StorageClassAny)
	return &VolumesPlacingPlan{
		plan:     VolumesPlanMap{testNode1: VolToACMap{testVol: testAC1}, testNode2: VolToACMap{testVol: testAC2}},
		capacity: NodeCapacityMap{testNode1: ACMap{testAC1.Name: testAC1}, testNode2: ACMap{testAC2.Name: testAC2}},
	}
}
