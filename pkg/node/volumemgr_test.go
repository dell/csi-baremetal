package node

import (
	"context"
	"errors"
	"fmt"
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

// todo refactor these UTs - https://jira.cec.lab.emc.com:8443/browse/AK8S-724

var (
	testErr            = errors.New("error")
	lsblkAllDevicesCmd = fmt.Sprintf(lsblk.CmdTmpl, "")

	drive1UUID = uuid.New().String()
	drive2UUID = uuid.New().String()

	drive1 = &api.Drive{
		UUID:         drive1UUID,
		SerialNumber: "hdd1",
		Size:         1024 * 1024 * 1024 * 500,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthGood,
	} // /dev/sda in LsblkTwoDevices

	drive2 = &api.Drive{
		UUID:         drive2UUID,
		SerialNumber: "hdd2",
		Size:         1024 * 1024 * 1024 * 200,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthGood,
	} // /dev/sdb in LsblkTwoDevices

	driveMgrRespDrives = []*api.Drive{drive1, drive2}

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
	vm = GetVolumeManagerForTest(t)
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
	vm = GetVolumeManagerForTest(t)
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
	vm = GetVolumeManagerForTest(t)
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
	vm = GetVolumeManagerForTest(t)
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

	volCR.Spec.CSIStatus = apiV1.Created
	err = vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR)
	assert.Nil(t, err)

	//successfully add finalizer
	res, err = vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	volCR.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	err = vm.k8sClient.UpdateCR(testCtx, &volCR)
	assert.Nil(t, err)

	//successfully release volume
	res, err = vm.Reconcile(req)
	assert.NotNil(t, k8sError.IsNotFound(err))
	assert.Equal(t, res, ctrl.Result{})
}

func TestReconcile_FailedToCreateAndRemoveVolume(t *testing.T) {
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
	vm = GetVolumeManagerForTest(t)

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.NotNil(t, err)
	assert.True(t, k8sError.IsNotFound(err))
	assert.Equal(t, expectedResRequeue, res)

	// LVG is not found, volume CR was updated successfully (CSIStatus=failed)
	vm = GetVolumeManagerForTest(t)
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, &testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, vol))
	assert.Equal(t, apiV1.Failed, vol.Spec.CSIStatus)

	volCR.Spec.CSIStatus = apiV1.Creating
	err = vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR)
	// LVG in creating state
	vm = GetVolumeManagerForTest(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Creating
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, expectedResRequeue, res)

	// LVG in failed state and volume is updated successfully
	vm = GetVolumeManagerForTest(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Failed
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testVol.Name, &testVol))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.Failed, volume.Spec.CSIStatus)
	assert.Equal(t, ctrl.Result{}, res)

	vol = &vcrd.Volume{}
	assert.Nil(t, vm.k8sClient.ReadCR(testCtx, testVol.Name, vol))
	assert.Equal(t, apiV1.Failed, vol.Spec.CSIStatus)

	// LVG in failed state and volume is failed to update
	vm = GetVolumeManagerForTest(t)
	testLVG = testLVGCR
	testLVG.Spec.Status = apiV1.Failed
	testVol = testVolumeLVGCR
	assert.Nil(t, vm.k8sClient.CreateCR(testCtx, testLVG.Name, &testLVG))

	res, err = vm.handleCreatingVolumeInLVG(testCtx, &testVol)
	assert.NotNil(t, err)
	assert.Equal(t, expectedResRequeue, res)
	assert.True(t, k8sError.IsNotFound(err))

	// LVG in created state and volume.ID is not in VolumeRefs
	vm = GetVolumeManagerForTest(t)
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
	vm = GetVolumeManagerForTest(t)
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
	vm = GetVolumeManagerForTest(t)
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

	vm = GetVolumeManagerForTest(t)
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

func TestVolumeManager_DrivesNotInUse(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	driveCR1 := vm.k8sClient.ConstructDriveCR(
		"hdd1", api.Drive{UUID: "hdd1", SerialNumber: "hdd1", Type: apiV1.DriveTypeHDD, NodeId: nodeID})
	driveCR2 := vm.k8sClient.ConstructDriveCR(
		"nvme1", api.Drive{UUID: "nvme1", SerialNumber: "nvme1", Type: apiV1.DriveTypeNVMe, NodeId: nodeID})
	addDriveCRs(kubeClient, driveCR1, driveCR2)

	drivesNotInUse := vm.drivesAreNotUsed()
	// empty volumes cache, method should return all drives
	assert.NotNil(t, drivesNotInUse)
	assert.Equal(t, 2, len(drivesNotInUse))

	volumeCR := kubeClient.ConstructVolumeCR("test_name", api.Volume{
		NodeId:       nodeID,
		LocationType: apiV1.LocationTypeDrive,
		Location:     "hdd1",
	})
	err = kubeClient.CreateCR(testCtx, volumeCR.Name, volumeCR)
	assert.Nil(t, err)

	// expect that nvme drive is not used
	drivesNotInUse = vm.drivesAreNotUsed()
	assert.Equal(t, 1, len(drivesNotInUse))
	assert.Equal(t, "nvme1", drivesNotInUse[0].Spec.UUID)
}

func TestVolumeManager_DiscoverFail(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	// expect: hwMgrClient request fail with error
	vm.driveMgrClient = mocks.MockDriveMgrClientFail{}
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "drivemgr error", err.Error())

	// expect: lsblk fail with error
	vm = NewVolumeManager(mocks.MockDriveMgrClient{}, mocks.EmptyExecutorFail{}, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	vm.listBlk.SetExecutor(mocks.EmptyExecutorFail{})
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error")

}

