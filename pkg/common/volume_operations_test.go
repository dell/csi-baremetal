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

// TODO - refactor UTs https://github.com/dell/csi-baremetal/issues/371
/*func TestVolumeOperationsImpl_CreateVolume_VolumeExists(t *testing.T) {
	// 1. Volume CR has already exist
	svc := setupVOOperationsTest(t)

	v := testVolume1
	v.Spec.CSIStatus = apiV1.Created
	ctx := context.WithValue(testCtx, util.VolumeInfoKey, testNS)
	err := svc.k8sClient.CreateCR(ctx, testVolume1Name, &v)
	assert.Nil(t, err)

	createdVolume1, err := svc.CreateVolume(ctx, api.Volume{Id: v.Spec.Id})
	assert.Nil(t, err)
	assert.Equal(t, &v.Spec, createdVolume1)
}*/

// Volume CR was successfully created, HDD SC
/*func TestVolumeOperationsImpl_CreateVolume_HDDVolumeCreated(t *testing.T) {
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
			Usage:             apiV1.VolumeUsageInUse,
		}
	)

	ctx := context.WithValue(testCtx, util.VolumeInfoKey, testNS)

	capMBuilder, capMMock := getCapacityManagerMock()
	svc.capacityManagerBuilder = capMBuilder
	capMMock.On("PlanVolumesPlacing", ctxWithID, mock.Anything).
		Return(buildVolumePlacingPlan(testNode1Name, expectedVolume, expectedAC), nil).Times(1)

	createdVolume, err := svc.CreateVolume(ctx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, err)
	assert.Equal(t, expectedVolume, createdVolume)
}*/

