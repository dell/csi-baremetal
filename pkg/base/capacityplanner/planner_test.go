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
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
)

var (
	testErr             = errors.New("test error")
	testLogger          = logrus.New()
	testNS              = "default"
	testNode1           = uuid.New().String()
	testNode2           = uuid.New().String()
	testSmallSize int64 = 10737418240
	testLargeSize       = testSmallSize * 2
)

func getTestVol(nodeID string, size int64, sc string) *genV1.Volume {
	return &genV1.Volume{
		Id:           uuid.New().String(),
		StorageClass: sc,
		Size:         size,
		NodeId:       nodeID,
	}
}

func getTestAC(nodeID string, size int64, sc string) *accrd.AvailableCapacity {
	return &accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1VersionOld},
		ObjectMeta: k8smetav1.ObjectMeta{Name: uuid.New().String(), Namespace: testNS},
		Spec: genV1.AvailableCapacity{
			Size:         size,
			StorageClass: sc,
			NodeId:       nodeID,
		},
	}
}

func getTestACR(size int64, sc string,
	acList []*accrd.AvailableCapacity) *acrcrd.AvailableCapacityReservation {
	acNames := make([]string, len(acList))
	for i, ac := range acList {
		acNames[i] = ac.Name
	}
	return &acrcrd.AvailableCapacityReservation{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacityReservation", APIVersion: apiV1.APIV1VersionOld},
		ObjectMeta: k8smetav1.ObjectMeta{Name: uuid.New().String(), Namespace: testNS},
		Spec: genV1.AvailableCapacityReservation{
			Size:         size,
			StorageClass: sc,
			Reservations: acNames,
		},
	}
}

func TestCapacityManager(t *testing.T) {
	logger := testLogger.WithField("component", "test")
	ctx := context.Background()

	callPlanVolumesPlacing := func(capRead CapacityReader, volumes []*genV1.Volume) (*VolumesPlacingPlan, error) {
		capManager := NewCapacityManager(logger, capRead)
		return capManager.PlanVolumesPlacing(ctx, volumes)
	}
	t.Run("Failed to read capacity", func(t *testing.T) {
		capManager := NewCapacityManager(logger, getCapReaderMock(nil, testErr))
		plan, err := capManager.PlanVolumesPlacing(ctx,
			[]*genV1.Volume{getTestVol(testNode1, testSmallSize, apiV1.StorageClassHDD)})
		assert.Nil(t, plan)
		assert.Error(t, err)
	})
	t.Run("Capacity not found", func(t *testing.T) {
		// no capacity
		testVols := []*genV1.Volume{
			getTestVol(testNode1, testLargeSize, apiV1.StorageClassHDD),
		}
		var testACs []*accrd.AvailableCapacity
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACs, nil), testVols)
		assert.Nil(t, plan)
		assert.Nil(t, err)
		// no enough capacity
		testACs = []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
		}
		plan, err = callPlanVolumesPlacing(getCapReaderMock(testACs, nil), testVols)
		assert.Nil(t, plan)
		assert.Nil(t, err)
	})
	t.Run("Capacity not found for some volumes", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol(testNode1, testSmallSize, apiV1.StorageClassHDD),
			getTestVol(testNode1, testSmallSize, apiV1.StorageClassHDD),
		}
		testACs := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACs, nil), testVols)
		assert.Nil(t, plan)
		assert.Nil(t, err)
	})
	t.Run("Smoke test", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol(testNode1, testSmallSize, apiV1.StorageClassHDD),
		}
		testACs := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACs, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, VolumesPlanMap{
				testNode1: VolToACMap{testVols[0]: testACs[0]}}, plan.plan)
		}

	})
	t.Run("Multiple volumes", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol(testNode1, testSmallSize, apiV1.StorageClassHDD),
			getTestVol(testNode1, testLargeSize, apiV1.StorageClassHDDLVG),
		}
		testACs := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
			getTestAC(testNode1, testLargeSize, apiV1.StorageClassHDDLVG),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACs, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
	})
	t.Run("ANY StorageClass", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol(testNode1, testSmallSize, apiV1.StorageClassAny),
			getTestVol(testNode1, testSmallSize, apiV1.StorageClassAny),
			getTestVol(testNode1, testSmallSize, apiV1.StorageClassAny),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassSSD),
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDDLVG),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACS, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			// check that each vol has its own AC
			usedAC := map[string]struct{}{}
			for _, vol := range testVols {
				usedAC[plan.plan[testNode1][vol].Name] = struct{}{}
			}
			assert.Len(t, usedAC, len(testVols))
		}
	})
	t.Run("Find AC on multiple nodes", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassAny),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassSSD),
			getTestAC(testNode2, testSmallSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACS, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[0]))
			assert.Equal(t, testACS[1], plan.GetACForVolume(testNode2, testVols[0]))
			assert.ElementsMatch(t, testACS, plan.GetACsForVolumes()[testVols[0]])
		}
	})
	t.Run("Using sub class for LVG", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize+LvgDefaultMetadataSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACS, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[0]))
		}
	})
	t.Run("Using sub class for LVG", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize+LvgDefaultMetadataSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACS, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[0]))
		}
	})
	t.Run("Multiple LVM volumes on same drive", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, (testSmallSize*2)+LvgDefaultMetadataSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACS, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[0]))
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[1]))
		}
	})
	t.Run("Node selection", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, (testSmallSize*2)+LvgDefaultMetadataSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(getCapReaderMock(testACS, nil), testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[0]))
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[1]))
		}
	})
}