func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	hwMgrClient := mocks.NewMockDriveMgrClient(driveMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	vm.listBlk.SetExecutor(e1)
	// expect that cache is empty because of all drives has not children
	err = vm.Discover()
	assert.Nil(t, err)
	assertLenVListItemsEqualsTo(t, vm.k8sClient, 0)

	// expect that one volume will appear in cache
	expectedCmdOut2 := map[string]mocks.CmdOut{
		lsblkAllDevicesCmd:         mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb --info=1": {Stdout: "Partition unique GUID: uniq-guid-for-dev-sdb"},
	}
	e3 := mocks.NewMockExecutor(expectedCmdOut2)
	vm = NewVolumeManager(*hwMgrClient, e3, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	vm.listBlk.SetExecutor(e3)
	err = vm.Discover()
	assert.Nil(t, err)
	// LsblkDevWithChildren contains 2 devices with children however one of them without size
	// that because we expect one item in volumes cache
	vItems := getVolumeCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 1, len(vItems))
}

func TestVolumeManager_DiscoverAvailableCapacitySuccess(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	hwMgrClient := mocks.NewMockDriveMgrClient(driveMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	err = vm.Discover()
	assert.Nil(t, err)

	err = vm.discoverAvailableCapacity(context.Background(), nodeID)
	assert.Nil(t, err)

	acList := &accrd.AvailableCapacityList{}
	err = vm.k8sClient.ReadList(context.Background(), acList)

	assert.Nil(t, err)
	assert.Equal(t, 2, len(acList.Items))
}

func TestVolumeManager_DiscoverAvailableCapacityDriveUnhealthy(t *testing.T) {
	//expected 1 available capacity because 1 drive is unhealthy
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	hwMgrDrivesWithBad := driveMgrRespDrives
	hwMgrDrivesWithBad[1].Health = apiV1.HealthBad
	hwMgrClient := mocks.NewMockDriveMgrClient(hwMgrDrivesWithBad)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	err = vm.Discover()
	assert.Nil(t, err)

	err = vm.discoverAvailableCapacity(context.Background(), nodeID)
	assert.Nil(t, err)

	acList := &accrd.AvailableCapacityList{}
	err = vm.k8sClient.ReadList(context.Background(), acList)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items))
}

func TestVolumeManager_updatesDrivesCRs(t *testing.T) {
	hwMgrClient := mocks.NewMockDriveMgrClient(driveMgrRespDrives)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(hwMgrClient, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	assert.Nil(t, err)
	assert.Empty(t, vm.crHelper.GetDriveCRs(vm.nodeID))
	ctx := context.Background()
	updates := vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 2)
	assert.Len(t, updates.Created, 2)

	driveMgrRespDrives[0].Health = apiV1.HealthBad
	updates = vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthBad)
	assert.Len(t, updates.Updated, 1)
	assert.Len(t, updates.NotChanged, 1)

	drives := driveMgrRespDrives[1:]
	updates = vm.updateDrivesCRs(ctx, drives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthUnknown)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Status, apiV1.DriveStatusOffline)
	assert.Len(t, updates.Updated, 1)
	assert.Len(t, updates.NotChanged, 1)

	kubeClient, err = k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm = NewVolumeManager(hwMgrClient, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	assert.Nil(t, err)
	assert.Empty(t, vm.crHelper.GetDriveCRs(vm.nodeID))
	updates = vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 2)
	assert.Len(t, updates.Created, 2)
	driveMgrRespDrives = append(driveMgrRespDrives, &api.Drive{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd3",
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024 * 1024 * 150,
		NodeId:       nodeID,
	})
	updates = vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 3)
	assert.Len(t, updates.Created, 1)
	assert.Len(t, updates.NotChanged, 2)
}

func TestVolumeManager_handleDriveStatusChange(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives(nil)

	ac := acCR
	err := vm.k8sClient.CreateCR(context.Background(), ac.Name, &ac)
	assert.Nil(t, err)

	drive := drive1
	drive.UUID = driveUUID
	drive.Health = apiV1.HealthBad

	// Check AC deletion
	vm.handleDriveStatusChange(context.Background(), drive)
	acList := &accrd.AvailableCapacityList{}
	err = vm.k8sClient.ReadList(testCtx, acList)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))

	vol := volCR
	vol.Spec.Location = driveUUID
	err = vm.k8sClient.CreateCR(context.Background(), testID, &vol)
	assert.Nil(t, err)

	// Check volume's health change
	vm.handleDriveStatusChange(context.Background(), drive)
	rVolume := &vcrd.Volume{}
	err = vm.k8sClient.ReadCR(context.Background(), testID, rVolume)
	assert.Nil(t, err)
	assert.Equal(t, apiV1.HealthBad, rVolume.Spec.Health)
}

func Test_discoverLVGOnSystemDrive_LVGAlreadyExists(t *testing.T) {
	var (
		m     = prepareSuccessVolumeManager()
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
		m       = prepareSuccessVolumeManager()
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
	lvmOps.On("GetLVsInVG", vgName).Return([]string{"lv_swap", "lv_boot"})

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

	err = m.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))
}

func prepareSuccessVolumeManager() *VolumeManager {
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
	if err != nil {
		panic(err)
	}
	return NewVolumeManager(c, e, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
}

func prepareSuccessVolumeManagerWithDrives(drives []*api.Drive) *VolumeManager {
	nVM := prepareSuccessVolumeManager()
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

	vm = GetVolumeManagerForTest(t)
	vol = testVolumeCR1
	vol.Spec.NodeId = vm.nodeID
	assert.True(t, vm.isCorrespondedToNodePredicate(&vol))

	vol.Spec.NodeId = ""
	assert.False(t, vm.isCorrespondedToNodePredicate(&vol))

}

func GetVolumeManagerForTest(t *testing.T) *VolumeManager {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	return NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
}