// Volume CR was successfully created, HDDLVG SC
/*func TestVolumeOperationsImpl_CreateVolume_HDDLVGVolumeCreated(t *testing.T) {
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
			Usage:             apiV1.VolumeUsageInUse,
		}
		createdVolume *api.Volume
		err           error
	)

	// expect volume with "creating" CSIStatus, AC with HDDLVG exists and LogicalVolumeGroup has "created" status
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

	ctx := context.WithValue(testCtx, util.VolumeInfoKey, testNS)
	createdVolume, err = svc.CreateVolume(ctx, api.Volume{
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
	svc.cache.Set(v.Name, v.Namespace)
	ctx := context.WithValue(testCtx, util.VolumeInfoKey, v.Namespace)
	assert.Nil(t, svc.k8sClient.CreateCR(ctx, testVolume1Name, &v))

	createdVolume, err := svc.CreateVolume(ctx, api.Volume{Id: v.Spec.Id})
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
	ctx := context.WithValue(testCtx, util.VolumeInfoKey, testNS)
	err := svc.k8sClient.CreateCR(ctx, v.Name, &v)
	assert.Nil(t, err)

	createdVolume, err := svc.CreateVolume(ctx, api.Volume{Id: v.Name})
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

	ctx := context.WithValue(testCtx, util.VolumeInfoKey, testNS)
	createdVolume, err := svc.CreateVolume(ctx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.NotNil(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Nil(t, createdVolume)
}

// Fail to recreate AC from HDD to LogicalVolumeGroup
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
	acProvider.On("RecreateACToLVGSC", ctxWithID, mock.Anything, requiredSC, mock.Anything).
		Return(nil).Times(1)

	ctx := context.WithValue(testCtx, util.VolumeInfoKey, testNS)
	createdVolume, err := svc.CreateVolume(ctx, api.Volume{
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

	err = svc.DeleteVolume(testCtx, "unknown-namespace")
	assert.NotNil(t, err)

	svc = setupVOOperationsTest(t)

	svc.cache.Set("unknown-volume", testNS)
	err = svc.DeleteVolume(testCtx, "unknown-volume")
	assert.NotNil(t, err)

	svc = setupVOOperationsTest(t)
	volumeCR = testVolume1
	volumeCR.Spec.CSIStatus = apiV1.Removed
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	svc.cache.Set(volumeCR.Name, volumeCR.Namespace)
	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.Nil(t, err)

	svc = setupVOOperationsTest(t)
	svc.cache.Set(volumeCR.Name, volumeCR.Namespace)
	volumeCR = testVolume1
	volumeCR.Spec.CSIStatus = ""
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.NotNil(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	svc = setupVOOperationsTest(t)
	svc.cache.Set(volumeCR.Name, volumeCR.Namespace)
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

	svc.cache.Set(v.Name, v.Namespace)
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
		svc.cache.Set(v.Name, v.Namespace)
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
	svc.cache.Set(v.Name, v.Namespace)
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	err = svc.DeleteVolume(testCtx, testVolume1Name)
	assert.Nil(t, err)

	err = svc.k8sClient.ReadCR(testCtx, testVolume1Name, testVolume1.Namespace, &updatedVol)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Removing, updatedVol.Spec.CSIStatus)
}

func TestVolumeOperationsImpl_WaitStatus_Success(t *testing.T) {
	svc := setupVOOperationsTest(t)

	v := testVolume1
	v.Spec.CSIStatus = apiV1.Created
	svc.cache.Set(v.Name, v.Namespace)
	err := svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	ctx, closeFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeFn()

	err = svc.WaitStatus(ctx, v.Name, apiV1.Failed, apiV1.Created)
	assert.Nil(t, err)
}

func TestVolumeOperationsImpl_WaitStatus_Fails(t *testing.T) {
	svc := setupVOOperationsTest(t)
	// namespace wasn't found
	err := svc.WaitStatus(testCtx, "unknown_name", apiV1.Created)
	assert.NotNil(t, err)
	// volume CR wasn't found scenario
	svc.cache.Set("unknown_name", testNS)
	err = svc.WaitStatus(testCtx, "unknown_name", apiV1.Created)
	assert.NotNil(t, err)
	// ctx is done scenario
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &testVolume1)
	svc.cache.Set(testVolume1.Name, testVolume1.Namespace)
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
	svc1.cache.Set(testVolume1Name, v1.Namespace)
	svc1.UpdateCRsAfterVolumeDeletion(testCtx, testVolume1Name)

	err = svc1.k8sClient.ReadCR(testCtx, testVolume1Name, v1.Namespace, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// create AC, LogicalVolumeGroup and Volume
	err = svc1.k8sClient.CreateCR(testCtx, testAC4Name, &testAC4)
	assert.Nil(t, err)
	err = svc1.k8sClient.CreateCR(testCtx, testLVGName, &testLVG)
	assert.Nil(t, err)
	v1.Spec.StorageClass = apiV1.StorageClassHDDLVG
	v1.Spec.Location = testLVGName
	err = svc1.k8sClient.CreateCR(testCtx, testVolume1Name, &v1)
	svc1.cache.Set(testVolume1Name, v1.Namespace)
	assert.Nil(t, err)

	svc1.UpdateCRsAfterVolumeDeletion(testCtx, testVolume1Name)
	// check that Volume was removed
	err = svc1.k8sClient.ReadCR(testCtx, testVolume1Name, v1.Namespace, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// check that AC size was increased
	var updatedAC = &accrd.AvailableCapacity{}
	err = svc1.k8sClient.ReadCR(testCtx, testAC4Name, "", updatedAC)
	assert.Nil(t, err)
	assert.Equal(t, testAC4.Spec.Size+v1.Spec.Size, updatedAC.Spec.Size)
}

func TestVolumeOperationsImpl_ExpandVolume_DifferentStatuses(t *testing.T) {
	var (
		svc      *VolumeOperationsImpl
		err      error
		capacity = int64(util.GBYTE) * 10
	)

	volumeCR := testVolume1
	volumeCR.Spec.StorageClass = apiV1.StorageClassSystemLVG
	svc = setupVOOperationsTest(t)
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &volumeCR)
	volAC := testAC2
	volAC.Spec.Location = testDrive1UUID
	err = svc.k8sClient.CreateCR(testCtx, testAC2Name, &volAC)
	assert.Nil(t, err)

	for _, v := range [2]string{apiV1.Resizing, apiV1.Resized} {
		volumeCR.Spec.CSIStatus = v
		err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
		assert.Nil(t, err)
		err := svc.ExpandVolume(testCtx, &volumeCR, capacity)
		assert.Nil(t, err)
	}
	for _, v := range [3]string{apiV1.VolumeReady, apiV1.Created, apiV1.Published} {
		volumeCR.Spec.CSIStatus = v
		err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
		assert.Nil(t, err)

		err := svc.ExpandVolume(testCtx, &volumeCR, capacity)
		assert.Nil(t, err)
		uVol := &volumecrd.Volume{}
		err = svc.k8sClient.ReadCR(testCtx, volumeCR.Spec.Id, testNS, uVol)
		assert.Nil(t, err)
		assert.Equal(t, apiV1.Resizing, uVol.Spec.CSIStatus)
	}
}

func TestVolumeOperationsImpl_ExpandVolume_Fail(t *testing.T) {
	var (
		svc      *VolumeOperationsImpl
		err      error
		capacity = int64(util.TBYTE)
	)

	volumeCR := testVolume1
	svc = setupVOOperationsTest(t)
	err = svc.k8sClient.CreateCR(testCtx, volumeCR.Spec.Id, &volumeCR)

	for _, v := range [4]string{apiV1.Failed, apiV1.Removed, apiV1.Removing, apiV1.Creating} {
		volumeCR.Spec.CSIStatus = v
		err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
		assert.Nil(t, err)
		err := svc.ExpandVolume(testCtx, &volumeCR, capacity)
		assert.NotNil(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	}

	// Storage class is not lvg
	volumeCR.Spec.CSIStatus = apiV1.Created
	err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
	err = svc.ExpandVolume(testCtx, &volumeCR, capacity)
	assert.NotNil(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	// Failed to get AC
	volumeCR.Spec.StorageClass = apiV1.StorageClassSystemLVG
	err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
	err = svc.ExpandVolume(testCtx, &volumeCR, capacity)
	assert.NotNil(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	//Required capacity is more than capacity of AC
	volAC := testAC2
	volAC.Spec.Location = testDrive1UUID
	err = svc.k8sClient.CreateCR(testCtx, testAC2Name, &volAC)
	assert.Nil(t, err)
	err = svc.ExpandVolume(testCtx, &volumeCR, capacity)
	assert.NotNil(t, err)
	assert.Equal(t, codes.OutOfRange, status.Code(err))
}

func TestVolumeOperationsImpl_UpdateCRsAfterVolumeExpansion(t *testing.T) {
	var (
		svc *VolumeOperationsImpl
		err error
	)

	volumeCR := testVolume1
	svc = setupVOOperationsTest(t)
	svc.cache.Set(volumeCR.Spec.Id, testNS)
	volumeCR.Spec.CSIStatus = apiV1.Failed
	err = svc.k8sClient.CreateCR(testCtx, volumeCR.Spec.Id, &volumeCR)
	volAC := testAC2
	volAC.Spec.Location = testDrive1UUID
	err = svc.k8sClient.CreateCR(testCtx, testAC2Name, &volAC)
	assert.Nil(t, err)

	// volume doesn't have annotation
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)

	capacity, err := svc.crHelper.GetACByLocation(volumeCR.Spec.Location)
	assert.Nil(t, err)
	assert.Equal(t, volAC.Spec.Size, capacity.Spec.Size)

	// volume has annotation and status failed
	volumeCR.Annotations = map[string]string{apiV1.VolumePreviousCapacity: strconv.FormatInt(int64(util.MBYTE), 10)}
	err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
	pAC, err := svc.crHelper.GetACByLocation(volumeCR.Spec.Location)
	assert.Nil(t, err)
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)

	err = svc.k8sClient.ReadCR(testCtx, volumeCR.Name, volumeCR.Namespace, &volumeCR)
	assert.Nil(t, err)
	capacity, err = svc.crHelper.GetACByLocation(volumeCR.Spec.Location)
	assert.Nil(t, err)
	assert.Equal(t, pAC.Spec.Size+int64(util.GBYTE)*100-int64(util.MBYTE), capacity.Spec.Size)

	// volume has resized status and doesn't have annotation
	volumeCR.Spec.CSIStatus = apiV1.Resized
	err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
	assert.Nil(t, err)
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)
	err = svc.k8sClient.ReadCR(testCtx, volumeCR.Name, volumeCR.Namespace, &volumeCR)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Resized, volumeCR.Spec.CSIStatus)

	// volume has resized status
	volumeCR.Annotations = map[string]string{apiV1.VolumePreviousStatus: apiV1.Created}
	err = svc.k8sClient.UpdateCR(testCtx, &volumeCR)
	assert.Nil(t, err)
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)
	err = svc.k8sClient.ReadCR(testCtx, volumeCR.Name, volumeCR.Namespace, &volumeCR)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Created, volumeCR.Spec.CSIStatus)
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
	lvg := &lvgcrd.LogicalVolumeGroup{}
	err = svc.k8sClient.ReadCR(context.Background(), testLVG.Name, "", lvg)
	assert.True(t, k8sError.IsNotFound(err))
	ac := &accrd.AvailableCapacity{}
	err = svc.k8sClient.ReadCR(context.Background(), testAC4.Name, "", ac)
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

	return NewVolumeOperationsImpl(k8sClient, testLogger, cache.NewMemCache(), featureconfig.NewFeatureConfig())
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
}*/
