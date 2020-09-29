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

package node

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/mocks"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
	mockProv "github.com/dell/csi-baremetal/pkg/mocks/provisioners"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
)

// TODO: refactor these UTs - https://github.com/dell/csi-baremetal/issues/90

var (
	testErr            = errors.New("error")
	lsblkAllDevicesCmd = fmt.Sprintf(lsblk.CmdTmpl, "")

	drive1UUID   = uuid.New().String()
	drive2UUID   = uuid.New().String()
	testPartUUID = uuid.New().String()

	drive1 = api.Drive{
		UUID:         drive1UUID,
		SerialNumber: "hdd1-serial",
		Size:         1024 * 1024 * 1024 * 500,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthGood,
		Path:         "/dev/sda",
	} // /dev/sda in LsblkTwoDevices

	drive2 = api.Drive{
		UUID:         drive2UUID,
		SerialNumber: "hdd2-serial",
		Size:         1024 * 1024 * 1024 * 200,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthGood,
		Path:         "/dev/sdb",
		IsSystem:     true,
	} // /dev/sdb in LsblkTwoDevices

	// block device that corresponds to the drive1
	bdev1 = lsblk.BlockDevice{
		Name:     drive1.Path,
		Type:     drive1.Type,
		Size:     strconv.FormatInt(drive1.Size, 10),
		Serial:   drive1.SerialNumber,
		Children: nil,
	}

	// block device that corresponds to the drive2
	bdev2 = lsblk.BlockDevice{
		Name:     drive2.Path,
		Type:     drive2.Type,
		Size:     strconv.FormatInt(drive1.Size, 10),
		Serial:   drive2.SerialNumber,
		Children: nil,
	}

	// todo don't hardcode device name
	lsblkSingleDeviceCmd = fmt.Sprintf(lsblk.CmdTmpl, "/dev/sda")

	volCR = vcrd.Volume{
		TypeMeta: v1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{
			Name:              testID,
			Namespace:         testNs,
			CreationTimestamp: v1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:           testID,
			Size:         1024 * 1024 * 1024 * 150,
			StorageClass: apiV1.StorageClassHDD,
			Location:     "",
			CSIStatus:    apiV1.Creating,
			NodeId:       nodeID,
			Mode:         apiV1.ModeFS,
			Type:         string(fs.XFS),
		},
	}

	testLVGCR = lvgcrd.LVG{
		TypeMeta: v1.TypeMeta{
			Kind:       "LVG",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      testLVGName,
			Namespace: testNs,
		},
		Spec: api.LogicalVolumeGroup{
			Name:       testLVGName,
			Node:       nodeID,
			Locations:  []string{drive1.UUID},
			Size:       int64(1024 * 500 * util.GBYTE),
			Status:     apiV1.Created,
			VolumeRefs: []string{},
		},
	}

	testVolumeLVGCR = vcrd.Volume{
		TypeMeta: v1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{
			Name:              volLVGName,
			Namespace:         testNs,
			CreationTimestamp: v1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:           volLVGName,
			Size:         1024 * 1024 * 1024 * 150,
			StorageClass: apiV1.StorageClassHDDLVG,
			Location:     testLVGCR.Name,
			CSIStatus:    apiV1.Creating,
			NodeId:       nodeID,
			Mode:         apiV1.ModeFS,
			Type:         string(fs.XFS),
		},
	}

	acCR = accrd.AvailableCapacity{
		TypeMeta:   v1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: driveUUID, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:         drive1.Size,
			StorageClass: apiV1.StorageClassHDD,
			Location:     "drive-uuid",
			NodeId:       drive1.NodeId},
	}
)

func getTestDrive(id, sn string) *api.Drive {
	return &api.Drive{
		UUID:         id,
		SerialNumber: sn,
		Size:         1024 * 1024 * 1024 * 500,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthGood,
	}
}

func TestVolumeManager_NewVolumeManager(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	assert.NotNil(t, vm)
	assert.Nil(t, vm.driveMgrClient)
	assert.NotNil(t, vm.fsOps)
	assert.NotNil(t, vm.lvmOps)
	assert.NotNil(t, vm.listBlk)
	assert.NotNil(t, vm.partOps)
	assert.True(t, len(vm.provisioners) > 0)
	assert.NotNil(t, vm.acProvider)
	assert.NotNil(t, vm.crHelper)
}

func TestReconcile_MultipleRequest(t *testing.T) {
	//Try to create volume multiple time in go routine, expect that CSI status Created and volume was created without error
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	volCR.Spec.CSIStatus = apiV1.Creating
	err = vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR)
	assert.Nil(t, err)

	pMock := mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		res, err := vm.Reconcile(req)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		res, err := vm.Reconcile(req)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)
		wg.Done()
	}()
	wg.Wait()
	volume := &vcrd.Volume{}
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Created, volume.Spec.CSIStatus)
}

