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
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
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
	dataDiscover "github.com/dell/csi-baremetal/pkg/base/linuxutils/datadiscover/types"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/logger/objects"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/mocks"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
	mockProv "github.com/dell/csi-baremetal/pkg/mocks/provisioners"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
	wbtcommon "github.com/dell/csi-baremetal/pkg/node/wbt/common"
)

// TODO: refactor these UTs - https://github.com/dell/csi-baremetal/issues/90

var (
	testErr            = errors.New("error")
	lsblkAllDevicesCmd = fmt.Sprintf(lsblk.CmdTmpl, "")

	drive1UUID = uuid.New().String()
	drive2UUID = uuid.New().String()

	drive1 = api.Drive{
		UUID:         drive1UUID,
		SerialNumber: "hdd1-serial",
		Size:         1024 * 1024 * 1024 * 500,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthGood,
		Path:         "/dev/sda",
		IsSystem:     true,
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
		Size:     lsblk.CustomInt64{Int64: drive1.Size},
		Serial:   drive1.SerialNumber,
		Children: nil,
	}

	// todo don't hardcode device name
	lsblkSingleDeviceCmd = fmt.Sprintf(lsblk.CmdTmpl, "/dev/sda")

	testDriveCR = drivecrd.Drive{
		TypeMeta: v1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{
			Name:              drive1.UUID,
			CreationTimestamp: v1.Time{Time: time.Now()},
		},
		Spec: drive1,
	}

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
			// TODO location cannot be empty - need to add check
			Location:  drive1UUID,
			CSIStatus: apiV1.Creating,
			NodeId:    nodeID,
			Mode:      apiV1.ModeFS,
			Type:      string(fs.XFS),
		},
	}

	testLVGCR = lvgcrd.LogicalVolumeGroup{
		TypeMeta: v1.TypeMeta{
			Kind:       "LogicalVolumeGroup",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: testLVGName,
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
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)
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
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)
	newVolume := volCR.DeepCopy()
	newVolume.Spec.CSIStatus = apiV1.Creating
	err = vm.k8sClient.CreateCR(testCtx, newVolume.Name, newVolume)
	assert.Nil(t, err)

	pMock := mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		res, err := vm.Reconcile(testCtx, req)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		res, err := vm.Reconcile(testCtx, req)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)
		wg.Done()
	}()
	wg.Wait()
	volume := &vcrd.Volume{}
	err = vm.k8sClient.ReadCR(testCtx, req.Name, testNs, volume)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Created, volume.Spec.CSIStatus)
}

func TestReconcile_SuccessNotFound(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "not-found-that-name"}}
	res, err := vm.Reconcile(testCtx, req)
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
	testVol := volCR.DeepCopy()
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))
	pMock = mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err = vm.prepareVolume(testCtx, testVol)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, testNs, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Created)

	// failed to update
	vm = prepareSuccessVolumeManager(t)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})
	testVol = volCR.DeepCopy()

	res, err = vm.prepareVolume(testCtx, testVol)
	assert.NotNil(t, err)
	assert.True(t, res.Requeue)

	// PrepareVolume failed
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))
	pMock = &mockProv.MockProvisioner{}
	pMock.On("PrepareVolume", &testVol.Spec).Return(testErr)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err = vm.prepareVolume(testCtx, testVol)
	assert.NotNil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, testNs, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Failed)
}

func TestVolumeManager_handleRemovingStatus(t *testing.T) {
	var (
		vm  *VolumeManager
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
		res ctrl.Result
		err error
	)

	t.Run("happy path", func(t *testing.T) {
		vm = prepareSuccessVolumeManager(t)
		testVol := volCR.DeepCopy()
		testVol.Spec.CSIStatus = apiV1.Removing
		assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))
		drive := testDriveCR.DeepCopy()
		assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Spec.Location, drive))
		pMock := mockProv.GetMockProvisionerSuccess("/some/path")
		vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

		res, err = vm.handleRemovingStatus(testCtx, testVol)
		assert.Nil(t, err)
		assert.Equal(t, res, ctrl.Result{})
		volume := &vcrd.Volume{}
		err = vm.k8sClient.ReadCR(testCtx, req.Name, testNs, volume)
		assert.Nil(t, err)
		assert.Equal(t, apiV1.Removed, volume.Spec.CSIStatus)
	})

	t.Run("failed to update", func(t *testing.T) {
		vm = prepareSuccessVolumeManager(t)
		testVol := volCR.DeepCopy()
		drive := testDriveCR.DeepCopy()
		assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Spec.Location, drive))
		pMock := mockProv.GetMockProvisionerSuccess("/some/path")
		vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

		res, err = vm.handleRemovingStatus(testCtx, testVol)
		assert.NotNil(t, err)
		assert.True(t, res.Requeue)
	})

	t.Run("failed to get drive", func(t *testing.T) {
		vm = prepareSuccessVolumeManager(t)
		testVol := volCR.DeepCopy()
		testVol.Spec.CSIStatus = apiV1.Removing
		assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))

		res, err = vm.handleRemovingStatus(testCtx, testVol)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{Requeue: true}, res)
		volume := &vcrd.Volume{}
		err = vm.k8sClient.ReadCR(testCtx, req.Name, testNs, volume)
		assert.Nil(t, err)
		assert.Equal(t, volume.Spec.CSIStatus, apiV1.Removing)
	})

	t.Run("ReleaseVolume failed", func(t *testing.T) {
		vm = prepareSuccessVolumeManager(t)
		testVol := volCR.DeepCopy()
		testVol.Spec.CSIStatus = apiV1.Removing
		assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))
		drive := testDriveCR.DeepCopy()
		assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Spec.Location, drive))
		pMock := &mockProv.MockProvisioner{}
		pMock.On("ReleaseVolume", &testVol.Spec, &drive1).Return(testErr)
		vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

		res, err = vm.handleRemovingStatus(testCtx, testVol)
		assert.NotNil(t, err)
		assert.Equal(t, res, ctrl.Result{})
		volume := &vcrd.Volume{}
		err = vm.k8sClient.ReadCR(testCtx, req.Name, testNs, volume)
		assert.Nil(t, err)
		assert.Equal(t, volume.Spec.CSIStatus, apiV1.Failed)
	})

	t.Run("Volume missing", func(t *testing.T) {
		vm = prepareSuccessVolumeManager(t)
		testVol := volCR.DeepCopy()
		testVol.Spec.CSIStatus = apiV1.Removing
		testVol.Spec.OperationalStatus = apiV1.OperationalStatusMissing
		assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))

		res, err = vm.handleRemovingStatus(testCtx, testVol)
		assert.Nil(t, err)
		assert.Equal(t, res, ctrl.Result{})
		volume := &vcrd.Volume{}
		err = vm.k8sClient.ReadCR(testCtx, req.Name, testNs, volume)
		assert.Nil(t, err)
		assert.Equal(t, volume.Spec.CSIStatus, apiV1.Removed)
	})
}

func TestVolumeManager_handleRemovingStatus_DeleteVolume(t *testing.T) {
	drive := drive1
	drive.UUID = driveUUID
	drive.Health = apiV1.HealthGood
	testVol := volCR.DeepCopy()
	testVol.Spec.Location = drive.UUID
	testVol.Spec.CSIStatus = apiV1.Removing

	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{&drive}, t)
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))

	pMock := &mockProv.MockProvisioner{}
	pMock.On("ReleaseVolume", &testVol.Spec, &drive).Return(testErr)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err := vm.handleRemovingStatus(testCtx, testVol)
	assert.Error(t, testErr)

	driveCR := &drivecrd.Drive{}
	err = vm.k8sClient.ReadCR(context.Background(), testVol.Spec.Location, "", driveCR)
	assert.Nil(t, err)

	assert.Equal(t, res, ctrl.Result{})
	assert.Equal(t, apiV1.DriveUsageFailed, driveCR.Spec.Usage)
}

