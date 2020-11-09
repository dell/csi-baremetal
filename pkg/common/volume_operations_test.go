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

package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

func TestVolumeOperationsImpl_CreateVolume_VolumeExists(t *testing.T) {
	// 1. Volume CR has already exist
	svc := setupVOOperationsTest(t)

	v := testVolume1
	v.Spec.CSIStatus = apiV1.Created
	err := svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	createdVolume1, err := svc.CreateVolume(testCtx, api.Volume{Id: v.Spec.Id})
	assert.Nil(t, err)
	assert.Equal(t, &v.Spec, createdVolume1)
}

// Volume CR was successfully created, HDD SC
func TestVolumeOperationsImpl_CreateVolume_HDDVolumeCreated(t *testing.T) {
	var (
		svc           = setupVOOperationsTest(t)
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = apiV1.StorageClassHDD
		requiredBytes = int64(util.GBYTE)
		expectedAC    = &accrd.AvailableCapacity{
			ObjectMeta: v1.ObjectMeta{
				Name: "testAC"},
			Spec: api.AvailableCapacity{
				Location:     testDrive1UUID,
				NodeId:       testNode1Name,
				StorageClass: requiredSC,
				Size:         int64(util.GBYTE) * 42,
			},
		}
		expectedVolume = &api.Volume{
			Id:                volumeID,
			Location:          expectedAC.Spec.Location,
			StorageClass:      expectedAC.Spec.StorageClass,
			NodeId:            expectedAC.Spec.NodeId,
			Size:              expectedAC.Spec.Size,
			CSIStatus:         apiV1.Creating,
			Health:            apiV1.HealthGood,
			LocationType:      apiV1.LocationTypeDrive,
			OperationalStatus: apiV1.OperationalStatusOperative,
		}
	)

	capMBuilder, capMMock := getCapacityManagerMock()
	svc.capacityManagerBuilder = capMBuilder
	capMMock.On("PlanVolumesPlacing", ctxWithID, mock.Anything).
		Return(buildVolumePlacingPlan(testNode1Name, expectedVolume, expectedAC), nil).Times(1)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, err)
	assert.Equal(t, expectedVolume, createdVolume)
}

// Volume CR was successfully created, HDDLVG SC
func TestVolumeOperationsImpl_CreateVolume_HDDLVGVolumeCreated(t *testing.T) {
	var (
		svc           *VolumeOperationsImpl
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = apiV1.StorageClassHDDLVG
		requiredBytes = int64(util.GBYTE)
		acToReturn    = accrd.AvailableCapacity{
			Spec: api.AvailableCapacity{
				Location:     testLVG.Spec.Name,
				NodeId:       testLVG.Spec.Node,
				StorageClass: apiV1.StorageClassHDDLVG,
				Size:         testLVG.Spec.Size,
			},
		}
		expectedVolume = api.Volume{
			Id:                volumeID,
			Location:          acToReturn.Spec.Location,
			StorageClass:      requiredSC,
			NodeId:            acToReturn.Spec.NodeId,
			Size:              requiredBytes,
			CSIStatus:         apiV1.Creating,
			Health:            apiV1.HealthGood,
			LocationType:      apiV1.LocationTypeLVM,
			OperationalStatus: apiV1.OperationalStatusOperative,
		}
		createdVolume *api.Volume
		err           error
	)

	// expect volume with "creating" CSIStatus, AC with HDDLVG exists and LVG has "created" status
	svc = setupVOOperationsTest(t)
	svc.acProvider = acProvider
	recreatedAC := acToReturn
	recreatedAC.Spec.StorageClass = requiredSC
	capMBuilder, capMMock := getCapacityManagerMock()
	svc.capacityManagerBuilder = capMBuilder
	capMMock.On("PlanVolumesPlacing", ctxWithID, mock.Anything).
		Return(buildVolumePlacingPlan(testNode1Name, &expectedVolume, &acToReturn), nil).Times(1)
	acProvider.On("RecreateACToLVGSC", ctxWithID, requiredSC, &acToReturn).
		Return(&recreatedAC).Times(1)

	createdVolume, err = svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, err)
	assert.NotNil(t, createdVolume)
	assert.Equal(t, expectedVolume, *createdVolume)
}

// Volume CR exists and has "failed" CSIStatus
func TestVolumeOperationsImpl_CreateVolume_FaileCauseExist(t *testing.T) {
	svc := setupVOOperationsTest(t)

	v := testVolume1
	v.Spec.CSIStatus = apiV1.Failed
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v))

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{Id: v.Spec.Id})
	assert.NotNil(t, err)
	assert.Nil(t, createdVolume)
}