func TestReconcile_SuccessNotFound(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "not-found-that-name"}}
	res, err := vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func TestVolumeManager_prepareVolume(t *testing.T) {
	var (
		vm     *VolumeManager
		req    = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
		volume = &vcrd.Volume{}
		pMock  *mockProv.MockProvisioner
		res    ctrl.Result
		err    error
	)

	// happy pass
	vm = prepareSuccessVolumeManager(t)
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR))
	pMock = mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	testVol := volCR
	res, err = vm.prepareVolume(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Created)

	// failed to update
	vm = prepareSuccessVolumeManager(t)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err = vm.prepareVolume(testCtx, &testVol)
	assert.NotNil(t, err)
	assert.True(t, res.Requeue)

	// PrepareVolume failed
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR))
	pMock = &mockProv.MockProvisioner{}
	pMock.On("PrepareVolume", volCR.Spec).Return(testErr)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err = vm.prepareVolume(testCtx, &volCR)
	assert.NotNil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Failed)
}

func TestVolumeManager_handleRemovingStatus(t *testing.T) {
	var (
		vm     *VolumeManager
		req    = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
		volume = &vcrd.Volume{}
		res    ctrl.Result
		err    error
	)

	// happy path
	vm = prepareSuccessVolumeManager(t)
	testVol := volCR
	testVol.Spec.CSIStatus = apiV1.Removing
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, volCR.Name, &testVol))
	pMock := mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err = vm.handleRemovingStatus(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Removed)

	// failed to update
	vm = prepareSuccessVolumeManager(t)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err = vm.handleRemovingStatus(testCtx, &volCR)
	assert.NotNil(t, err)
	assert.True(t, res.Requeue)

	// ReleaseVolume failed
	testVol = volCR
	testVol.Spec.CSIStatus = apiV1.Removing
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR))
	pMock = &mockProv.MockProvisioner{}
	pMock.On("ReleaseVolume", volCR.Spec).Return(testErr)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err = vm.handleRemovingStatus(testCtx, &volCR)
	assert.NotNil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Failed)

}

func TestReconcile_SuccessDeleteVolume(t *testing.T) {
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	volCR.Spec.CSIStatus = apiV1.Removed
	err = vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR)
	assert.Nil(t, err)

	pMock := mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	//successfully add finalizer
	res, err := vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	//successfully remove finalizer
	volCR.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	err = vm.k8sClient.UpdateCR(testCtx, &volCR)
	assert.Nil(t, err)

	res, err = vm.Reconcile(req)
	assert.NotNil(t, k8sError.IsNotFound(err))
	assert.Equal(t, res, ctrl.Result{})
}

func TestVolumeManager_handleCreatingVolumeInLVG(t *testing.T) {
	var (
		vm                 *VolumeManager
		pMock              *mockProv.MockProvisioner
		vol                *vcrd.Volume
		lvg                *lvgcrd.LVG
		testVol            vcrd.Volume
		testLVG            lvgcrd.LVG
		expectedResRequeue = ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}
		res                ctrl.Result
		err                error
	)

	// unable to read LVG (not found) and unable to update corresponding volume CR
	vm = prepareSuccessVolumeManager(t)

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))
	assert.Equal(t, expectedResRequeue, res)

	// LVG is not found, volume CR was updated successfully (CSIStatus=failed)
	vm = prepareSuccessVolumeManager(t)
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, &testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, vol))
	assert.Equal(t, apiV1.Failed, vol.Spec.CSIStatus)

	// LVG in creating state
	vm = prepareSuccessVolumeManager(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Creating
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, expectedResRequeue, res)

	// LVG in failed state and volume is updated successfully
	vm = prepareSuccessVolumeManager(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Failed
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, &testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, vol))
	assert.Equal(t, apiV1.Failed, vol.Spec.CSIStatus)

	// LVG in failed state and volume is failed to update
	vm = prepareSuccessVolumeManager(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Failed
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.NotNil(t, err)
	assert.Equal(t, expectedResRequeue, res)
	assert.True(t, k8sError.IsNotFound(err))

	// LVG in created state and volume.ID is not in VolumeRefs
	vm = prepareSuccessVolumeManager(t)
	pMock = &mockProv.MockProvisioner{}
	pMock.On("PrepareVolume", mock.Anything).Return(nil)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.LVMBasedVolumeType: pMock})
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Created
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, &testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	lvg = &lvgcrd.LVG{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testLVG.Name, lvg))
	assert.True(t, util.ContainsString(lvg.Spec.VolumeRefs, testVol.Spec.Id))

	// LVG in created state and volume.ID is in VolumeRefs
	vm = prepareSuccessVolumeManager(t)
	pMock = &mockProv.MockProvisioner{}
	pMock.On("PrepareVolume", mock.Anything).Return(nil)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.LVMBasedVolumeType: pMock})
	testVol = testVolumeLVGCR
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Created
	testLVG.Spec.VolumeRefs = []string{testVol.Spec.Id}
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, &testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	lvg = &lvgcrd.LVG{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testLVG.Name, lvg))
	assert.True(t, util.ContainsString(lvg.Spec.VolumeRefs, testVol.Spec.Id))
	assert.Equal(t, 1, len(lvg.Spec.VolumeRefs))

	// LVG state wasn't recognized
	vm = prepareSuccessVolumeManager(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = ""
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, expectedResRequeue, res)
}