func TestReconcile_SuccessDeleteVolume(t *testing.T) {
	removeVolume(t, apiV1.Removed)
}

func TestReconcile_DeleteFailedVolume(t *testing.T) {
	removeVolume(t, apiV1.Failed)
}

func removeVolume(t *testing.T, status string) {
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)
	newVolumeCR := volCR.DeepCopy()
	newVolumeCR.Spec.CSIStatus = status
	err = vm.k8sClient.CreateCR(testCtx, newVolumeCR.Name, newVolumeCR)
	assert.Nil(t, err)

	pMock := mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	err = vm.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	//successfully add finalizer
	res, err := vm.Reconcile(testCtx, req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	//successfully remove finalizer
	err = vm.k8sClient.ReadCR(testCtx, newVolumeCR.Name, newVolumeCR.Namespace, newVolumeCR)
	assert.Nil(t, err)
	newVolumeCR.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	err = vm.k8sClient.UpdateCR(testCtx, newVolumeCR)
	assert.Nil(t, err)

	res, err = vm.Reconcile(testCtx, req)
	assert.NotNil(t, k8sError.IsNotFound(err))
	assert.Equal(t, res, ctrl.Result{})
}

func TestVolumeManager_handleCreatingVolumeInLVG(t *testing.T) {
	var (
		vm                 *VolumeManager
		pMock              *mockProv.MockProvisioner
		vol                *vcrd.Volume
		lvg                *lvgcrd.LogicalVolumeGroup
		testVol            = testVolumeLVGCR.DeepCopy()
		testLVG            lvgcrd.LogicalVolumeGroup
		expectedResRequeue = ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}
		res                ctrl.Result
		err                error
	)

	// unable to read LogicalVolumeGroup (not found) and unable to update corresponding volume CR
	vm = prepareSuccessVolumeManager(t)

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))
	assert.Equal(t, expectedResRequeue, res)

	// LogicalVolumeGroup is not found, volume CR was updated successfully (CSIStatus=failed)
	vm = prepareSuccessVolumeManager(t)
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, testVol.Namespace, vol))
	assert.Equal(t, apiV1.Failed, vol.Spec.CSIStatus)

	// LogicalVolumeGroup in creating state
	vm = prepareSuccessVolumeManager(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Creating
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
	assert.Nil(t, err)
	assert.Equal(t, expectedResRequeue, res)

	// LogicalVolumeGroup in failed state and volume is updated successfully
	vm = prepareSuccessVolumeManager(t)
	testVol = testVolumeLVGCR.DeepCopy()
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Failed
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, testVol.Namespace, vol))
	assert.Equal(t, apiV1.Failed, vol.Spec.CSIStatus)

	// LogicalVolumeGroup in failed state and volume is failed to update
	vm = prepareSuccessVolumeManager(t)
	testVol = testVolumeLVGCR.DeepCopy()
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Failed
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
	assert.NotNil(t, err)
	assert.Equal(t, expectedResRequeue, res)
	assert.True(t, k8sError.IsNotFound(err))

	// LogicalVolumeGroup in created state and volume.ID is not in VolumeRefs
	vm = prepareSuccessVolumeManager(t)
	testVol = testVolumeLVGCR.DeepCopy()
	pMock = &mockProv.MockProvisioner{}
	pMock.On("PrepareVolume", mock.Anything).Return(nil)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.LVMBasedVolumeType: pMock})
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Created
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	lvg = &lvgcrd.LogicalVolumeGroup{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testLVG.Name, "", lvg))
	assert.True(t, util.ContainsString(lvg.Spec.VolumeRefs, testVol.Spec.Id))

	// LogicalVolumeGroup in created state and volume.ID is in VolumeRefs
	vm = prepareSuccessVolumeManager(t)
	testVol = testVolumeLVGCR.DeepCopy()
	pMock = &mockProv.MockProvisioner{}
	pMock.On("PrepareVolume", mock.Anything).Return(nil)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.LVMBasedVolumeType: pMock})
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Created
	testLVG.Spec.VolumeRefs = []string{testVol.Spec.Id}
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	lvg = &lvgcrd.LogicalVolumeGroup{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testLVG.Name, "", lvg))
	assert.True(t, util.ContainsString(lvg.Spec.VolumeRefs, testVol.Spec.Id))
	assert.Equal(t, 1, len(lvg.Spec.VolumeRefs))

	// LogicalVolumeGroup state wasn't recognized
	vm = prepareSuccessVolumeManager(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = ""
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, testVol)
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
	newVolumeCR := volCR.DeepCopy()
	newVolumeCR.Spec.CSIStatus = apiV1.Published
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, newVolumeCR.Name, newVolumeCR))

	res, err = vm.Reconcile(testCtx, req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func TestNewVolumeManager_SetProvisioners(t *testing.T) {
	vm := NewVolumeManager(nil, mocks.EmptyExecutorSuccess{},
		logrus.New(), nil, nil, new(mocks.NoOpRecorder), nodeID, nodeName)
	newProv := &mockProv.MockProvisioner{}
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: newProv})
	assert.Equal(t, newProv, vm.provisioners[p.DriveBasedVolumeType])
}

func TestVolumeManager_DiscoverFail(t *testing.T) {
	var (
		vm  *VolumeManager
		err error
	)

	t.Run("driveMgr return error", func(t *testing.T) {
		vm = prepareSuccessVolumeManager(t)
		vm.driveMgrClient = &mocks.MockDriveMgrClientFail{}

		err = vm.Discover()
		assert.NotNil(t, err)
		assert.Equal(t, "drivemgr error", err.Error())
	})

	t.Run("update driveCRs failed", func(t *testing.T) {
		mockK8sClient := &mocks.K8Client{}
		kubeClient := k8s.NewKubeClient(mockK8sClient, testLogger, objects.NewObjectLogger(), testNs)
		// expect: updateDrivesCRs failed
		vm = NewVolumeManager(&mocks.MockDriveMgrClient{},
			nil, testLogger, kubeClient, kubeClient, nil, nodeID, nodeName)
		mockK8sClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(testErr).Once()

		err = vm.Discover()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "updateDrivesCRs return error")
	})

	t.Run("discoverDataOnDrives failed", func(t *testing.T) {
		mockK8sClient := &mocks.K8Client{}
		kubeClient := k8s.NewKubeClient(mockK8sClient, testLogger, objects.NewObjectLogger(), testNs)
		vm = NewVolumeManager(&mocks.MockDriveMgrClient{}, nil, testLogger, kubeClient, kubeClient, nil, nodeID, nodeName)
		discoverData := &mocklu.MockWrapDataDiscover{}
		discoverData.On("DiscoverData", mock.Anything, mock.Anything).Return(false, testErr).Once()
		vm.dataDiscover = discoverData
		vm.discoverSystemLVG = false
		mockK8sClient.On("List", mock.Anything, &drivecrd.DriveList{}, mock.Anything).Return(nil)
		mockK8sClient.On("List", mock.Anything, &lvgcrd.LogicalVolumeGroupList{}, mock.Anything).Return(nil)
		mockK8sClient.On("List", mock.Anything, &vcrd.VolumeList{}, mock.Anything).Return(testErr)

		err = vm.Discover()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "discoverDataOnDrives return error")
	})
}

