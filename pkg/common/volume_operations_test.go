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
	"strconv"
	"testing"
	"time"

	"github.com/dell/csi-baremetal/pkg/base"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/cache"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

var (
	namespace = "my-namespace"
)

// creates fake k8s client and creates AC CRs based on provided acs
// returns instance of ACOperationsImpl based on created k8s client
func setupVOOperationsTest(t *testing.T) *VolumeOperationsImpl {
	k8sClient, err := k8s.GetFakeKubeClient(namespace, testLogger)
	assert.Nil(t, err)
	assert.NotNil(t, k8sClient)

	return NewVolumeOperationsImpl(k8sClient, testLogger, cache.NewMemCache(), featureconfig.NewFeatureConfig())
}

func Test_getPersistentVolumeClaimLabels(t *testing.T) {
	var (
		svc     = setupVOOperationsTest(t)
		ctx     = context.TODO()
		pvcName = "my-pvc"
	)
	// no PVC
	labels, err := svc.getPersistentVolumeClaimLabels(ctx, pvcName, namespace)
	assert.Nil(t, labels)
	assert.NotNil(t, err)

	// create PVC
	var (
		appName     = "my-app"
		releaseName = "my-release"
		pvcLabels   = map[string]string{
			k8s.AppLabelKey:     appName,
			k8s.ReleaseLabelKey: releaseName,
		}
		pvc = &v1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: namespace,
			Labels: pvcLabels}}
	)
	err = svc.k8sClient.Create(ctx, pvc)
	assert.Nil(t, err)

	// check labels
	labels, err = svc.getPersistentVolumeClaimLabels(ctx, pvcName, namespace)
	assert.NotNil(t, labels)
	assert.Nil(t, err)
	assert.Equal(t, labels[k8s.AppLabelKey], appName)
	assert.Equal(t, labels[k8s.AppLabelShortKey], appName)
	assert.Equal(t, labels[k8s.ReleaseLabelKey], releaseName)
}

func TestVolumeOperationsImpl_CreateVolume_VolumeExists(t *testing.T) {
	// 1. Volume CR has already exist
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1.DeepCopy()
	)

	v.Spec.CSIStatus = apiV1.Created
	ctx := context.WithValue(testCtx, util.VolumeInfoKey, &util.VolumeInfo{Namespace: testNS})
	err := svc.k8sClient.CreateCR(ctx, testVolume1Name, v)
	assert.Nil(t, err)

	createdVolume1, err := svc.CreateVolume(ctx, api.Volume{Id: v.Spec.Id})
	assert.Nil(t, err)
	assert.Equal(t, &v.Spec, createdVolume1)
}

// Volume CR was successfully created, HDD SC
func TestVolumeOperationsImpl_CreateVolume_HDDVolumeCreated(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)

		testAC     = testAC1.DeepCopy()
		testVolume = testVolume1.DeepCopy()
		testPVC    = testPVC1.DeepCopy()

		volumeID      = testVolume.Spec.Id
		requiredSC    = testVolume.Spec.StorageClass
		requiredNode  = testVolume.Spec.NodeId
		requiredBytes = testVolume.Spec.Size
	)

	parameters := map[string]string{
		util.ClaimNamespaceKey: testNS,
		util.ClaimNameKey:      testPVC.Name,
	}

	volumeInfo, err := util.NewVolumeInfo(parameters)
	assert.Nil(t, err)
	ctx := context.WithValue(testCtx, util.VolumeInfoKey, volumeInfo)

	err = svc.k8sClient.CreateCR(ctx, testAC.Name, testAC)
	assert.Nil(t, err)

	err = svc.k8sClient.Create(testCtx, testPVC)
	assert.Nil(t, err)

	testACR := getTestACR(testVolume.Spec.Size, apiV1.StorageClassHDD, parameters[util.ClaimNameKey],
		testVolume.Namespace, []*accrd.AvailableCapacity{testAC})
	err = svc.k8sClient.CreateCR(ctx, testACR.Name, testACR)
	assert.Nil(t, err)

	createdVolume, err := svc.CreateVolume(ctx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, err)
	assert.Equal(t, &testVolume.Spec, createdVolume)
}