// Volume CR exists and timeout for creation exceeded
func TestVolumeOperationsImpl_CreateVolume_FailCauseTimeout(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1
	)
	v.ObjectMeta.CreationTimestamp = v1.Time{
		Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local),
	}
	err := svc.k8sClient.CreateCR(testCtx, v.Name, &v)
	assert.Nil(t, err)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{Id: v.Name})
	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.Internal, "Unable to create volume in allocated time"), err)
	assert.Nil(t, createdVolume)
}

// There is no suitable AC
func TestVolumeOperationsImpl_CreateVolume_FailNoAC(t *testing.T) {
	var (
		svc           = setupVOOperationsTest(t)
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = apiV1.StorageClassHDD
		requiredBytes = int64(util.GBYTE)
	)

	svc.acProvider = acProvider
	acProvider.On("SearchAC", ctxWithID, requiredNode, requiredBytes, requiredSC).
		Return(nil).Times(1)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.NotNil(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Nil(t, createdVolume)
}

// Fail to recreate AC from HDD to LVG
func TestVolumeOperationsImpl_CreateVolume_FailRecreateAC(t *testing.T) {
	var (
		svc           *VolumeOperationsImpl
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = apiV1.StorageClassHDDLVG
		requiredBytes = int64(util.GBYTE)
		acToReturn    = accrd.AvailableCapacity{
			Spec: api.AvailableCapacity{
				StorageClass: apiV1.StorageClassHDD,
			},
		}
		expectedVolume = api.Volume{
			Id: volumeID,
		}
	)

	svc = setupVOOperationsTest(t)
	svc.acProvider = acProvider

	capMBuilder, capMMock := getCapacityManagerMock()
	svc.capacityManagerBuilder = capMBuilder
	capMMock.On("PlanVolumesPlacing", ctxWithID, mock.Anything).
		Return(buildVolumePlacingPlan(testNode1Name, &expectedVolume, &acToReturn), nil).Times(1)
	acProvider.On("RecreateACToLVGSC", ctxWithID, requiredSC, mock.Anything).
		Return(nil).Times(1)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, createdVolume)
	assert.NotNil(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestVolumeOperationsImpl_DeleteVolume_DifferentStatuses(t *testing.T) {
	var (
		svc      *VolumeOperationsImpl
		err      error
		volumeCR volumecrd.Volume
	)

	svc = setupVOOperationsTest(t)

	err = svc.DeleteVolume(testCtx, "unknown-volume")
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	svc = setupVOOperationsTest(t)
	volumeCR = testVolume1
	volumeCR.Spec.CSIStatus = apiV1.Removed
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.Nil(t, err)

	svc = setupVOOperationsTest(t)
	volumeCR = testVolume1
	volumeCR.Spec.CSIStatus = ""
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.NotNil(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	svc = setupVOOperationsTest(t)
	volumeCR = testVolume1
	volumeCR.Spec.Ephemeral = true
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.NotNil(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestVolumeOperationsImpl_DeleteVolume_FailToRemoveSt(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1
		err error
	)

	v.Spec.CSIStatus = apiV1.Failed
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	err = svc.DeleteVolume(testCtx, testVolume1Name)
	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.Internal, "volume has reached failed status"), err)
}

// volume has status Removed or Removing
func TestVolumeOperationsImpl_DeleteVolume(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1
		err error
	)

	for _, st := range []string{apiV1.Removing, apiV1.Removed} {
		v.Spec.CSIStatus = st
		err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
		assert.Nil(t, err)

		err = svc.DeleteVolume(testCtx, testVolume1Name)
		assert.Nil(t, err)
	}
}

func TestVolumeOperationsImpl_DeleteVolume_SetStatus(t *testing.T) {
	var (
		svc        = setupVOOperationsTest(t)
		v          = testVolume1
		updatedVol = volumecrd.Volume{}
		err        error
	)

	v.Spec.CSIStatus = apiV1.Created
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	err = svc.DeleteVolume(testCtx, testVolume1Name)
	assert.Nil(t, err)

	err = svc.k8sClient.ReadCR(testCtx, testVolume1Name, &updatedVol)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Removing, updatedVol.Spec.CSIStatus)
}

func TestVolumeOperationsImpl_WaitStatus_Success(t *testing.T) {
	svc := setupVOOperationsTest(t)

	v := testVolume1
	v.Spec.CSIStatus = apiV1.Created
	err := svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	ctx, closeFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeFn()

	err = svc.WaitStatus(ctx, v.Name, apiV1.Failed, apiV1.Created)
	assert.Nil(t, err)
}

func TestVolumeOperationsImpl_WaitStatus_Fails(t *testing.T) {
	svc := setupVOOperationsTest(t)

	// volume CR wasn't found scenario
	err := svc.WaitStatus(testCtx, "unknown_name", apiV1.Created)
	assert.NotNil(t, err)
	// ctx is done scenario
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &testVolume1)
	assert.Nil(t, err)

	ctx, closeFn := context.WithTimeout(context.Background(), 10*time.Second)
	closeFn()
	ctx.Done()

	// volume CR wasn't found
	err = svc.WaitStatus(ctx, testVolume1Name, apiV1.Created)
	assert.NotNil(t, err)
}