func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	var (
		vm             *VolumeManager
		driveMgrClient = mocks.NewMockDriveMgrClient(getDriveMgrRespBasedOnDrives(drive1, drive2))
		err            error
	)

	vm = prepareSuccessVolumeManager(t)
	vm.driveMgrClient = driveMgrClient
	discoverData := &mocklu.MockWrapDataDiscover{}
	discoverData.On("DiscoverData", mock.Anything, mock.Anything).Return(&dataDiscover.DiscoverResult{}, nil)
	vm.dataDiscover = discoverData
	// expect that Volume CRs won't be created because of all drives don't have children
	err = vm.Discover()
	assert.Nil(t, err)
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
	dItems := getDriveCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 0, len(dItems))

	discoverData := &mocklu.MockWrapDataDiscover{}
	testResultDrive1 := &dataDiscover.DiscoverResult{
		Message: fmt.Sprintf("Drive with path %s, SN %s, doesn't have filesystem, partition table, partitions and PV", drive1.Path, drive1.SerialNumber),
		HasData: false,
	}
	testResultDrive2 := &dataDiscover.DiscoverResult{
		Message: fmt.Sprintf("Drive with path %s, SN %s, doesn't have filesystem, partition table, partitions and PV", drive2.Path, drive2.SerialNumber),
		HasData: false,
	}
	discoverData.On("DiscoverData", drive1.Path, drive1.SerialNumber).Return(testResultDrive1, nil)
	discoverData.On("DiscoverData", drive2.Path, drive2.SerialNumber).Return(testResultDrive2, nil)
	vm.dataDiscover = discoverData

	err := vm.Discover()
	assert.Nil(t, err)

	dItems = getDriveCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 2, len(dItems))
	assert.Equal(t, true, dItems[0].Spec.IsClean)
	assert.Equal(t, true, dItems[1].Spec.IsClean)

	// second iteration
	discoverData = &mocklu.MockWrapDataDiscover{}
	testResultDrive1 = &dataDiscover.DiscoverResult{
		Message: fmt.Sprintf("Drive with path %s, SN %s, has filesystem", drive1.Path, drive1.SerialNumber),
		HasData: true,
	}
	testResultDrive2 = &dataDiscover.DiscoverResult{
		Message: fmt.Sprintf("Drive with path %s, SN %s, has filesystem", drive2.Path, drive2.SerialNumber),
		HasData: true,
	}
	discoverData.On("DiscoverData", drive1.Path, drive1.SerialNumber).Return(testResultDrive1, nil)
	discoverData.On("DiscoverData", drive2.Path, drive2.SerialNumber).Return(testResultDrive2, nil)
	vm.dataDiscover = discoverData
	err = vm.Discover()
	assert.Nil(t, err)

	dItems = getDriveCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 2, len(dItems))
	assert.Equal(t, false, dItems[0].Spec.IsClean)
	assert.Equal(t, false, dItems[1].Spec.IsClean)
}

func TestVolumeManager_updatesDrivesCRs_Success(t *testing.T) {

	t.Run("happy path", func(t *testing.T) {
		vm := prepareSuccessVolumeManager(t)
		driveMgrRespDrives := getDriveMgrRespBasedOnDrives(drive1, drive2)
		vm.driveMgrClient = mocks.NewMockDriveMgrClient(driveMgrRespDrives)

		updates, err := vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
		assert.Nil(t, err)
		driveCRs, err := vm.crHelper.GetDriveCRs(vm.nodeID)
		assert.Nil(t, err)
		assert.Equal(t, len(driveCRs), 2)
		assert.Len(t, updates.Created, 2)
	})

	t.Run("disk become bad", func(t *testing.T) {
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
		updatedDrive := &drivecrd.Drive{}
		assert.Nil(t, vm.k8sClient.ReadCR(testCtx, driveMgrRespDrives[0].UUID, "", updatedDrive))
		assert.Equal(t, updatedDrive.Spec.Health, apiV1.HealthBad)
		assert.Len(t, updates.Updated, 1)
		assert.Len(t, updates.NotChanged, 1)
	})

	t.Run("missing disk", func(t *testing.T) {
		vm := prepareSuccessVolumeManager(t)
		driveMgrRespDrives := getDriveMgrRespBasedOnDrives(drive1, drive2)
		vm.driveMgrClient = mocks.NewMockDriveMgrClient(driveMgrRespDrives)

		updates, err := vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
		assert.Nil(t, err)
		driveCRs, err := vm.crHelper.GetDriveCRs(vm.nodeID)
		assert.Nil(t, err)
		assert.Equal(t, len(driveCRs), 2)
		assert.Len(t, updates.Created, 2)

		drive := driveCRs[0]
		updates, err = vm.updateDrivesCRs(testCtx, []*api.Drive{&drive.Spec})
		assert.Nil(t, err)
		updatedDrive := &drivecrd.Drive{}
		assert.Nil(t, vm.k8sClient.ReadCR(testCtx, driveCRs[1].Name, "", updatedDrive))
		assert.Equal(t, updatedDrive.Spec.Health, apiV1.HealthUnknown)
		assert.Equal(t, updatedDrive.Spec.Status, apiV1.DriveStatusOffline)
		assert.Len(t, updates.Updated, 1)
		assert.Len(t, updates.NotChanged, 1)
	})

	t.Run("health bad annotation", func(t *testing.T) {
		vm := prepareSuccessVolumeManager(t)
		driveMgrRespDrives := getDriveMgrRespBasedOnDrives(drive1, drive2)
		vm.driveMgrClient = mocks.NewMockDriveMgrClient(driveMgrRespDrives)

		updates, err := vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
		assert.Nil(t, err)
		driveCRs, err := vm.crHelper.GetDriveCRs(vm.nodeID)
		assert.Nil(t, err)
		assert.Equal(t, len(driveCRs), 2)
		assert.Len(t, updates.Created, 2)

		drive := driveCRs[0]
		drive.Annotations = map[string]string{"health": "bad"}
		_ = vm.k8sClient.UpdateCR(testCtx, &drive)
		updates, err = vm.updateDrivesCRs(testCtx, []*api.Drive{&driveCRs[0].Spec, &driveCRs[1].Spec})

		actualDrive := &drivecrd.Drive{}
		assert.Nil(t, vm.k8sClient.ReadCR(testCtx, drive.Name, "", actualDrive))
		assert.Nil(t, err)
		assert.Equal(t, actualDrive.Spec.Health, apiV1.HealthBad)
	})

	t.Run("new drive", func(t *testing.T) {
		vm := prepareSuccessVolumeManager(t)
		driveMgrRespDrives := getDriveMgrRespBasedOnDrives(drive1, drive2)
		vm.driveMgrClient = mocks.NewMockDriveMgrClient(driveMgrRespDrives)

		updates, err := vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
		assert.Nil(t, err)
		driveCRs, err := vm.crHelper.GetDriveCRs(vm.nodeID)
		assert.Nil(t, err)
		assert.Equal(t, len(driveCRs), 2)
		assert.Len(t, updates.Created, 2)

		driveMgrRespDrives = []*api.Drive{&driveCRs[0].Spec, &driveCRs[1].Spec, {
			UUID:         uuid.New().String(),
			SerialNumber: "hdd3",
			Health:       apiV1.HealthGood,
			Type:         apiV1.DriveTypeHDD,
			Size:         1024 * 1024 * 1024 * 150,
			NodeId:       nodeID,
		}}
		updates, err = vm.updateDrivesCRs(testCtx, driveMgrRespDrives)
		assert.Nil(t, err)
		driveCRs, err = vm.crHelper.GetDriveCRs(vm.nodeID)
		assert.Nil(t, err)
		assert.Equal(t, len(driveCRs), 3)
		assert.Len(t, updates.Created, 1)
		assert.Len(t, updates.NotChanged, 2)
	})
}

func TestVolumeManager_updatesDrivesCRs_Fail(t *testing.T) {
	mockK8sClient := &mocks.K8Client{}
	kubeClient := k8s.NewKubeClient(mockK8sClient, testLogger, objects.NewObjectLogger(), testNs)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)

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
	mockK8sClient.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(testErr).Twice() // CreateCR will failed

	d1 := drive1
	res, err = vm.updateDrivesCRs(testCtx, []*api.Drive{&d1})
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 1, len(res.Created))
}