func TestReconcile_ReconcileDefaultStatus(t *testing.T) {
	var (
		vm  *VolumeManager
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
		res ctrl.Result
		err error
	)

	vm = prepareSuccessVolumeManager(t)
	volCR.Spec.CSIStatus = apiV1.Published
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR))

	res, err = vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func TestNewVolumeManager_SetProvisioners(t *testing.T) {
	vm := NewVolumeManager(nil, mocks.EmptyExecutorSuccess{}, logrus.New(), nil, new(mocks.NoOpRecorder), nodeID)
	newProv := &mockProv.MockProvisioner{}
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: newProv})
	assert.Equal(t, newProv, vm.provisioners[p.DriveBasedVolumeType])
}

func TestVolumeManager_DrivesNotInUse_Success(t *testing.T) {
	vm := prepareSuccessVolumeManager(t)

	addDriveCRs(vm.k8sClient,
		vm.k8sClient.ConstructDriveCR(drive1UUID, drive1),
		vm.k8sClient.ConstructDriveCR(drive2UUID, drive2),
	)

	drivesNotInUse, err := vm.drivesAreNotUsed()
	assert.Nil(t, err)
	// there are no Volume CRs, method should return all drives
	assert.NotNil(t, drivesNotInUse)
	assert.Equal(t, 2, len(drivesNotInUse))

	// add Volume CR that points on drive1
	volumeCR := vm.k8sClient.ConstructVolumeCR("test_name", api.Volume{
		NodeId:       nodeID,
		LocationType: apiV1.LocationTypeDrive,
		Location:     drive1.UUID,
	})
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, volumeCR.Name, volumeCR))

	// expect drive2 isn't in use
	drivesNotInUse, err = vm.drivesAreNotUsed()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(drivesNotInUse))
	assert.Equal(t, drive2.UUID, drivesNotInUse[0].Spec.UUID)
}

func TestVolumeManager_DrivesNotInUse_Fail(t *testing.T) {
	mockK8sClient := &mocks.K8Client{}
	vm := NewVolumeManager(nil, nil, testLogger,
		k8s.NewKubeClient(mockK8sClient, testLogger, testNs),
		new(mocks.NoOpRecorder), nodeID)

	var (
		res []*drivecrd.Drive
		err error
	)

	// unable to list Volume CRs
	mockK8sClient.On("List", mock.Anything, &vcrd.VolumeList{}, mock.Anything).Return(testErr).Once()

	res, err = vm.drivesAreNotUsed()
	assert.Nil(t, res)
	assert.Equal(t, testErr, err)

	// unable to list Drive CRs
	mockK8sClient.On("List", mock.Anything, &vcrd.VolumeList{}, mock.Anything).Return(nil).Once()
	mockK8sClient.On("List", mock.Anything, &drivecrd.DriveList{}, mock.Anything).Return(testErr).Once()

	res, err = vm.drivesAreNotUsed()
	assert.Nil(t, res)
	assert.Equal(t, testErr, err)
}