func Test_handleVolumeInProgress(t *testing.T) {
	var (
		svc             = setupVOOperationsTest(t)
		ctx             = context.TODO()
		testVolume      = testVolume1.DeepCopy()
		podName         = "my-pod"
		reservationName = "default-" + podName
		logger          = testLogger.WithField("test", "handleVolumeInProgress")
	)

	// Creating
	volume, err := svc.handleVolumeInProgress(ctx, logger, testVolume, podName, reservationName)
	assert.Nil(t, err)
	assert.Equal(t, volume.CSIStatus, apiV1.Creating)

	// Created
	testVolume.Spec.CSIStatus = apiV1.Created
	volume, err = svc.handleVolumeInProgress(ctx, logger, testVolume, podName, reservationName)
	assert.Nil(t, err)
	assert.Equal(t, volume.CSIStatus, apiV1.Created)

	// Failed
	testVolume.Spec.CSIStatus = apiV1.Failed
	volume, err = svc.handleVolumeInProgress(ctx, logger, testVolume, podName, reservationName)
	assert.NotNil(t, err)

	// Published
	testVolume.Spec.CSIStatus = apiV1.Published
	volume, err = svc.handleVolumeInProgress(ctx, logger, testVolume, podName, reservationName)
	assert.NotNil(t, err)

	// Timeout
	testVolume.Spec.CSIStatus = apiV1.Creating
	// Update creation timestamp
	testVolume.ObjectMeta.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-base.DefaultTimeoutForVolumeOperations)})
	volume, err = svc.handleVolumeInProgress(ctx, logger, testVolume, podName, reservationName)
	assert.NotNil(t, err)
	assert.Nil(t, volume)
}

// Volume CR was successfully created, HDDLVG SC
func TestVolumeOperationsImpl_CreateVolume_HDDLVGVolumeCreated(t *testing.T) {
	var (
		svc           = setupVOOperationsTest(t)
		requiredSC    = apiV1.StorageClassHDDLVG
		volumeID      = "pvc-aaaa-bbbb"
		acName        = "aaaa-1111"
		requiredBytes = int64(util.GBYTE)
		testPVC       = testPVC1.DeepCopy()
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		acToReturn    = &accrd.AvailableCapacity{
			TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacityReservation", APIVersion: apiV1.APIV1Version},
			ObjectMeta: k8smetav1.ObjectMeta{Name: acName, CreationTimestamp: k8smetav1.NewTime(time.Now())},
			Spec: api.AvailableCapacity{
				StorageClass: requiredSC,
				Size:         requiredBytes,
			},
		}
		acrToReturn = &acrcrd.AvailableCapacityReservation{
			TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacityReservation", APIVersion: apiV1.APIV1Version},
			ObjectMeta: k8smetav1.ObjectMeta{Name: "test-ac", CreationTimestamp: k8smetav1.NewTime(time.Now())},
			Spec: api.AvailableCapacityReservation{
				Namespace: testNS,
				Status:    apiV1.ReservationConfirmed,
				ReservationRequests: []*api.ReservationRequest{
					{
						CapacityRequest: &api.CapacityRequest{
							StorageClass: requiredSC,
							Size:         requiredBytes,
							Name:         volumeID,
						},
						Reservations: []string{acName}},
				},
			},
		}
		expectedVolume = &api.Volume{
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
	)
	testPVC.ObjectMeta.Name = volumeID
	assert.Nil(t, svc.k8sClient.Create(ctxWithID, testPVC))
	assert.Nil(t, svc.k8sClient.CreateCR(ctxWithID, acToReturn.Name, acToReturn))
	assert.Nil(t, svc.k8sClient.CreateCR(ctxWithID, acrToReturn.Name, acrToReturn))
	tv := api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		Size:         requiredBytes,
		Location:     testLVG.Spec.Name,
	}

	ctx := context.WithValue(testCtx, util.VolumeInfoKey, &util.VolumeInfo{Name: volumeID, Namespace: testNS})
	createdVolume, err := svc.CreateVolume(ctx, tv)
	assert.Nil(t, err)
	assert.NotNil(t, createdVolume)
	assert.Equal(t, expectedVolume, createdVolume)
}