func TestVolumeManager_updatesDrivesCRs_Override(t *testing.T) {
	client, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, client, client, new(mocks.NoOpRecorder), nodeID, nodeName)

	testDriveCR1 := testDriveCR.DeepCopy()
	testDriveCR1.Annotations = map[string]string{driveHealthOverrideAnnotation: apiV1.HealthSuspect}
	err = client.CreateCR(testCtx, testDriveCR1.Name, testDriveCR1)
	assert.Nil(t, err)

	drive := drive1
	drives := []*api.Drive{&drive}

	updates, err := vm.updateDrivesCRs(testCtx, drives)
	assert.Nil(t, err)
	assert.Equal(t, len(updates.Updated), 1)
	assert.Equal(t, updates.Updated[0].PreviousState.Spec.Health, drive1.Health)
	assert.Equal(t, updates.Updated[0].CurrentState.Spec.Health, apiV1.HealthSuspect)
}

func TestVolumeManager_handleDriveStatusChange(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives(nil, t)

	ac := acCR
	err := vm.k8sClient.CreateCR(testCtx, ac.Name, &ac)
	assert.Nil(t, err)

	drive := drive1
	drive.UUID = driveUUID
	drive.Health = apiV1.HealthBad
	driveCR := &drivecrd.Drive{Spec: drive}

	update := updatedDrive{
		PreviousState: driveCR,
		CurrentState:  driveCR,
	}

	// Check AC deletion
	vm.handleDriveStatusChange(testCtx, update)
	vol := volCR.DeepCopy()
	vol.Spec.Location = driveUUID
	err = vm.k8sClient.CreateCR(testCtx, testID, vol)
	assert.Nil(t, err)

	// Check volume's health change
	vm.handleDriveStatusChange(testCtx, update)
	rVolume := &vcrd.Volume{}
	err = vm.k8sClient.ReadCR(testCtx, testID, vol.Namespace, rVolume)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.HealthBad, rVolume.Spec.Health)

	lvg := testLVGCR
	lvg.Spec.Locations = []string{driveUUID}
	err = vm.k8sClient.CreateCR(testCtx, testLVGName, &lvg)
	assert.Nil(t, err)
	// Check lvg's health change
	vm.handleDriveStatusChange(testCtx, update)
	updatedLVG := &lvgcrd.LogicalVolumeGroup{}
	err = vm.k8sClient.ReadCR(testCtx, testLVGName, "", updatedLVG)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.HealthBad, updatedLVG.Spec.Health)
}

func Test_discoverLVGOnSystemDrive_LVGAlreadyExists(t *testing.T) {
	var (
		m     = prepareSuccessVolumeManager(t)
		lvgCR = m.k8sClient.ConstructLVGCR("some-name", api.LogicalVolumeGroup{
			Name:      "some-name",
			Node:      m.nodeID,
			Locations: []string{"some-uuid"},
		})
		lvgList = lvgcrd.LogicalVolumeGroupList{}
		err     error
	)
	lvmOps := &mocklu.MockWrapLVM{}
	lvmOps.On("GetVgFreeSpace", "some-name").Return(int64(0), nil)
	m.lvmOps = lvmOps
	m.systemDrivesUUIDs = append(m.systemDrivesUUIDs, lvgCR.Spec.Locations...)

	err = m.k8sClient.CreateCR(testCtx, lvgCR.Name, lvgCR.DeepCopy())
	assert.Nil(t, err)

	err = m.discoverLVGOnSystemDrive()
	assert.Nil(t, err)

	err = m.k8sClient.ReadList(testCtx, &lvgList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvgList.Items))
	assert.Equal(t, lvgCR.Spec, lvgList.Items[0].Spec)

	// increase free space on lvg
	lvmOps = &mocklu.MockWrapLVM{}
	lvmOps.On("GetVgFreeSpace", "some-name").Return(int64(2*1024*1024), nil)
	m.lvmOps = lvmOps

	err = m.k8sClient.CreateCR(testCtx, lvgCR.Name, lvgCR.DeepCopy())
	assert.Nil(t, err)

	err = m.discoverLVGOnSystemDrive()
	assert.Nil(t, err)

	err = m.k8sClient.ReadList(testCtx, &lvgList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvgList.Items))
	assert.Equal(t, lvgCR.Spec, lvgList.Items[0].Spec)

}

func Test_discoverLVGOnSystemDrive_LVGCreatedACNo(t *testing.T) {
	var (
		m             = prepareSuccessVolumeManager(t)
		lvgList       = lvgcrd.LogicalVolumeGroupList{}
		listBlk       = &mocklu.MockWrapLsblk{}
		fsOps         = &mockProv.MockFsOpts{}
		lvmOps        = &mocklu.MockWrapLVM{}
		vgName        = "root-vg"
		systemDriveCR = testDriveCR.DeepCopy()
		err           error
	)

	m.listBlk = listBlk
	m.fsOps = fsOps
	m.lvmOps = lvmOps

	pvName := testDriveCR.Spec.Path + "1"
	lvmOps.On("GetAllPVs").Return([]string{pvName, "/dev/sdx"}, nil)
	lvmOps.On("GetVGNameByPVName", pvName).Return(vgName, nil)
	lvmOps.On("GetVgFreeSpace", vgName).Return(int64(1024), nil)
	lvmOps.On("GetLVsInVG", vgName).Return([]string{"lv_swap", "lv_boot"}, nil).Once()

	assert.Nil(t, m.k8sClient.CreateCR(testCtx, systemDriveCR.Name, systemDriveCR))

	// expect success, LogicalVolumeGroup CR and AC CR was created
	m.systemDrivesUUIDs = append(m.systemDrivesUUIDs, systemDriveCR.Spec.UUID)
	err = m.discoverLVGOnSystemDrive()
	assert.Nil(t, err)

	err = m.k8sClient.ReadList(testCtx, &lvgList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvgList.Items))
	lvg := lvgList.Items[0]
	assert.Equal(t, 1, len(lvg.Spec.Locations))
	assert.Equal(t, testDriveCR.Spec.UUID, lvg.Spec.Locations[0])
	assert.Equal(t, apiV1.Created, lvg.Spec.Status)
	assert.Equal(t, 2, len(lvg.Spec.VolumeRefs))

	// unable to read LVs in system vg
	m = prepareSuccessVolumeManager(t)
	// mocks were setup for previous scenario
	m.listBlk = listBlk
	m.fsOps = fsOps
	m.lvmOps = lvmOps

	lvmOps.On("GetLVsInVG", vgName).Return(nil, testErr)

	systemDriveCR = testDriveCR.DeepCopy()
	assert.Nil(t, m.k8sClient.CreateCR(testCtx, systemDriveCR.Name, systemDriveCR))
	m.systemDrivesUUIDs = append(m.systemDrivesUUIDs, systemDriveCR.Spec.UUID)

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

	expectEvent := func(drive *drivecrd.Drive, event *eventing.EventDescription) bool {
		for _, c := range rec.Calls {
			driveObj, ok := c.Object.(*drivecrd.Drive)
			if !ok {
				continue
			}
			if driveObj.Name != drive.Name {
				continue
			}
			if reflect.DeepEqual(c.Event, event) {
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
		assert.True(t, expectEvent(drive1CR, eventing.DriveDiscovered), msgDiscovered)
		assert.True(t, expectEvent(drive2CR, eventing.DriveDiscovered), msgDiscovered)
		assert.True(t, expectEvent(drive1CR, eventing.DriveHealthGood), msgHealth)
		assert.True(t, expectEvent(drive2CR, eventing.DriveHealthGood), msgHealth)
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
		assert.True(t, expectEvent(drive1CR, eventing.DriveStatusOffline))
		assert.True(t, expectEvent(drive1CR, eventing.DriveHealthUnknown))
	})

	t.Run("Drive health overridden", func(t *testing.T) {
		init()
		modifiedDrive := drive1CR.DeepCopy()
		modifiedDrive.Annotations = map[string]string{"health": "bad"}
		modifiedDrive.Spec.Health = apiV1.HealthBad

		upd := &driveUpdates{
			Updated: []updatedDrive{{
				PreviousState: drive1CR,
				CurrentState:  modifiedDrive}},
		}
		mgr.createEventsForDriveUpdates(upd)
		assert.True(t, expectEvent(drive1CR, eventing.DriveHealthOverridden))
		assert.True(t, expectEvent(drive1CR, eventing.DriveHealthFailure))
	})

	// disk is offline
	// health=bad annotation placed - DR initiated
	t.Run("Missing disk replacement", func(t *testing.T) {
		init()
		modifiedDrive := drive1CR.DeepCopy()
		modifiedDrive.Annotations = map[string]string{"health": "bad"}
		modifiedDrive.Spec.Health = apiV1.HealthBad
		modifiedDrive.Spec.Status = apiV1.DriveStatusOffline

		upd := &driveUpdates{
			Updated: []updatedDrive{{
				PreviousState: drive1CR,
				CurrentState:  modifiedDrive}},
		}
		mgr.createEventsForDriveUpdates(upd)
		assert.True(t, expectEvent(drive1CR, eventing.DriveHealthOverridden))
		assert.True(t, expectEvent(drive1CR, eventing.MissingDriveReplacementInitiated))
		assert.True(t, expectEvent(drive1CR, eventing.DriveHealthFailure))
	})

	t.Run("Drive removed", func(t *testing.T) {
		init()
		modifiedDrive := drive1CR.DeepCopy()
		modifiedDrive.Spec.Usage = apiV1.DriveUsageRemoved
		modifiedDrive.Spec.Status = apiV1.DriveStatusOffline
		modifiedDrive.Spec.Health = apiV1.HealthUnknown

		upd := &driveUpdates{
			Updated: []updatedDrive{{
				PreviousState: drive1CR,
				CurrentState:  modifiedDrive}},
		}
		mgr.createEventsForDriveUpdates(upd)
		assert.True(t, expectEvent(drive1CR, eventing.DriveSuccessfullyRemoved))
		assert.True(t, expectEvent(drive1CR, eventing.DriveHealthUnknown))
	})
}