func TestVolumeManager_DiscoverFail(t *testing.T) {
	var (
		vm  *VolumeManager
		err error
	)

	// expect: hwMgrClient request fail with error
	vm = prepareSuccessVolumeManager(t)
	vm.driveMgrClient = mocks.MockDriveMgrClientFail{}

	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "drivemgr error", err.Error())

	// used in next scenarios
	mockK8sClient := &mocks.K8Client{}

	// expect: updateDrivesCRs failed
	vm = NewVolumeManager(mocks.MockDriveMgrClient{}, nil, testLogger, k8s.NewKubeClient(mockK8sClient, testLogger, testNs), nil, nodeID)
	mockK8sClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(testErr).Once()

	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "updateDrivesCRs return error")

	// expect: driveAreNotUsed failed
	vm = NewVolumeManager(mocks.MockDriveMgrClient{}, nil, testLogger, k8s.NewKubeClient(mockK8sClient, testLogger, testNs), nil, nodeID)
	mockK8sClient.On("List", mock.Anything, &drivecrd.DriveList{}, mock.Anything).Return(nil).Once()
	mockK8sClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(testErr).Once()

	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "drivesAreNotUsed return error")

	// expect: discoverVolumeCRs failed
	vm = NewVolumeManager(mocks.MockDriveMgrClient{}, nil, testLogger, k8s.NewKubeClient(mockK8sClient, testLogger, testNs), nil, nodeID)
	listBlk := &mocklu.MockWrapLsblk{}
	listBlk.On("GetBlockDevices", "").Return(nil, testErr).Once()
	vm.listBlk = listBlk
	vm.discoverLvgSSD = false
	mockK8sClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(4)

	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "discoverVolumeCRs return error")

	//expect: discoverAvailableCapacities failed
	vm = NewVolumeManager(mocks.MockDriveMgrClient{}, nil, testLogger, k8s.NewKubeClient(mockK8sClient, testLogger, testNs), nil, nodeID)
	listBlk.On("GetBlockDevices", "").Return([]lsblk.BlockDevice{}, nil)
	vm.listBlk = listBlk
	mockK8sClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(4)
	mockK8sClient.On("List", mock.Anything, &accrd.AvailableCapacityList{}, mock.Anything).Return(testErr).Once()
	vm.discoverLvgSSD = false
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "discoverAvailableCapacity return error")

}

func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	var (
		vm             *VolumeManager
		listBlk        = &mocklu.MockWrapLsblk{}
		driveMgrClient = mocks.NewMockDriveMgrClient(getDriveMgrRespBasedOnDrives(drive1, drive2))
		err            error
	)

	vm = prepareSuccessVolumeManager(t)
	vm.driveMgrClient = driveMgrClient
	vm.listBlk = listBlk

	listBlk.On("GetBlockDevices", "").Return([]lsblk.BlockDevice{bdev1, bdev2}, nil).Once()
	listBlk.On("GetBlockDevices", drive1.Path).Return([]lsblk.BlockDevice{bdev1}, nil).Once()
	listBlk.On("GetBlockDevices", drive2.Path).Return([]lsblk.BlockDevice{bdev2}, nil).Once()
	// expect that Volume CRs won't be created because of all drives don't have children
	err = vm.Discover()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(getVolumeCRsListItems(t, vm.k8sClient)))
	assert.Equal(t, 2, len(getACCRsListItems(t, vm.k8sClient)))
}

func TestVolumeManager_Discover_noncleanDisk(t *testing.T) {
	/*
		test scenario consists of 2 Discover iteration on first:
		 - driveMgr returned 2 drives and there are no partition on them from lsblk response, expect that
		   2 Drive CRs, 2 AC CRs and 0 Volume CR will be created
		on second iteration:
		 - driveMgr returned 2 drives and on one of them lsblk detect partition, expect that amount of Drive CRs won't be changed
		   1 Volume CR will be created and on AC CR will be removed (1 AC remains)
	*/

	// fist iteration
	vm := prepareSuccessVolumeManager(t)
	vm.driveMgrClient = mocks.NewMockDriveMgrClient([]*api.Drive{&drive1, &drive2})
	vItems := getVolumeCRsListItems(t, vm.k8sClient)
	dItems := getDriveCRsListItems(t, vm.k8sClient)
	acItems := getACCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 0, len(dItems))
	assert.Equal(t, 0, len(vItems))
	assert.Equal(t, 0, len(acItems))

	listBlk := &mocklu.MockWrapLsblk{}
	vm.listBlk = listBlk
	listBlk.On("GetBlockDevices", "").Return([]lsblk.BlockDevice{bdev1, bdev2}, nil).Once()
	listBlk.On("GetBlockDevices", drive1.Path).Return([]lsblk.BlockDevice{bdev1}, nil).Once()
	listBlk.On("GetBlockDevices", drive2.Path).Return([]lsblk.BlockDevice{bdev2}, nil).Once()

	err := vm.Discover()
	assert.Nil(t, err)

	vItems = getVolumeCRsListItems(t, vm.k8sClient)
	dItems = getDriveCRsListItems(t, vm.k8sClient)
	acItems = getACCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 2, len(dItems))
	assert.Equal(t, 2, len(acItems))
	assert.Equal(t, 0, len(vItems))

	// second iteration
	bdev2WithChildren := bdev2
	bdev2WithChildren.Children = []lsblk.BlockDevice{{Name: "/dev/sda1", PartUUID: testPartUUID}}
	listBlk.On("GetBlockDevices", "").Return([]lsblk.BlockDevice{bdev1, bdev2WithChildren}, nil).Once()

	err = vm.Discover()
	assert.Nil(t, err)

	vItems = getVolumeCRsListItems(t, vm.k8sClient)
	dItems = getDriveCRsListItems(t, vm.k8sClient)
	acItems = getACCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 2, len(dItems))
	assert.Equal(t, 1, len(acItems))
	assert.Equal(t, 1, len(vItems))

	var (
		sdaDriveUUID string
		sdbDriveUUID string
	)
	for _, d := range dItems {
		if d.Spec.SerialNumber == drive1.SerialNumber {
			sdaDriveUUID = d.Spec.UUID
		} else if d.Spec.SerialNumber == drive2.SerialNumber {
			sdbDriveUUID = d.Spec.UUID
		}
	}
	assert.Equal(t, vItems[0].Spec.Location, sdbDriveUUID)
	assert.Equal(t, acItems[0].Spec.Location, sdaDriveUUID)
}