// Volume CR exists and has "failed" CSIStatus
func TestVolumeOperationsImpl_CreateVolume_FaileCauseExist(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1.DeepCopy()
	)

	v.Spec.CSIStatus = apiV1.Failed
	svc.cache.Set(v.Name, v.Namespace)
	ctx := context.WithValue(testCtx, util.VolumeInfoKey, &util.VolumeInfo{Namespace: v.Namespace})
	assert.Nil(t, svc.k8sClient.CreateCR(ctx, testVolume1Name, v))

	createdVolume, err := svc.CreateVolume(ctx, api.Volume{Id: v.Spec.Id})
	assert.NotNil(t, err)
	assert.Nil(t, createdVolume)
}

// Volume CR exists and timeout for creation exceeded
func TestVolumeOperationsImpl_CreateVolume_FailCauseTimeout(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1.DeepCopy()
	)
	v.ObjectMeta.CreationTimestamp = k8smetav1.Time{
		Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local),
	}
	ctx := context.WithValue(testCtx, util.VolumeInfoKey, &util.VolumeInfo{Namespace: v.Namespace})
	err := svc.k8sClient.CreateCR(ctx, v.Name, v)
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
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		testPVC       = testPVC1.DeepCopy()
		node1         = "node1"
		node2         = "node2"
		requiredSC    = apiV1.StorageClassHDDLVG
		requiredBytes = int64(util.GBYTE)
		acName        = "aaaa-1111"
		acToReturn    = &accrd.AvailableCapacity{
			TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacityReservation", APIVersion: apiV1.APIV1Version},
			ObjectMeta: k8smetav1.ObjectMeta{Name: acName, CreationTimestamp: k8smetav1.NewTime(time.Now())},
			Spec: api.AvailableCapacity{
				StorageClass: requiredSC,
				NodeId:       node1,
				Size:         requiredBytes,
			},
		}
		acrToReturn = &acrcrd.AvailableCapacityReservation{
			TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacityReservation", APIVersion: apiV1.APIV1Version},
			ObjectMeta: k8smetav1.ObjectMeta{Name: "test-ac", CreationTimestamp: k8smetav1.NewTime(time.Now())},
			Spec: api.AvailableCapacityReservation{
				Namespace: testNS,
				Status:    apiV1.ReservationConfirmed,
				ReservationRequests: []*api.ReservationRequest{
					{
						CapacityRequest: &api.CapacityRequest{
							StorageClass: requiredSC,
							Size:         requiredBytes,
							Name:         volumeID,
						},
						Reservations: []string{acName}},
				},
			},
		}
	)

	testPVC.ObjectMeta.Name = volumeID
	assert.Nil(t, svc.k8sClient.Create(ctxWithID, testPVC))
	assert.Nil(t, svc.k8sClient.CreateCR(ctxWithID, acToReturn.Name, acToReturn))
	assert.Nil(t, svc.k8sClient.CreateCR(ctxWithID, acrToReturn.Name, acrToReturn))

	ctxWithID = context.WithValue(ctxWithID, util.VolumeInfoKey, &util.VolumeInfo{Name: volumeID, Namespace: testNS})
	createdVolume, err := svc.CreateVolume(ctxWithID, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       node2,
		Size:         requiredBytes,
	})
	assert.NotNil(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Nil(t, createdVolume)
}