func TestVolumeManager_isShouldBeReconciled(t *testing.T) {
	var (
		vm  *VolumeManager = prepareSuccessVolumeManager(t)
		vol *vcrd.Volume   = testVolumeCR1.DeepCopy()
	)

	vol.Spec.NodeId = vm.nodeID
	assert.True(t, vm.isCorrespondedToNodePredicate(vol))

	vol.Spec.NodeId = ""
	assert.False(t, vm.isCorrespondedToNodePredicate(vol))

}

func TestVolumeManager_isDriveIsInLVG(t *testing.T) {
	vm := prepareSuccessVolumeManager(t)
	drive1 := api.Drive{UUID: drive1UUID}
	drive2 := api.Drive{UUID: drive2UUID}
	lvgCR := lvgcrd.LogicalVolumeGroup{
		TypeMeta: v1.TypeMeta{
			Kind:       "LogicalVolumeGroup",
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
	// there are no LogicalVolumeGroup CRs
	assert.False(t, vm.isDriveInLVG(drive1))
	// create LogicalVolumeGroup CR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, lvgCR.Name, &lvgCR))

	assert.True(t, vm.isDriveInLVG(drive1))
	assert.False(t, vm.isDriveInLVG(drive2))
}

func TestVolumeManager_handleExpandingStatus(t *testing.T) {
	var (
		vm      *VolumeManager
		pMock   *mockProv.MockProvisioner
		vol     *vcrd.Volume
		testVol vcrd.Volume
		res     ctrl.Result
		err     error
	)

	vm = prepareSuccessVolumeManager(t)
	pMock = &mockProv.MockProvisioner{}
	pMock.On("GetVolumePath", &testVol.Spec).Return("path", testErr)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.LVMBasedVolumeType: pMock})
	res, err = vm.handleExpandingStatus(testCtx, &testVol)
	assert.NotNil(t, err)

	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, &testVol))

	pMock = &mockProv.MockProvisioner{}
	pMock.On("GetVolumePath", &testVol.Spec).Return("path", nil)
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.LVMBasedVolumeType: pMock})
	lvmOps := &mocklu.MockWrapLVM{}
	lvmOps.On("ExpandLV", "path", testVol.Spec.Size).Return(fmt.Errorf("error"))
	vm.lvmOps = lvmOps
	res, err = vm.handleExpandingStatus(testCtx, &testVol)
	assert.NotNil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, testVol.Namespace, vol))
	assert.Equal(t, apiV1.Failed, vol.Spec.CSIStatus)

	pMock.On("GetVolumePath", &vol.Spec).Return("path", nil)

	lvmOps = &mocklu.MockWrapLVM{}
	lvmOps.On("ExpandLV", "path", vol.Spec.Size).Return(nil)
	vm.lvmOps = lvmOps
	assert.Nil(t, vm.k8sClient.UpdateCR(testCtx, &testVol))
	res, err = vm.handleExpandingStatus(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, testVol.Namespace, vol))
	assert.Equal(t, apiV1.Resized, vol.Spec.CSIStatus)
}

func TestVolumeManager_discoverDataOnDrives(t *testing.T) {
	t.Run("Disk has data", func(t *testing.T) {
		var vm *VolumeManager
		vm = prepareSuccessVolumeManager(t)
		testDrive := testDriveCR
		testDrive.Spec.Path = "/dev/sda"
		testDrive.Spec.IsClean = true
		discoverData := &mocklu.MockWrapDataDiscover{}
		testResult := &dataDiscover.DiscoverResult{
			Message: fmt.Sprintf("Drive with path %s, SN %s, has filesystem", testDrive.Spec.Path, testDrive.Spec.SerialNumber),
			HasData: true,
		}
		discoverData.On("DiscoverData", testDrive.Spec.Path, testDrive.Spec.SerialNumber).Return(testResult, nil).Once()
		vm.dataDiscover = discoverData

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, &testDrive)
		assert.Nil(t, err)

		err = vm.discoverDataOnDrives()
		assert.Nil(t, err)

		newDrive := &drivecrd.Drive{}
		err = vm.k8sClient.ReadCR(testCtx, testDrive.Name, "", newDrive)
		assert.Nil(t, err)

		assert.Equal(t, false, newDrive.Spec.IsClean)
	})
	t.Run("Disk has data and field IsClean is false", func(t *testing.T) {
		var vm *VolumeManager
		vm = prepareSuccessVolumeManager(t)

		testDrive := testDriveCR
		testDrive.Spec.Path = "/dev/sda"
		testDrive.Spec.IsClean = false

		discoverData := &mocklu.MockWrapDataDiscover{}
		testResult := &dataDiscover.DiscoverResult{
			Message: fmt.Sprintf("Drive with path %s, SN %s, has filesystem", testDrive.Spec.Path, testDrive.Spec.SerialNumber),
			HasData: true,
		}
		discoverData.On("DiscoverData", testDrive.Spec.Path, testDrive.Spec.SerialNumber).Return(testResult, nil).Once()
		vm.dataDiscover = discoverData

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, &testDrive)
		assert.Nil(t, err)

		err = vm.discoverDataOnDrives()
		assert.Nil(t, err)

		newDrive := &drivecrd.Drive{}
		err = vm.k8sClient.ReadCR(testCtx, testDrive.Name, "", newDrive)
		assert.Nil(t, err)

		assert.Equal(t, false, newDrive.Spec.IsClean)
	})
	t.Run("DiscoverData function failed", func(t *testing.T) {
		var vm *VolumeManager
		vm = prepareSuccessVolumeManager(t)
		testDrive := testDriveCR
		testDrive.Spec.Path = "/dev/sda"
		discoverData := &mocklu.MockWrapDataDiscover{}

		discoverData.On("DiscoverData", testDrive.Spec.Path, testDrive.Spec.SerialNumber).
			Return(&dataDiscover.DiscoverResult{}, testErr).Once()
		vm.dataDiscover = discoverData

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, &testDrive)
		assert.Nil(t, err)

		err = vm.discoverDataOnDrives()
		assert.Nil(t, err)
		newDrive := &drivecrd.Drive{}
		err = vm.k8sClient.ReadCR(testCtx, testDriveCR.Name, "", newDrive)
		assert.Nil(t, err)
		assert.Equal(t, false, newDrive.Spec.IsClean)
	})

	t.Run("Drive doesn't have data", func(t *testing.T) {
		var vm *VolumeManager
		vm = prepareSuccessVolumeManager(t)
		testDrive := testDriveCR
		testDrive.Spec.Path = "/dev/sda"

		discoverData := &mocklu.MockWrapDataDiscover{}
		vm.dataDiscover = discoverData
		testResult := &dataDiscover.DiscoverResult{
			Message: fmt.Sprintf("Drive with path %s, SN %s doesn't have filesystem, partition table, partitions and PV", testDrive.Spec.Path, testDrive.Spec.SerialNumber),
			HasData: false,
		}
		discoverData.On("DiscoverData", testDrive.Spec.Path, testDrive.Spec.SerialNumber).Return(testResult, nil).Once()
		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, &testDrive)
		assert.Nil(t, err)

		err = vm.discoverDataOnDrives()
		assert.Nil(t, err)
		newDrive := &drivecrd.Drive{}
		err = vm.k8sClient.ReadCR(testCtx, testDriveCR.Name, "", newDrive)
		assert.Nil(t, err)
		assert.Equal(t, true, newDrive.Spec.IsClean)
	})
}