func TestVolumeOperationsImpl_UpdateCRsAfterVolumeDeletion(t *testing.T) {
	var err error

	svc1 := setupVOOperationsTest(t)

	// 1. volume with HDDLVG SC, corresponding AC should be increased, volume CR should be removed
	v1 := testVolume1
	err = svc1.k8sClient.CreateCR(testCtx, testVolume1Name, &v1)
	assert.Nil(t, err)

	svc1.UpdateCRsAfterVolumeDeletion(testCtx, testVolume1Name)

	err = svc1.k8sClient.ReadCR(testCtx, testVolume1Name, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// create AC, LVG and Volume
	err = svc1.k8sClient.CreateCR(testCtx, testAC4Name, &testAC4)
	assert.Nil(t, err)
	err = svc1.k8sClient.CreateCR(testCtx, testLVGName, &testLVG)
	assert.Nil(t, err)
	v1.Spec.StorageClass = apiV1.StorageClassHDDLVG
	v1.Spec.Location = testLVGName
	err = svc1.k8sClient.CreateCR(testCtx, testVolume1Name, &v1)
	assert.Nil(t, err)

	svc1.UpdateCRsAfterVolumeDeletion(testCtx, testVolume1Name)
	// check that Volume was removed
	err = svc1.k8sClient.ReadCR(testCtx, testVolume1Name, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// check that AC size was increased
	var updatedAC = &accrd.AvailableCapacity{}
	err = svc1.k8sClient.ReadCR(testCtx, testAC4Name, updatedAC)
	assert.Nil(t, err)
	assert.Equal(t, testAC4.Spec.Size+v1.Spec.Size, updatedAC.Spec.Size)
}

func TestVolumeOperationsImpl_deleteLVGIfVolumesNotExistOrUpdate(t *testing.T) {
	svc := setupVOOperationsTest(t)
	volumeID := "volumeID"
	volumeID1 := "volumeID1"

	// CR not found error
	testLVG.Spec.VolumeRefs = [](string){volumeID, volumeID1}
	isDeleted, err := svc.deleteLVGIfVolumesNotExistOrUpdate(&testLVG, volumeID, &testAC4)
	assert.False(t, isDeleted)
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	err = svc.k8sClient.CreateCR(context.Background(), testLVG.Name, &testLVG)
	assert.Nil(t, err)
	err = svc.k8sClient.CreateCR(context.Background(), testAC4.Name, &testAC4)
	assert.Nil(t, err)

	// test deletion
	isDeleted, err = svc.deleteLVGIfVolumesNotExistOrUpdate(&testLVG, volumeID, &testAC4)
	assert.True(t, isDeleted)
	assert.Nil(t, err)
	lvg := &lvgcrd.LVG{}
	err = svc.k8sClient.ReadCR(context.Background(), testLVG.Name, lvg)
	assert.True(t, k8sError.IsNotFound(err))
	ac := &accrd.AvailableCapacity{}
	err = svc.k8sClient.ReadCR(context.Background(), testAC4.Name, ac)
	assert.True(t, k8sError.IsNotFound(err))

	// try to remove again
	isDeleted, err = svc.deleteLVGIfVolumesNotExistOrUpdate(&testLVG, volumeID, &testAC4)
	assert.False(t, isDeleted)
	assert.True(t, k8sError.IsNotFound(err))
}

// creates fake k8s client and creates AC CRs based on provided acs
// returns instance of ACOperationsImpl based on created k8s client
func setupVOOperationsTest(t *testing.T) *VolumeOperationsImpl {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)
	assert.NotNil(t, k8sClient)

	return NewVolumeOperationsImpl(k8sClient, testLogger, featureconfig.NewFeatureConfig())
}

func buildVolumePlacingPlan(node string, vol *api.Volume,
	ac *accrd.AvailableCapacity) *capacityplanner.VolumesPlacingPlan {
	return capacityplanner.NewVolumesPlacingPlan(
		capacityplanner.VolumesPlanMap{
			node: capacityplanner.VolToACMap{
				vol: ac,
			},
		},
		capacityplanner.NodeCapacityMap{
			node: capacityplanner.ACMap{ac.Name: ac},
		})
}

func getCapacityManagerMock() (capacityplanner.CapacityManagerBuilder, *capacityplanner.PlannerMock) {
	plannerMock := &capacityplanner.PlannerMock{}
	return &capacityplanner.MockCapacityManagerBuilder{Manager: plannerMock}, plannerMock
}