func TestReservedCapacityManager(t *testing.T) {
	logger := testLogger.WithField("component", "test")
	ctx := context.Background()

	callPlanVolumesPlacing := func(capRead CapacityReader,
		resRead ReservationReader, volumes []*genV1.Volume) (*VolumesPlacingPlan, error) {
		capManager := NewReservedCapacityManager(logger, capRead, resRead)
		return capManager.PlanVolumesPlacing(ctx, volumes)
	}
	t.Run("Failed to read capacity", func(t *testing.T) {
		capManager := NewReservedCapacityManager(logger,
			getCapReaderMock(nil, testErr),
			getResReaderMock(nil, testErr))
		plan, err := capManager.PlanVolumesPlacing(ctx,
			[]*genV1.Volume{getTestVol(testNode1, testSmallSize, apiV1.StorageClassHDD)})
		assert.Nil(t, plan)
		assert.Error(t, err)
	})
	t.Run("No reservations", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassAny),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
		}
		plan, err := callPlanVolumesPlacing(
			getCapReaderMock(testACS, nil),
			getResReaderMock(nil, nil),
			testVols)
		assert.Nil(t, plan)
		assert.Nil(t, err)
	})
	t.Run("Smoke", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassHDDLVG),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize+LvgDefaultMetadataSize, apiV1.StorageClassHDD),
		}
		testACRS := []*acrcrd.AvailableCapacityReservation{
			getTestACR(testSmallSize, apiV1.StorageClassHDDLVG, testACS),
		}
		plan, err := callPlanVolumesPlacing(
			getCapReaderMock(testACS, nil),
			getResReaderMock(testACRS, nil),
			testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[0]))
		}
	})
	t.Run("Should select AC from reservation which holds less ACs", func(t *testing.T) {
		testVols := []*genV1.Volume{
			getTestVol("", testSmallSize, apiV1.StorageClassAny),
		}
		testACS := []*accrd.AvailableCapacity{
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
			getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
			getTestAC(testNode2, testSmallSize, apiV1.StorageClassHDD),
		}
		testACRS := []*acrcrd.AvailableCapacityReservation{
			getTestACR(testSmallSize, apiV1.StorageClassAny, testACS[1:]),
			getTestACR(testSmallSize, apiV1.StorageClassAny, testACS[:1]),
		}
		plan, err := callPlanVolumesPlacing(
			getCapReaderMock(testACS, nil),
			getResReaderMock(testACRS, nil),
			testVols)
		assert.NotNil(t, plan)
		assert.Nil(t, err)
		if plan != nil {
			assert.Equal(t, testNode1, plan.SelectNode())
			assert.Equal(t, testACS[0], plan.GetACForVolume(testNode1, testVols[0]))
		}
	})
}