func TestVolumeManager_WbtConfiguration(t *testing.T) {
	// setWbtValue UT
	t.Run("setWbtValue: success", func(t *testing.T) {
		var (
			testVol          = volCR.DeepCopy()
			testDrive        = testDriveCR.DeepCopy()
			wbtValue  uint32 = 0
			mockWbt          = &mocklu.MockWrapWbt{}
			device           = "sda" //testDrive.Spec.Path = "/dev/sda"
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtOps = mockWbt
		vm.wbtConfig = &wbtcommon.WbtConfig{Value: wbtValue}

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		mockWbt.On("SetValue", device, wbtValue).Return(nil)

		err = vm.setWbtValue(testVol)
		assert.Nil(t, err)
	})
	t.Run("setWbtValue: findDeviceName failed", func(t *testing.T) {
		var (
			testVol   = volCR.DeepCopy()
			testDrive = testDriveCR.DeepCopy()
		)
		vm := prepareSuccessVolumeManager(t)

		testDrive.Spec.Path = "/dev"

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		err = vm.setWbtValue(testVol)
		assert.NotNil(t, err)
	})
	t.Run("setWbtValue: wbtOps.SetValue failed", func(t *testing.T) {
		var (
			testVol          = volCR.DeepCopy()
			testDrive        = testDriveCR.DeepCopy()
			wbtValue  uint32 = 0
			mockWbt          = &mocklu.MockWrapWbt{}
			device           = "sda" //testDrive.Spec.Path = "/dev/sda"
			mockErr          = fmt.Errorf("some error")
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtOps = mockWbt
		vm.wbtConfig = &wbtcommon.WbtConfig{Value: wbtValue}

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		mockWbt.On("SetValue", device, wbtValue).Return(mockErr)

		err = vm.setWbtValue(testVol)
		assert.NotNil(t, err)
		assert.Equal(t, mockErr, err)
	})

	// restoreWbtValue UT
	t.Run("restoreWbtValue: success", func(t *testing.T) {
		var (
			testVol   = volCR.DeepCopy()
			testDrive = testDriveCR.DeepCopy()
			mockWbt   = &mocklu.MockWrapWbt{}
			device    = "sda" //testDrive.Spec.Path = "/dev/sda"
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtOps = mockWbt

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		mockWbt.On("RestoreDefault", device).Return(nil)

		err = vm.restoreWbtValue(testVol)
		assert.Nil(t, err)
	})
	t.Run("restoreWbtValue: findDeviceName failed", func(t *testing.T) {
		var (
			testVol   = volCR.DeepCopy()
			testDrive = testDriveCR.DeepCopy()
		)
		vm := prepareSuccessVolumeManager(t)

		testDrive.Spec.Path = "/dev"

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		err = vm.restoreWbtValue(testVol)
		assert.NotNil(t, err)
	})
	t.Run("restoreWbtValue: wbtOps.RestoreDefault failed", func(t *testing.T) {
		var (
			testVol   = volCR.DeepCopy()
			testDrive = testDriveCR.DeepCopy()
			mockWbt   = &mocklu.MockWrapWbt{}
			device    = "sda" //testDrive.Spec.Path = "/dev/sda"
			mockErr   = fmt.Errorf("some error")
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtOps = mockWbt

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		mockWbt.On("RestoreDefault", device).Return(mockErr)

		err = vm.restoreWbtValue(testVol)
		assert.NotNil(t, err)
		assert.Equal(t, mockErr, err)
	})

	// checkWbtChangingEnable
	t.Run("checkWbtChangingEnable: success", func(t *testing.T) {
		var (
			testVol    = volCR.DeepCopy()
			volumeMode = apiV1.ModeFS
			volumeSC   = "csi-baremetal-sc-hdd"
			wbtConf    = &wbtcommon.WbtConfig{
				Enable: true,
				VolumeOptions: wbtcommon.VolumeOptions{
					Modes:          []string{volumeMode},
					StorageClasses: []string{volumeSC},
				},
			}
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtConfig = wbtConf

		pv := &corev1.PersistentVolume{}
		pv.Name = testVol.Name
		pv.Spec.StorageClassName = volumeSC
		err := vm.k8sClient.Create(testCtx, pv)
		assert.Nil(t, err)

		result := vm.checkWbtChangingEnable(testCtx, testVol)
		assert.True(t, result)
	})
	t.Run("checkWbtChangingEnable: wbtConf is nil", func(t *testing.T) {
		var (
			testVol = volCR.DeepCopy()
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtConfig = nil

		result := vm.checkWbtChangingEnable(testCtx, testVol)
		assert.False(t, result)
	})
	t.Run("checkWbtChangingEnable: wbt disabled", func(t *testing.T) {
		var (
			testVol    = volCR.DeepCopy()
			volumeMode = apiV1.ModeFS
			volumeSC   = "csi-baremetal-sc-hdd"
			wbtConf    = &wbtcommon.WbtConfig{
				Enable: false,
				VolumeOptions: wbtcommon.VolumeOptions{
					Modes:          []string{volumeMode},
					StorageClasses: []string{volumeSC},
				},
			}
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtConfig = wbtConf

		result := vm.checkWbtChangingEnable(testCtx, testVol)
		assert.False(t, result)
	})
	t.Run("checkWbtChangingEnable: wrong Mode", func(t *testing.T) {
		var (
			testVol   = volCR.DeepCopy()
			wrongMode = apiV1.ModeRAW
			volumeSC  = "csi-baremetal-sc-hdd"
			wbtConf   = &wbtcommon.WbtConfig{
				Enable: true,
				VolumeOptions: wbtcommon.VolumeOptions{
					Modes:          []string{wrongMode},
					StorageClasses: []string{volumeSC},
				},
			}
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtConfig = wbtConf

		result := vm.checkWbtChangingEnable(testCtx, testVol)
		assert.False(t, result)
	})
	t.Run("checkWbtChangingEnable: wrong SC", func(t *testing.T) {
		var (
			testVol    = volCR.DeepCopy()
			volumeMode = apiV1.ModeFS
			volumeSC   = "csi-baremetal-sc-hdd"
			wrongSC    = "csi-baremetal-sc-hddlvg"
			wbtConf    = &wbtcommon.WbtConfig{
				Enable: true,
				VolumeOptions: wbtcommon.VolumeOptions{
					Modes:          []string{volumeMode},
					StorageClasses: []string{wrongSC},
				},
			}
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtConfig = wbtConf

		pv := &corev1.PersistentVolume{}
		pv.Name = testVol.Name
		pv.Spec.StorageClassName = volumeSC
		err := vm.k8sClient.Create(testCtx, pv)
		assert.Nil(t, err)

		result := vm.checkWbtChangingEnable(testCtx, testVol)
		assert.False(t, result)
	})
	t.Run("checkWbtChangingEnable: PV not found", func(t *testing.T) {
		var (
			testVol    = volCR.DeepCopy()
			volumeMode = apiV1.ModeFS
			volumeSC   = "csi-baremetal-sc-hdd"
			wrongSC    = "csi-baremetal-sc-hddlvg"
			wbtConf    = &wbtcommon.WbtConfig{
				Enable: true,
				VolumeOptions: wbtcommon.VolumeOptions{
					Modes:          []string{volumeMode},
					StorageClasses: []string{wrongSC},
				},
			}
		)
		vm := prepareSuccessVolumeManager(t)
		vm.wbtConfig = wbtConf

		pv := &corev1.PersistentVolume{}
		pv.Name = "some_name"
		pv.Spec.StorageClassName = volumeSC
		err := vm.k8sClient.Create(testCtx, pv)
		assert.Nil(t, err)

		result := vm.checkWbtChangingEnable(testCtx, testVol)
		assert.False(t, result)
	})

	// findDeviceName
	t.Run("findDeviceName: success", func(t *testing.T) {
		var (
			testVol        = volCR.DeepCopy()
			testDrive      = testDriveCR.DeepCopy()
			expectedDevice = "sda" //testDrive.Spec.Path = "/dev/sda"
		)
		vm := prepareSuccessVolumeManager(t)

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		device, err := vm.findDeviceName(testVol)
		assert.Nil(t, err)
		assert.Equal(t, expectedDevice, device)
	})
	t.Run("findDeviceName: GetDriveCRByVolume failed", func(t *testing.T) {
		var (
			testVol = volCR.DeepCopy()
		)
		vm := prepareSuccessVolumeManager(t)

		testVol.Spec.LocationType = apiV1.LocationTypeLVM

		device, err := vm.findDeviceName(testVol)
		assert.NotNil(t, err)
		assert.Equal(t, "", device)
	})
	t.Run("findDeviceName: GetDriveCRByVolume failed", func(t *testing.T) {
		var (
			testVol = volCR.DeepCopy()
		)
		vm := prepareSuccessVolumeManager(t)

		device, err := vm.findDeviceName(testVol)
		assert.NotNil(t, err)
		assert.Equal(t, "", device)
	})
	t.Run("findDeviceName: regexp failed", func(t *testing.T) {
		var (
			testVol     = volCR.DeepCopy()
			testDrive   = testDriveCR.DeepCopy()
			wrongDevice = "/dev/"
		)
		vm := prepareSuccessVolumeManager(t)

		testDrive.Spec.Path = wrongDevice

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		device, err := vm.findDeviceName(testVol)
		assert.NotNil(t, err)
		assert.Equal(t, "", device)
	})
	t.Run("findDeviceName: regexp failed", func(t *testing.T) {
		var (
			testVol     = volCR.DeepCopy()
			testDrive   = testDriveCR.DeepCopy()
			wrongDevice = "/nodev/sda"
		)
		vm := prepareSuccessVolumeManager(t)

		testDrive.Spec.Path = wrongDevice

		err := vm.k8sClient.CreateCR(testCtx, testDrive.Name, testDrive)
		assert.Nil(t, err)

		device, err := vm.findDeviceName(testVol)
		assert.NotNil(t, err)
		assert.Equal(t, "", device)
	})

	// SetWbtConfig UT
	t.Run("SetWbtConfig: success", func(t *testing.T) {
		var (
			volumeMode = apiV1.ModeFS
			volumeSC   = apiV1.StorageClassHDD
			wbtConf    = &wbtcommon.WbtConfig{
				Enable: true,
				VolumeOptions: wbtcommon.VolumeOptions{
					Modes:          []string{volumeMode},
					StorageClasses: []string{volumeSC},
				},
			}
		)

		// nil
		vm := prepareSuccessVolumeManager(t)
		vm.SetWbtConfig(wbtConf)
		assert.Equal(t, wbtConf, vm.wbtConfig)

		// changed
		changedWbtConf := &wbtcommon.WbtConfig{
			Enable: true,
			VolumeOptions: wbtcommon.VolumeOptions{
				Modes:          []string{volumeMode},
				StorageClasses: []string{volumeSC},
			},
		}
		vm.SetWbtConfig(changedWbtConf)
		assert.Equal(t, changedWbtConf, vm.wbtConfig)
	})
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
	vm := NewVolumeManager(c, e, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)
	vm.discoverSystemLVG = false
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
	vm := NewVolumeManager(hwMgrClient, nil, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)
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

	bdev1.MountPoint = base.HostRootPath
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

func TestVolumeManager_checkVGErrors(t *testing.T) {
	var (
		blockDeviceWithNoLV = []lsblk.BlockDevice{
			{
				Name:       "/dev/sdz",
				Type:       "disk",
				Size:       lsblk.CustomInt64{Int64: 8001563222016},
				Rota:       lsblk.CustomBool{Bool: true},
				Serial:     "VDH1AAND",
				WWN:        "0x5000cca0bbceb2b4",
				Vendor:     "ATA     ",
				Model:      "LVM PV gVhS8Z-IYdr-nVxF-jVd7-wnfv-ob2d-PZVkHM on /dev/sdz",
				Rev:        "RT04",
				MountPoint: "",
				FSType:     "LVM2_member",
				PartUUID:   "",
				Children:   []lsblk.BlockDevice{},
			},
		}
		blockDeviceWithLVs = []lsblk.BlockDevice{
			{
				Name:       "/dev/sdz",
				Type:       "disk",
				Size:       lsblk.CustomInt64{Int64: 8001563222016},
				Rota:       lsblk.CustomBool{Bool: true},
				Serial:     "VDH1AAND",
				WWN:        "0x5000cca0bbceb2b4",
				Vendor:     "ATA     ",
				Model:      "LVM PV gVhS8Z-IYdr-nVxF-jVd7-wnfv-ob2d-PZVkHM on /dev/sdz",
				Rev:        "RT04",
				MountPoint: "",
				FSType:     "LVM2_member",
				PartUUID:   "",
				Children: []lsblk.BlockDevice{
					{
						Name:       "/dev/mapper/a525d62e--746f--4e3d--ab1f--4931de511b51-pvc--db79c049--7003--4998--b63f--12c200cefc1d",
						Type:       "lvm",
						Size:       lsblk.CustomInt64{Int64: 1002438656},
						Rota:       lsblk.CustomBool{Bool: true},
						Serial:     "",
						WWN:        "",
						Vendor:     "",
						Model:      "",
						Rev:        "",
						MountPoint: "/var/lib/kubelet/pods/0f38c332-f906-4227-ae79-8f91f8aa6970/volumes/kubernetes.io~csi/pvc-db79c049-7003-4998-b63f-12c200cefc1d/mount",
						FSType:     "xfs",
						PartUUID:   "",
						Children:   []lsblk.BlockDevice{},
					},
					{
						Name:       "/dev/mapper/a525d62e--746f--4e3d--ab1f--4931de511b51-pvc--28b3d196--e268--402c--bae6--a00dd01964af",
						Type:       "lvm",
						Size:       lsblk.CustomInt64{Int64: 1002438656},
						Rota:       lsblk.CustomBool{Bool: true},
						Serial:     "",
						WWN:        "",
						Vendor:     "",
						Model:      "",
						Rev:        "",
						MountPoint: "/var/lib/kubelet/pods/0f38c332-f906-4227-ae79-8f91f8aa6970/volumes/kubernetes.io~csi/pvc-db79c049-7003-4998-b63f-12c200cefc1d/mount",
						FSType:     "xfs",
						PartUUID:   "",
						Children:   []lsblk.BlockDevice{},
					},
				},
			},
		}
		lvgName = "lvg1"
		lvg     = &lvgcrd.LogicalVolumeGroup{
			TypeMeta: v1.TypeMeta{
				Kind:       "LVG",
				APIVersion: apiV1.APIV1Version,
			},
			ObjectMeta: v1.ObjectMeta{
				Name: lvgName,
			},
			Spec: api.LogicalVolumeGroup{
				Name: lvgName,
				VolumeRefs: []string{
					"pvc-db79c049-7003-4998-b63f-12c200cefc1d",
					"pvc-28b3d196-e268-402c-bae6-a00dd01964af",
				},
			},
		}
		drivePath = "/dev/sdh"
	)

	t.Run("No errors found", func(t *testing.T) {
		var (
			vm           = prepareSuccessVolumeManager(t)
			lvmMock      = &mocklu.MockWrapLVM{}
			lsblkMock    = &mocklu.MockWrapLsblk{}
			recorderMock = &mocks.NoOpRecorder{}
		)

		vm.lvmOps = lvmMock
		vm.listBlk = lsblkMock
		vm.recorder = recorderMock

		lvmMock.On("VGScan", lvg.GetName()).Return(false, nil).Once()
		lsblkMock.On("GetBlockDevices", drivePath).Return(blockDeviceWithLVs, nil).Once()

		vm.checkVGErrors(lvg, drivePath)

		assert.Equal(t, 2, len(recorderMock.Calls))
		assert.Equal(t, eventing.VolumeGroupScanInvolved, recorderMock.Calls[0].Event)
		assert.Equal(t, eventing.VolumeGroupScanNoErrors, recorderMock.Calls[1].Event)
	})

	t.Run("vgscan failed", func(t *testing.T) {
		var (
			vm           = prepareSuccessVolumeManager(t)
			lvmMock      = &mocklu.MockWrapLVM{}
			lsblkMock    = &mocklu.MockWrapLsblk{}
			recorderMock = &mocks.NoOpRecorder{}
		)

		vm.lvmOps = lvmMock
		vm.listBlk = lsblkMock
		vm.recorder = recorderMock

		lvmMock.On("VGScan", lvg.GetName()).Return(true, errors.New("failed")).Once()

		vm.checkVGErrors(lvg, drivePath)

		assert.Equal(t, 2, len(recorderMock.Calls))
		assert.Equal(t, eventing.VolumeGroupScanInvolved, recorderMock.Calls[0].Event)
		assert.Equal(t, eventing.VolumeGroupScanFailed, recorderMock.Calls[1].Event)
	})

	t.Run("vgscan return io error", func(t *testing.T) {
		var (
			vm           = prepareSuccessVolumeManager(t)
			lvmMock      = &mocklu.MockWrapLVM{}
			lsblkMock    = &mocklu.MockWrapLsblk{}
			recorderMock = &mocks.NoOpRecorder{}
		)

		vm.lvmOps = lvmMock
		vm.listBlk = lsblkMock
		vm.recorder = recorderMock

		lvmMock.On("VGScan", lvg.GetName()).Return(true, nil).Once()

		vm.checkVGErrors(lvg, drivePath)

		assert.Equal(t, 2, len(recorderMock.Calls))
		assert.Equal(t, eventing.VolumeGroupScanInvolved, recorderMock.Calls[0].Event)
		assert.Equal(t, eventing.VolumeGroupScanErrorsFound, recorderMock.Calls[1].Event)
	})

	t.Run("get volumes via lsblk failed", func(t *testing.T) {
		var (
			vm           = prepareSuccessVolumeManager(t)
			lvmMock      = &mocklu.MockWrapLVM{}
			lsblkMock    = &mocklu.MockWrapLsblk{}
			recorderMock = &mocks.NoOpRecorder{}
		)

		vm.lvmOps = lvmMock
		vm.listBlk = lsblkMock
		vm.recorder = recorderMock

		lvmMock.On("VGScan", lvg.GetName()).Return(false, nil).Once()
		lsblkMock.On("GetBlockDevices", drivePath).Return(nil, errors.New("failed")).Once()

		vm.checkVGErrors(lvg, drivePath)

		assert.Equal(t, 2, len(recorderMock.Calls))
		assert.Equal(t, eventing.VolumeGroupScanInvolved, recorderMock.Calls[0].Event)
		assert.Equal(t, eventing.VolumeGroupScanFailed, recorderMock.Calls[1].Event)
	})

	t.Run("volumes not found", func(t *testing.T) {
		var (
			vm           = prepareSuccessVolumeManager(t)
			lvmMock      = &mocklu.MockWrapLVM{}
			lsblkMock    = &mocklu.MockWrapLsblk{}
			recorderMock = &mocks.NoOpRecorder{}
		)

		vm.lvmOps = lvmMock
		vm.listBlk = lsblkMock
		vm.recorder = recorderMock

		lvmMock.On("VGScan", lvg.GetName()).Return(false, nil).Once()
		lsblkMock.On("GetBlockDevices", drivePath).Return(blockDeviceWithNoLV, nil).Once()

		vm.checkVGErrors(lvg, drivePath)

		assert.Equal(t, 2, len(recorderMock.Calls))
		assert.Equal(t, eventing.VolumeGroupScanInvolved, recorderMock.Calls[0].Event)
		assert.Equal(t, eventing.VolumeGroupScanErrorsFound, recorderMock.Calls[1].Event)
	})
}

func TestVolumeManager_reactivateVG(t *testing.T) {
	var (
		lvgName = "lvg1"
		lvg     = &lvgcrd.LogicalVolumeGroup{
			TypeMeta: v1.TypeMeta{
				Kind:       "LVG",
				APIVersion: apiV1.APIV1Version,
			},
			ObjectMeta: v1.ObjectMeta{
				Name: lvgName,
			},
			Spec: api.LogicalVolumeGroup{
				Name: lvgName,
				VolumeRefs: []string{
					"pvc--db79c049--7003--4998--b63f--12c200cefc1d",
					"pvc--28b3d196--e268--402c--bae6--a00dd01964af",
				},
			},
		}
	)

	t.Run("success", func(t *testing.T) {
		var (
			vm           = prepareSuccessVolumeManager(t)
			lvmMock      = &mocklu.MockWrapLVM{}
			recorderMock = &mocks.NoOpRecorder{}
		)

		vm.lvmOps = lvmMock
		vm.recorder = recorderMock

		lvmMock.On("VGReactivate", lvg.GetName()).Return(nil).Once()

		vm.reactivateVG(lvg)

		assert.Equal(t, 1, len(recorderMock.Calls))
		assert.Equal(t, eventing.VolumeGroupReactivateInvolved, recorderMock.Calls[0].Event)
	})

	t.Run("failed", func(t *testing.T) {
		var (
			vm           = prepareSuccessVolumeManager(t)
			lvmMock      = &mocklu.MockWrapLVM{}
			recorderMock = &mocks.NoOpRecorder{}
		)

		vm.lvmOps = lvmMock
		vm.recorder = recorderMock

		lvmMock.On("VGReactivate", lvg.GetName()).Return(errors.New("failed")).Once()

		vm.reactivateVG(lvg)

		assert.Equal(t, 2, len(recorderMock.Calls))
		assert.Equal(t, eventing.VolumeGroupReactivateInvolved, recorderMock.Calls[0].Event)
		assert.Equal(t, eventing.VolumeGroupReactivateFailed, recorderMock.Calls[1].Event)
	})
}

func TestVolumeManager_failedToRemoveFinalizer(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)
	volumeCR := volCR.DeepCopy()
	volumeCR.ObjectMeta.Finalizers = append(volumeCR.ObjectMeta.Finalizers, volumeFinalizer)
	// not found error
	_, err = vm.removeFinalizer(context.TODO(), volumeCR)
	assert.NotNil(t, err)
}