func TestVolumeManager_DiscoverAvailableCapacitySuccess(t *testing.T) {
	vm := prepareSuccessVolumeManager(t)
	vm.driveMgrClient = mocks.NewMockDriveMgrClient(getDriveMgrRespBasedOnDrives(drive1, drive2))
	listBlk := &mocklu.MockWrapLsblk{}
	vm.listBlk = listBlk
	listBlk.On("GetBlockDevices", "").Return([]lsblk.BlockDevice{bdev1, bdev2}, nil).Once()
	listBlk.On("GetBlockDevices", drive1.Path).Return([]lsblk.BlockDevice{bdev1}, nil).Once()
	listBlk.On("GetBlockDevices", drive2.Path).Return([]lsblk.BlockDevice{bdev2}, nil).Once()

	err := vm.Discover()
	assert.Nil(t, err)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(getACCRsListItems(t, vm.k8sClient)))
}

func TestVolumeManager_DiscoverAvailableCapacityDriveUnhealthy(t *testing.T) {
	var (
		vm      *VolumeManager
		listBlk = &mocklu.MockWrapLsblk{}
		err     error
	)

	// expected 1 available capacity because 1 drive is unhealthy
	vm = prepareSuccessVolumeManager(t)
	d2 := drive2
	d2.Health = apiV1.HealthBad
	vm.driveMgrClient = mocks.NewMockDriveMgrClient(getDriveMgrRespBasedOnDrives(drive1, d2))
	vm.listBlk = listBlk
	listBlk.On("GetBlockDevices", "").Return([]lsblk.BlockDevice{bdev1, bdev2}, nil).Once()
	listBlk.On("GetBlockDevices", drive1.Path).Return([]lsblk.BlockDevice{bdev1}, nil).Once()
	listBlk.On("GetBlockDevices", drive2.Path).Return([]lsblk.BlockDevice{bdev2}, nil).Once()

	err = vm.Discover()
	assert.Nil(t, err)

	acList := &accrd.AvailableCapacityList{}
	err = vm.k8sClient.ReadList(testCtx, acList)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items))
}

func TestVolumeManager_updatesDrivesCRs_Success(t *testing.T) {
	vm := prepareSuccessVolumeManager(t)
	driveMgrRespDrives := getDriveMgrRespBasedOnDrives(drive1, drive2)
	vm.driveMgrClient = mocks.NewMockDriveMgrClient(driveMgrRespDrives)

	updates, err := vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
	assert.Nil(t, err)
	driveCRs, err := vm.crHelper.GetDriveCRs(vm.nodeID)
	assert.Nil(t, err)
	assert.Equal(t, len(driveCRs), 2)
	assert.Len(t, updates.Created, 2)

	driveMgrRespDrives[0].Health = apiV1.HealthBad
	updates, err = vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthBad)
	assert.Len(t, updates.Updated, 1)
	assert.Len(t, updates.NotChanged, 1)

	drives := driveMgrRespDrives[1:]
	updates, err = vm.updateDrivesCRs(testCtx, drives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthUnknown)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Status, apiV1.DriveStatusOffline)
	assert.Len(t, updates.Updated, 1)
	assert.Len(t, updates.NotChanged, 1)

	vm = prepareSuccessVolumeManager(t)
	driveCRs, err = vm.crHelper.GetDriveCRs(vm.nodeID)
	assert.Nil(t, err)
	assert.Empty(t, driveCRs)
	updates, err = vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
	assert.Nil(t, err)
	driveCRs, err = vm.crHelper.GetDriveCRs(vm.nodeID)
	assert.Nil(t, err)
	assert.Equal(t, len(driveCRs), 2)
	assert.Len(t, updates.Created, 2)
	driveMgrRespDrives = append(driveMgrRespDrives, &api.Drive{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd3",
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024 * 1024 * 150,
		NodeId:       nodeID,
	})
	updates, err = vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
	assert.Nil(t, err)
	driveCRs, err = vm.crHelper.GetDriveCRs(vm.nodeID)
	assert.Nil(t, err)
	assert.Equal(t, len(driveCRs), 3)
	assert.Len(t, updates.Created, 1)
	assert.Len(t, updates.NotChanged, 2)
}