// Fail to recreate AC from HDD to LogicalVolumeGroup
func TestVolumeOperationsImpl_CreateVolume_FailRecreateAC(t *testing.T) {
	var (
		svc           = setupVOOperationsTest(t)
		volumeID      = "pvc-aaaa-bbbb"
		ctxWithID     = context.WithValue(testCtx, base.RequestUUID, volumeID)
		requiredNode  = ""
		requiredSC    = apiV1.StorageClassHDDLVG
		requiredBytes = int64(util.GBYTE)
	)

	ctx := context.WithValue(ctxWithID, util.VolumeInfoKey, &util.VolumeInfo{Name: volumeID, Namespace: testNS})
	createdVolume, err := svc.CreateVolume(ctx, api.Volume{
		Id:           volumeID,
		StorageClass: requiredSC,
		NodeId:       requiredNode,
		Size:         requiredBytes,
	})
	assert.Nil(t, createdVolume)
	assert.NotNil(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestVolumeOperationsImpl_DeleteVolume_DifferentStatuses(t *testing.T) {
	var (
		err      error
		volumeCR volumecrd.Volume
	)

	svc := setupVOOperationsTest(t)
	err = svc.DeleteVolume(testCtx, "unknown-namespace")
	assert.NotNil(t, err)

	svc = setupVOOperationsTest(t)

	svc.cache.Set("unknown-volume", testNS)
	err = svc.DeleteVolume(testCtx, "unknown-volume")
	assert.NotNil(t, err)

	svc = setupVOOperationsTest(t)
	volumeCR = *testVolume1.DeepCopy()
	volumeCR.Spec.CSIStatus = apiV1.Removed
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	svc.cache.Set(volumeCR.Name, volumeCR.Namespace)
	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.Nil(t, err)

	svc = setupVOOperationsTest(t)
	svc.cache.Set(volumeCR.Name, volumeCR.Namespace)
	volumeCR = *testVolume1.DeepCopy()
	volumeCR.Spec.CSIStatus = ""
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.NotNil(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	svc = setupVOOperationsTest(t)
	svc.cache.Set(volumeCR.Name, volumeCR.Namespace)
	volumeCR = *testVolume1.DeepCopy()
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Name, &volumeCR))

	err = svc.DeleteVolume(testCtx, volumeCR.Name)
	assert.NotNil(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestVolumeOperationsImpl_DeleteVolume_FailToRemoveSt(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1.DeepCopy()
		err error
	)

	svc.cache.Set(v.Name, v.Namespace)
	v.Spec.CSIStatus = apiV1.Failed
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, v)
	assert.Nil(t, err)

	err = svc.DeleteVolume(testCtx, testVolume1Name)
	assert.NotNil(t, err)
	assert.Equal(t, status.Error(codes.Internal, "volume has reached failed status"), err)
}

// volume has status Removed or Removing
func TestVolumeOperationsImpl_DeleteVolume(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1.DeepCopy()
		err error
	)

	for _, st := range []string{apiV1.Removing, apiV1.Removed} {
		v.ObjectMeta.ResourceVersion = ""
		v.Spec.CSIStatus = st
		err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, v)
		assert.Nil(t, err)
		svc.cache.Set(v.Name, v.Namespace)
		err = svc.DeleteVolume(testCtx, testVolume1Name)
		assert.Nil(t, err)
	}
}

func TestVolumeOperationsImpl_DeleteVolume_SetStatus(t *testing.T) {
	var (
		svc        = setupVOOperationsTest(t)
		v          = testVolume1.DeepCopy()
		updatedVol = volumecrd.Volume{}
		err        error
	)

	v.Spec.CSIStatus = apiV1.Created
	svc.cache.Set(v.Name, v.Namespace)
	err = svc.k8sClient.CreateCR(testCtx, v.Name, v)
	assert.Nil(t, err)

	err = svc.DeleteVolume(testCtx, v.Name)
	assert.Nil(t, err)

	err = svc.k8sClient.ReadCR(testCtx, v.Name, v.Namespace, &updatedVol)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Removing, updatedVol.Spec.CSIStatus)
}

