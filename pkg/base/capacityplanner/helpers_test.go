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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

func TestReservationHelper_CreateReservation(t *testing.T) {
	logger := testLogger.WithField("component", "test")
	ctx := context.Background()
	rh := createReservationHelper(t, logger, nil, nil, getKubeClient(t))
	plan := getSimpleVolumePlacingPlan()

	reservation := genV1.AvailableCapacityReservation{}
	reservation.ReservationRequests = make([]*genV1.ReservationRequest, 0)
	for volume := range plan.GetACsForVolumes() {
		reservation.ReservationRequests = append(reservation.ReservationRequests, &genV1.ReservationRequest{
			CapacityRequest: &genV1.CapacityRequest{
				Name:         volume.Id,
				Size:         volume.Size,
				StorageClass: volume.StorageClass,
			},
		})
	}
	reservation.NodeRequests = &genV1.NodeRequests{Reserved: make([]string, 0)}
	name := "test"
	reservationResource := &acrcrd.AvailableCapacityReservation{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       reservation,
	}
	// request reservation
	err := rh.client.CreateCR(ctx, name, reservationResource)
	assert.Nil(t, err)
	// confirm reservation
	err = rh.UpdateReservation(ctx, plan, nil, reservationResource)
	assert.Nil(t, err)
	// check reservations exist
	acrList := &acrcrd.AvailableCapacityReservationList{}
	err = rh.client.ReadList(ctx, acrList)
	if err != nil {
		t.FailNow()
	}
	assert.Len(t, acrList.Items, 1)
	assert.Len(t, acrList.Items[0].Spec.ReservationRequests[0].Reservations, 2)
	assert.Equal(t, acrList.Items[0].Spec.Status, apiV1.ReservationConfirmed)
}

func TestReservationHelper_ReleaseReservation(t *testing.T) {
	logger := testLogger.WithField("component", "test")
	ctx := context.Background()

	reservation := &acrcrd.AvailableCapacityReservation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-reservation"},
		Spec: genV1.AvailableCapacityReservation{ReservationRequests: []*genV1.ReservationRequest{
			{Reservations: []string{"uuid-1"}},
			{Reservations: []string{"uuid-2"}},
			{Reservations: []string{"uuid-3"}},
		}},
	}
	reservationList := []*acrcrd.AvailableCapacityReservation{reservation}
	client := getKubeClient(t)
	createACRsInAPi(t, client, reservationList)

	rh := createReservationHelper(t, logger, getCapReaderMock(nil, testErr),
		getResReaderMock(reservationList, testErr), client)

	// remove first
	err := rh.ReleaseReservation(ctx, reservation, 0)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(reservation.Spec.ReservationRequests))

	// remove last
	err = rh.ReleaseReservation(ctx, reservation, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(reservation.Spec.ReservationRequests))

	// delete reservation
	err = rh.ReleaseReservation(ctx, reservation, 0)
	assert.Nil(t, err)
	checkACRNotExist(t, client, reservation)
}

func TestReservationFilter(t *testing.T) {
	testACs := []accrd.AvailableCapacity{
		*getTestAC(testNode1, testLargeSize, apiV1.StorageClassHDD),
		*getTestAC(testNode1, testSmallSize, apiV1.StorageClassSSD),
		*getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
	}
	testACRs := []acrcrd.AvailableCapacityReservation{
		*getTestACR(testLargeSize, apiV1.StorageClassHDD, []*accrd.AvailableCapacity{&testACs[0]}),
		*getTestACR(testSmallSize, apiV1.StorageClassSSD, []*accrd.AvailableCapacity{&testACs[1]}),
	}
	t.Run("Filter reserved", func(t *testing.T) {
		filter := NewReservationFilter()
		assert.ElementsMatch(t, testACs[:2], filter.FilterByReservation(true, testACs, testACRs))
	})
	t.Run("Filter unreserved", func(t *testing.T) {
		filter := NewReservationFilter()
		assert.ElementsMatch(t, testACs[2:], filter.FilterByReservation(false, testACs, testACRs))
	})
}

func createACRsInAPi(t *testing.T, client *k8s.KubeClient, acrs []*acrcrd.AvailableCapacityReservation) {
	for _, acr := range acrs {
		err := client.CreateCR(context.Background(), acr.Name, acr)
		assert.Nil(t, err)
	}
}

func createACsInAPi(t *testing.T, client *k8s.KubeClient, acs []*accrd.AvailableCapacity) {
	for _, acs := range acs {
		err := client.CreateCR(context.Background(), acs.Name, acs)
		assert.Nil(t, err)
	}
}

func checkACRNotExist(t *testing.T, client *k8s.KubeClient, acr *acrcrd.AvailableCapacityReservation) {
	err := client.ReadCR(context.Background(), acr.Name, "", acr)
	assert.True(t, k8serrors.IsNotFound(err))
}

func createReservationHelper(t *testing.T, logger *logrus.Entry,
	capReader CapacityReader, resReader ReservationReader, client *k8s.KubeClient) *ReservationHelper {
	return NewReservationHelper(logger,
		client, capReader, resReader)
}

func getKubeClient(t *testing.T) *k8s.KubeClient {
	client, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)
	return client
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