func TestVolumeManager_updatesDrivesCRs_Fail(t *testing.T) {
	mockK8sClient := &mocks.K8Client{}
	vm := NewVolumeManager(nil, nil, testLogger,
		k8s.NewKubeClient(mockK8sClient, testLogger, testNs),
		new(mocks.NoOpRecorder), nodeID)

	var (
		res *driveUpdates
		err error
	)

	// GetDriveCRs failed
	mockK8sClient.On("List", mock.Anything, &drivecrd.DriveList{}, mock.Anything).Return(testErr).Once()

	res, err = vm.updateDrivesCRs(testCtx, nil)
	assert.Nil(t, res)
	assert.NotNil(t, err)
	assert.Equal(t, testErr, err)

	// CreateCR failed
	mockK8sClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()
	mockK8sClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(testErr).Twice() // CreateCR will failed

	d1 := drive1
	res, err = vm.updateDrivesCRs(testCtx, []*api.Drive{&d1})
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 1, len(res.Created))
}

func TestVolumeManager_handleDriveStatusChange(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives(nil, t)

	ac := acCR
	err := vm.k8sClient.CreateCR(testCtx, ac.Name, &ac)
	assert.Nil(t, err)

	drive := drive1
	drive.UUID = driveUUID
	drive.Health = apiV1.HealthBad

	// Check AC deletion
	vm.handleDriveStatusChange(testCtx, &drive)
	acList := &accrd.AvailableCapacityList{}
	err = vm.k8sClient.ReadList(testCtx, acList)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))

	vol := volCR
	vol.Spec.Location = driveUUID
	err = vm.k8sClient.CreateCR(testCtx, testID, &vol)
	assert.Nil(t, err)

	// Check volume's health change
	vm.handleDriveStatusChange(testCtx, &drive)
	rVolume := &vcrd.Volume{}
	err = vm.k8sClient.ReadCR(testCtx, testID, rVolume)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.HealthBad, rVolume.Spec.Health)
}

func Test_discoverLVGOnSystemDrive_LVGAlreadyExists(t *testing.T) {
	var (
		m     = prepareSuccessVolumeManager(t)
		lvgCR = m.k8sClient.ConstructLVGCR("some-name", api.LogicalVolumeGroup{
			Name:      "some-name",
			Node:      m.nodeID,
			Locations: []string{base.SystemDriveAsLocation},
		})
		lvgList = lvgcrd.LVGList{}
		err     error
		lvmOps  = &mocklu.MockWrapLVM{}
	)
	lvmOps.On("GetVgFreeSpace", "some-name").Return(int64(0), nil)
	m.lvmOps = lvmOps
	err = m.k8sClient.CreateCR(testCtx, lvgCR.Name, lvgCR)
	assert.Nil(t, err)

	err = m.discoverLVGOnSystemDrive()
	assert.Nil(t, err)

	err = m.k8sClient.ReadList(testCtx, &lvgList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvgList.Items))
	assert.Equal(t, lvgCR, &lvgList.Items[0])

	lvmOps.On("GetVgFreeSpace", "some-name").Return(int64(1024), nil)
	m.lvmOps = lvmOps
	err = m.k8sClient.CreateCR(testCtx, lvgCR.Name, lvgCR)
	assert.Nil(t, err)

	err = m.discoverLVGOnSystemDrive()
	assert.Nil(t, err)

	err = m.k8sClient.ReadList(testCtx, &lvgList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvgList.Items))
	assert.Equal(t, lvgCR, &lvgList.Items[0])
}