func TestVolumeOperationsImpl_WaitStatus_Success(t *testing.T) {
	var (
		svc = setupVOOperationsTest(t)
		v   = testVolume1.DeepCopy()
	)
	v.Spec.CSIStatus = apiV1.Created
	svc.cache.Set(v.Name, v.Namespace)
	err := svc.k8sClient.CreateCR(testCtx, v.Name, v)
	assert.Nil(t, err)

	ctx, closeFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeFn()

	err = svc.WaitStatus(ctx, v.Name, apiV1.Failed, apiV1.Created)
	assert.Nil(t, err)
}

func TestVolumeOperationsImpl_WaitStatus_Fails(t *testing.T) {
	var (
		svc     = setupVOOperationsTest(t)
		testVol = testVolume1.DeepCopy()
	)
	// namespace wasn't found
	err := svc.WaitStatus(testCtx, "unknown_name", apiV1.Created)
	assert.NotNil(t, err)
	// volume CR wasn't found scenario
	svc.cache.Set("unknown_name", testNS)
	err = svc.WaitStatus(testCtx, "unknown_name", apiV1.Created)
	assert.NotNil(t, err)
	// ctx is done scenario
	err = svc.k8sClient.CreateCR(testCtx, testVol.Name, testVol)
	svc.cache.Set(testVol.Name, testVol.Namespace)
	assert.Nil(t, err)

	ctx, closeFn := context.WithTimeout(context.Background(), 10*time.Second)
	closeFn()
	ctx.Done()

	// volume CR wasn't found
	err = svc.WaitStatus(ctx, testVol.Name, apiV1.Created)
	assert.NotNil(t, err)
}

func TestVolumeOperationsImpl_UpdateCRsAfterVolumeDeletion(t *testing.T) {
	var (
		err        error
		svc        = setupVOOperationsTest(t)
		volumeHDD  = testVolume1.DeepCopy()
		volume1    = testVolumeLVG1.DeepCopy()
		volume2    = testVolumeLVG1.DeepCopy()
		lvg        = testLVG.DeepCopy()
		lvgUpdated = &lvgcrd.LogicalVolumeGroup{}
		ACUpdated  = &accrd.AvailableCapacity{}
	)

	// Test Case 1: volume with HDD SC, removed
	volume1.ObjectMeta.ResourceVersion = ""
	err = svc.k8sClient.CreateCR(testCtx, volumeHDD.Name, volumeHDD)
	assert.Nil(t, err)
	svc.cache.Set(volumeHDD.Name, volume1.Namespace)
	svc.UpdateCRsAfterVolumeDeletion(testCtx, volumeHDD.Name)

	err = svc.k8sClient.ReadCR(testCtx, volumeHDD.Name, volumeHDD.Namespace, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// Test Case 2: volumes with HDDLVG SC
	// create AC, LVG and Volumes with LVG
	volume2.Name = testVolumeLVG2Name
	volume2.Spec.Id = testVolumeLVG2Name

	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, testAC4.Name, &testAC4))
	lvg.Spec.VolumeRefs = []string{volume1.Name, volume2.Name}
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, lvg.Name, lvg))
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, testDriveCR4.Name, &testDriveCR4))
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volume1.Name, volume1))
	svc.cache.Set(volume1.Name, volume1.Namespace)
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volume2.Name, volume2))
	svc.cache.Set(volume2.Name, volume2.Namespace)

	// remove one volume from two
	svc.UpdateCRsAfterVolumeDeletion(testCtx, volume1.Name)

	// check that Volume was removed
	err = svc.k8sClient.ReadCR(testCtx, volume1.Name, volume1.Namespace, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// check that decreased LVG VolumeRefs
	err = svc.k8sClient.ReadCR(testCtx, testLVGName, "", lvgUpdated)
	assert.Nil(t, err)
	assert.Equal(t, len(lvgUpdated.Spec.VolumeRefs), 1)
	// check that AC size was increased
	err = svc.k8sClient.ReadCR(testCtx, testAC4Name, "", ACUpdated)
	assert.Nil(t, err)
	assert.Equal(t, ACUpdated.Spec.Size, testAC4.Spec.Size+volume1.Spec.Size)

	// remove last volume from two
	svc.UpdateCRsAfterVolumeDeletion(testCtx, volume2.Name)

	// check that Volume was removed
	err = svc.k8sClient.ReadCR(testCtx, volume2.Name, volume2.Namespace, &volumecrd.Volume{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// check that LVG was removed
	err = svc.k8sClient.ReadCR(testCtx, lvg.Name, "", &lvgcrd.LogicalVolumeGroup{})
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))

	// check that AC size was increased
	err = svc.k8sClient.ReadCR(testCtx, testAC4Name, "", ACUpdated)
	assert.Nil(t, err)
	assert.Equal(t, testAC4.Spec.Size+volume1.Spec.Size+volume2.Spec.Size, ACUpdated.Spec.Size)
	// check that AC convert from LVG to Drive
	assert.Equal(t, ACUpdated.Spec.Location, testDriveCR4.Name)
	assert.Equal(t, ACUpdated.Spec.StorageClass, util.ConvertDriveTypeToStorageClass(testDriveCR4.Spec.Type))
}

