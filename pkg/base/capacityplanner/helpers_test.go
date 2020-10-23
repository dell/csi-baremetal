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

func TestReservationHelper_ReleaseReservation(t *testing.T) {
	logger := testLogger.WithField("component", "test")
	ctx := context.Background()

	callReleaseReservation := func(client *k8s.KubeClient, capReader CapacityReader, resReader ReservationReader,
		vol *genV1.Volume, ac, acReplacement *accrd.AvailableCapacity) error {
		rh := createReservationHelper(t, logger, capReader, resReader, client)
		return rh.ReleaseReservation(ctx, vol, ac, acReplacement)
	}
	t.Run("Error update data", func(t *testing.T) {
		rh := createReservationHelper(t, logger,
			getCapReaderMock(nil, testErr),
			getResReaderMock(nil, testErr),
			getKubeClient(t))
		err := rh.ReleaseReservation(ctx, nil, nil, nil)
		assert.Equal(t, testErr, err)
	})
	t.Run("Reservation not found", func(t *testing.T) {
		testACs := []*accrd.AvailableCapacity{
			getTestAC("", testSmallSize, apiV1.StorageClassHDD),
		}
		err := callReleaseReservation(
			getKubeClient(t),
			getCapReaderMock(testACs, nil),
			getResReaderMock(nil, nil),
			getTestVol("", testSmallSize, apiV1.StorageClassHDD),
			testACs[0], testACs[0])
		assert.Nil(t, err)
	})
	t.Run("Should remove right ACR", func(t *testing.T) {
		testACs := []*accrd.AvailableCapacity{
			getTestAC("", testSmallSize, apiV1.StorageClassHDD),
			getTestAC("", testSmallSize, apiV1.StorageClassHDD),
		}
		testACRs := []*acrcrd.AvailableCapacityReservation{
			// ACR has 1 AC, but size don't match
			getTestACR(testLargeSize, apiV1.StorageClassHDD, testACs[:1]),
			// ACR size match, has 2 AC
			getTestACR(testSmallSize, apiV1.StorageClassHDD, testACs),
			// ACR size match, has 1 AC, this ACR should be removed
			getTestACR(testSmallSize, apiV1.StorageClassHDD, testACs[:1]),
		}
		client := getKubeClient(t)
		createACRsInAPi(t, client, testACRs)
		err := callReleaseReservation(
			client,
			getCapReaderMock(testACs, nil),
			getResReaderMock(testACRs, nil),
			getTestVol("", testSmallSize, apiV1.StorageClassHDD),
			testACs[0], testACs[0])
		assert.Nil(t, err)
		checkACRNotExist(t, client, testACRs[2])
	})
	t.Run("Replace AC in ACR", func(t *testing.T) {
		replacementAC := getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDDLVG)
		testACs := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
		}
		testACRs := []*acrcrd.AvailableCapacityReservation{
			getTestACR(testSmallSize, apiV1.StorageClassHDDLVG, testACs),
			getTestACR(testSmallSize, apiV1.StorageClassHDDLVG, testACs),
		}
		client := getKubeClient(t)
		createACRsInAPi(t, client, testACRs)
		err := callReleaseReservation(
			client,
			getCapReaderMock(testACs, nil),
			getResReaderMock(testACRs, nil),
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
			testACs[0], replacementAC)
		assert.Nil(t, err)
		acrList := &acrcrd.AvailableCapacityReservationList{}
		err = client.List(ctx, acrList)
		assert.Nil(t, err)
		assert.Contains(t, acrList.Items[0].Spec.Reservations, replacementAC.Name)
	})
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
	err := client.ReadCR(context.Background(), acr.Name, acr)
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