func Test_discoverLVGOnSystemDrive_LVGCreatedACNo(t *testing.T) {
	var (
		m       = prepareSuccessVolumeManager(t)
		lvgList = lvgcrd.LVGList{}
		acList  = accrd.AvailableCapacityList{}
		listBlk = &mocklu.MockWrapLsblk{}
		fsOps   = &mockProv.MockFsOpts{}
		lvmOps  = &mocklu.MockWrapLVM{}
		err     error
	)

	m.listBlk = listBlk
	m.fsOps = fsOps
	m.lvmOps = lvmOps

	rootMountPoint := "/dev/sda"
	vgName := "root-vg"
	fsOps.On("FindMountPoint", base.KubeletRootPath).Return(rootMountPoint, nil)
	listBlk.On("GetBlockDevices", rootMountPoint).Return([]lsblk.BlockDevice{{Rota: base.NonRotationalNum}}, nil)
	lvmOps.On("FindVgNameByLvName", rootMountPoint).Return(vgName, nil)
	lvmOps.On("GetVgFreeSpace", vgName).Return(int64(1024), nil)
	lvmOps.On("IsLVGExists", rootMountPoint).Return(true, nil)
	lvmOps.On("GetLVsInVG", vgName).Return([]string{"lv_swap", "lv_boot"}, nil).Once()

	// expect success, LVG CR and AC CR was created
	err = m.discoverLVGOnSystemDrive()
	assert.Nil(t, err)

	err = m.k8sClient.ReadList(testCtx, &lvgList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvgList.Items))
	lvg := lvgList.Items[0]
	assert.Equal(t, 1, len(lvg.Spec.Locations))
	assert.Equal(t, base.SystemDriveAsLocation, lvg.Spec.Locations[0])
	assert.Equal(t, apiV1.Created, lvg.Spec.Status)
	assert.Equal(t, 2, len(lvg.Spec.VolumeRefs))

	err = m.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))

	// unable to read LVs in system vg
	m = prepareSuccessVolumeManager(t)
	// mocks were setup for previous scenario
	m.listBlk = listBlk
	m.fsOps = fsOps
	m.lvmOps = lvmOps

	lvmOps.On("GetLVsInVG", vgName).Return(nil, testErr)

	err = m.discoverLVGOnSystemDrive()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to determine LVs in system VG")

	assert.Nil(t, m.k8sClient.ReadList(testCtx, &lvgList))
	assert.Equal(t, 0, len(lvgList.Items))
}

func TestVolumeManager_createEventsForDriveUpdates(t *testing.T) {
	k, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)

	drive1CR := k.ConstructDriveCR(drive1UUID, *getTestDrive(drive1UUID, "SN1"))
	drive2CR := k.ConstructDriveCR(drive2UUID, *getTestDrive(drive1UUID, "SN2"))

	var (
		rec *mocks.NoOpRecorder
		mgr *VolumeManager
	)

	init := func() {
		rec = &mocks.NoOpRecorder{}
		mgr = &VolumeManager{recorder: rec}
	}

	expectEvent := func(drive *drivecrd.Drive, eventtype, reason string) bool {
		for _, c := range rec.Calls {
			driveObj, ok := c.Object.(*drivecrd.Drive)
			if !ok {
				continue
			}
			if driveObj.Name != drive.Name {
				continue
			}
			if c.Eventtype == eventtype && c.Reason == reason {
				return true
			}
		}
		return false
	}

	t.Run("Healthy drives discovered", func(t *testing.T) {
		init()
		upd := &driveUpdates{
			Created: []*drivecrd.Drive{drive1CR, drive2CR},
		}
		mgr.createEventsForDriveUpdates(upd)
		assert.NotEmpty(t, rec.Calls)
		msgDiscovered := "DriveDiscovered event should exist for drive"
		msgHealth := "DriveHealthGood event should exist for drive"
		assert.True(t, expectEvent(drive1CR, eventing.InfoType, eventing.DriveDiscovered), msgDiscovered)
		assert.True(t, expectEvent(drive2CR, eventing.InfoType, eventing.DriveDiscovered), msgDiscovered)
		assert.True(t, expectEvent(drive1CR, eventing.InfoType, eventing.DriveHealthGood), msgHealth)
		assert.True(t, expectEvent(drive2CR, eventing.InfoType, eventing.DriveHealthGood), msgHealth)
	})

	t.Run("No changes", func(t *testing.T) {
		init()
		upd := &driveUpdates{
			NotChanged: []*drivecrd.Drive{drive1CR, drive2CR},
		}
		mgr.createEventsForDriveUpdates(upd)
		assert.Empty(t, rec.Calls)
	})

	t.Run("Drive status and health changed", func(t *testing.T) {
		init()
		modifiedDrive := drive1CR.DeepCopy()
		modifiedDrive.Spec.Status = apiV1.DriveStatusOffline
		modifiedDrive.Spec.Health = apiV1.HealthUnknown

		upd := &driveUpdates{
			Updated: []updatedDrive{{
				PreviousState: drive1CR,
				CurrentState:  modifiedDrive}},
		}
		mgr.createEventsForDriveUpdates(upd)
		assert.True(t, expectEvent(drive1CR, eventing.ErrorType, eventing.DriveStatusOffline))
		assert.True(t, expectEvent(drive1CR, eventing.WarningType, eventing.DriveHealthUnknown))
	})
}