func TestVolumeOperationsImpl_ExpandVolume_DifferentStatuses(t *testing.T) {
	var (
		svc      *VolumeOperationsImpl
		err      error
		capacity = int64(util.GBYTE) * 10
	)

	volumeCR := testVolume1
	volumeCR.ObjectMeta.ResourceVersion = ""
	volumeCR.Spec.StorageClass = apiV1.StorageClassSystemLVG
	svc = setupVOOperationsTest(t)
	err = svc.k8sClient.CreateCR(testCtx, testVolume1Name, &volumeCR)
	volAC := &accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: uuid.New().String(), Namespace: testNS},
		Spec: api.AvailableCapacity{
			Size:         10000,
			StorageClass: apiV1.StorageClassSystemLVG,
		},
	}
	volAC.Spec.Location = testDrive1UUID
	err = svc.k8sClient.CreateCR(testCtx, "", volAC)
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
		capacity = int64(util.TBYTE)
		volumeCR = testVolume1.DeepCopy()
	)

	svc = setupVOOperationsTest(t)
	volumeCR.ObjectMeta.CreationTimestamp = k8smetav1.Time{}
	volumeCR.ObjectMeta.ResourceVersion = ""
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, volumeCR.Spec.Id, volumeCR))

	for _, v := range [4]string{apiV1.Failed, apiV1.Removed, apiV1.Removing, apiV1.Creating} {
		volumeCR.Spec.CSIStatus = v
		assert.Nil(t, svc.k8sClient.UpdateCR(testCtx, volumeCR))
		err := svc.ExpandVolume(testCtx, volumeCR, capacity)
		assert.NotNil(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	}

	// Storage class is not lvg
	volumeCR.Spec.CSIStatus = apiV1.Created
	volumeCR.ObjectMeta.ResourceVersion = ""
	assert.NotNil(t, svc.k8sClient.UpdateCR(testCtx, volumeCR))
	if err := svc.ExpandVolume(testCtx, volumeCR, capacity); err != nil {
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	}

	// Failed to get AC
	volumeCR.ObjectMeta.ResourceVersion = ""
	volumeCR.Spec.StorageClass = apiV1.StorageClassSystemLVG
	assert.NotNil(t, svc.k8sClient.UpdateCR(testCtx, volumeCR))
	err := svc.ExpandVolume(testCtx, volumeCR, capacity)
	assert.NotNil(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))

	// Required capacity is more than capacity of AC
	volAC := &accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: uuid.New().String(), Namespace: testNS},
		Spec: api.AvailableCapacity{
			Size:         10000,
			StorageClass: apiV1.StorageClassSystemLVG,
		},
	}
	volAC.Spec.Location = testDrive1UUID
	assert.Nil(t, svc.k8sClient.CreateCR(testCtx, "", volAC))
	err = svc.ExpandVolume(testCtx, volumeCR, capacity)
	assert.NotNil(t, err)
	assert.Equal(t, codes.OutOfRange, status.Code(err))
}

