package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
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
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, k8s.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = apiV1.StorageClassHDD
		requiredBytes = int64(util.GBYTE)
		expectedAC    = &accrd.AvailableCapacity{
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

	svc.acProvider = acProvider
	acProvider.On("SearchAC", ctxWithID, requiredNode, requiredBytes, requiredSC).
		Return(expectedAC).Times(1)

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
		svc           = setupVOOperationsTest(t)
		acProvider    = &mocks.ACOperationsMock{}
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, k8s.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = apiV1.StorageClassHDD
		requiredBytes = int64(util.GBYTE)
		expectedAC    = &accrd.AvailableCapacity{
			Spec: api.AvailableCapacity{
				Location:     testLVG.Spec.Name,
				NodeId:       testLVG.Spec.Node,
				StorageClass: apiV1.StorageClassHDDLVG,
				Size:         testLVG.Spec.Size,
			},
		}
	)
	err := svc.k8sClient.CreateCR(context.Background(), testLVG.Name, &testLVG)
	assert.Nil(t, err)

	svc.acProvider = acProvider
	acProvider.On("SearchAC", ctxWithID, requiredNode, requiredBytes, requiredSC).
		Return(expectedAC).Times(1)

	createdVolume, err := svc.CreateVolume(testCtx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, err)
	expectedVolume := &api.Volume{
		Id:                volumeID,
		Location:          expectedAC.Spec.Location,
		StorageClass:      expectedAC.Spec.StorageClass,
		NodeId:            expectedAC.Spec.NodeId,
		Size:              requiredBytes,
		CSIStatus:         apiV1.Creating,
		Health:            apiV1.HealthGood,
		LocationType:      apiV1.LocationTypeLVM,
		OperationalStatus: apiV1.OperationalStatusOperative,
	}
	assert.Equal(t, expectedVolume, createdVolume)
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
		ctxWithID     = context.WithValue(testCtx, k8s.RequestUUID, volumeID)
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

func TestVolumeOperationsImpl_DeleteVolume_NotFound(t *testing.T) {
	svc := setupVOOperationsTest(t)

	err := svc.DeleteVolume(testCtx, "unknown-volume")
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))
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

func TestVolumeOperationsImpl_ReadVolumeAndChangeStatus(t *testing.T) {
	svc := setupVOOperationsTest(t)

	var (
		v             = testVolume1
		updatedVolume = volumecrd.Volume{}
		newStatus     = apiV1.Created
		err           error
	)

	v.Spec.CSIStatus = apiV1.Creating
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &v)
	assert.Nil(t, err)

	err = svc.ReadVolumeAndChangeStatus(testVolume1Name, newStatus)
	assert.Nil(t, err)

	err = svc.k8sClient.ReadCR(testCtx, testVolume1Name, &updatedVolume)
	assert.Nil(t, err)
	assert.Equal(t, newStatus, updatedVolume.Spec.CSIStatus)

	// volume doesn't exist scenario
	err = svc.ReadVolumeAndChangeStatus("notExisting", newStatus)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestVolumeOperationsImpl_addVolumeToLVG(t *testing.T) {
	svc := setupVOOperationsTest(t)
	volumeID := "volumeID"
	err := svc.addVolumeToLVG(&testLVG, volumeID)
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	err = svc.k8sClient.CreateCR(context.Background(), testLVG.Name, &testLVG)
	assert.Nil(t, err)

	err = svc.addVolumeToLVG(&testLVG, volumeID)
	assert.Nil(t, err)
	lvg := &lvgcrd.LVG{}
	err = svc.k8sClient.ReadCR(context.Background(), testLVG.Name, lvg)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvg.Spec.VolumeRefs))

	err = svc.addVolumeToLVG(&testLVG, volumeID)
	assert.Nil(t, err)
	lvg = &lvgcrd.LVG{}
	err = svc.k8sClient.ReadCR(context.Background(), testLVG.Name, lvg)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvg.Spec.VolumeRefs))
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

	return NewVolumeOperationsImpl(k8sClient, testLogger)
}