func TestVolumeManager_isShouldBeReconciled(t *testing.T) {
	var (
		vm  *VolumeManager
		vol vcrd.Volume
	)

	vm = prepareSuccessVolumeManager(t)
	vol = testVolumeCR1
	vol.Spec.NodeId = vm.nodeID
	assert.True(t, vm.isCorrespondedToNodePredicate(&vol))

	vol.Spec.NodeId = ""
	assert.False(t, vm.isCorrespondedToNodePredicate(&vol))

}

func TestVolumeManager_isDriveIsInLVG(t *testing.T) {
	vm := prepareSuccessVolumeManager(t)
	// there are no LVG CRs
	assert.False(t, vm.isDriveInLVG(drive1))
	// create LVG CR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVGCR.Name, &testLVGCR))

	assert.True(t, vm.isDriveInLVG(drive1))
	assert.False(t, vm.isDriveInLVG(drive2))
}

func prepareSuccessVolumeManager(t *testing.T) *VolumeManager {
	c := mocks.NewMockDriveMgrClient(nil)
	// create map of commands which must be mocked
	cmds := make(map[string]mocks.CmdOut)
	// list of all devices
	cmds[lsblkAllDevicesCmd] = mocks.CmdOut{Stdout: mocks.LsblkTwoDevicesStr}
	// list partitions of specific device
	cmds[lsblkSingleDeviceCmd] = mocks.CmdOut{Stdout: mocks.LsblkListPartitionsStr}
	e := mocks.NewMockExecutor(cmds)
	e.SetSuccessIfNotFound(true)

	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(c, e, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	vm.discoverLvgSSD = false
	return vm
}

func prepareSuccessVolumeManagerWithDrives(drives []*api.Drive, t *testing.T) *VolumeManager {
	nVM := prepareSuccessVolumeManager(t)
	nVM.driveMgrClient = mocks.NewMockDriveMgrClient(drives)
	for _, d := range drives {
		dCR := nVM.k8sClient.ConstructDriveCR(d.UUID, *d)
		if err := nVM.k8sClient.CreateCR(testCtx, dCR.Name, dCR); err != nil {
			panic(err)
		}
	}
	return nVM
}

func addDriveCRs(k *k8s.KubeClient, drives ...*drivecrd.Drive) {
	var err error
	for _, d := range drives {
		if err = k.CreateCR(testCtx, d.Name, d); err != nil {
			panic(err)
		}
	}
}

func getDriveMgrRespBasedOnDrives(drives ...api.Drive) []*api.Drive {
	resp := make([]*api.Drive, len(drives))
	for i, d := range drives {
		dd := d
		resp[i] = &dd
	}
	return resp
}

func TestVolumeManager_isDriveSystem(t *testing.T) {
	driveMgrRespDrives := getDriveMgrRespBasedOnDrives(drive1, drive2)
	hwMgrClient := mocks.NewMockDriveMgrClient(driveMgrRespDrives)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	listBlk := &mocklu.MockWrapLsblk{}
	vm := NewVolumeManager(*hwMgrClient, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	listBlk.On("GetBlockDevices", drive2.Path).Return([]lsblk.BlockDevice{bdev1}, nil).Once()
	vm.listBlk = listBlk
	isSystem, err := vm.isDriveSystem("/dev/sdb")
	assert.Nil(t, err)
	assert.Equal(t, false, isSystem)

	bdev1.MountPoint = base.KubeletRootPath
	listBlk.On("GetBlockDevices", drive2.Path).Return([]lsblk.BlockDevice{bdev1}, nil).Once()
	vm.listBlk = listBlk
	isSystem, err = vm.isDriveSystem("/dev/sdb")
	assert.Nil(t, err)
	assert.Equal(t, isSystem, true)

	listBlk.On("GetBlockDevices", drive2.Path).Return([]lsblk.BlockDevice{bdev1}, testErr).Once()
	vm.listBlk = listBlk
	isSystem, err = vm.isDriveSystem("/dev/sdb")
	assert.NotNil(t, err)
	assert.Equal(t, isSystem, false)
}