func TestVolumeOperationsImpl_UpdateCRsAfterVolumeExpansion(t *testing.T) {
	var (
		svc      = setupVOOperationsTest(t)
		volumeCR = testVolume1.DeepCopy()
		err      error
	)

	svc.cache.Set(volumeCR.Spec.Id, volumeCR.Namespace)
	volumeCR.Spec.CSIStatus = apiV1.Failed
	err = svc.k8sClient.CreateCR(testCtx, volumeCR.Spec.Id, volumeCR)
	volAC := &accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: uuid.New().String()},
		Spec: api.AvailableCapacity{
			Size:         107373143824,
			StorageClass: apiV1.StorageClassSystemLVG,
		},
	}
	volAC.Spec.Location = testDrive1UUID
	err = svc.k8sClient.CreateCR(testCtx, "", volAC)
	assert.Nil(t, err)

	// volume doesn't have annotation
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)

	capacity, err := svc.crHelper.GetACByLocation(volumeCR.Spec.Location)
	assert.Nil(t, err)
	assert.Equal(t, volAC.Spec.Size, capacity.Spec.Size)

	// volume has annotation and status failed
	volumeCR.Annotations = map[string]string{
		apiV1.VolumePreviousCapacity: strconv.FormatInt(int64(util.MBYTE), 10),
	}
	err = svc.k8sClient.UpdateCR(testCtx, volumeCR)
	pAC, err := svc.crHelper.GetACByLocation(volumeCR.Spec.Location)
	assert.Nil(t, err)
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)

	err = svc.k8sClient.ReadCR(testCtx, volumeCR.Name, volumeCR.Namespace, volumeCR)
	assert.Nil(t, err)
	capacity, err = svc.crHelper.GetACByLocation(volumeCR.Spec.Location)
	assert.Nil(t, err)
	assert.Equal(t, pAC.Spec.Size+int64(util.GBYTE)*100-int64(util.MBYTE), capacity.Spec.Size)

	// volume has resized status and doesn't have annotation
	volumeCR.Spec.CSIStatus = apiV1.Resized
	err = svc.k8sClient.UpdateCR(testCtx, volumeCR)
	assert.Nil(t, err)
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)
	err = svc.k8sClient.ReadCR(testCtx, volumeCR.Name, volumeCR.Namespace, volumeCR)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Resized, volumeCR.Spec.CSIStatus)

	// volume has resized status
	volumeCR.Annotations = map[string]string{
		apiV1.VolumePreviousStatus: apiV1.Created,
	}
	err = svc.k8sClient.UpdateCR(testCtx, volumeCR)
	assert.Nil(t, err)
	svc.UpdateCRsAfterVolumeExpansion(testCtx, volumeCR.Spec.Id, int64(util.GBYTE)*100)
	err = svc.k8sClient.ReadCR(testCtx, volumeCR.Name, volumeCR.Namespace, volumeCR)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Created, volumeCR.Spec.CSIStatus)
}

func getTestACR(size int64, sc, name, podNamespace string,
	acList []*accrd.AvailableCapacity) *acrcrd.AvailableCapacityReservation {
	acNames := make([]string, len(acList))
	for i, ac := range acList {
		acNames[i] = ac.Name
	}
	return &acrcrd.AvailableCapacityReservation{
		TypeMeta: k8smetav1.TypeMeta{Kind: "AvailableCapacityReservation", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: uuid.New().String(),
			CreationTimestamp: k8smetav1.NewTime(time.Now())},
		Spec: api.AvailableCapacityReservation{
			Namespace: podNamespace,
			Status:    apiV1.ReservationConfirmed,
			ReservationRequests: []*api.ReservationRequest{
				{CapacityRequest: &api.CapacityRequest{
					StorageClass: sc,
					Size:         size,
					Name:         name,
				},
					Reservations: acNames},
			},
		},
	}
}
