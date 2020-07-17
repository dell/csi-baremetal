package node

import (
	"context"
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
	"github.com/dell/csi-baremetal/pkg/mocks"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
	mockProv "github.com/dell/csi-baremetal/pkg/mocks/provisioners"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
)

// todo refactor these UTs - https://jira.cec.lab.emc.com:8443/browse/AK8S-724

var (
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

	lvgCR = lvgcrd.LVG{
		TypeMeta: v1.TypeMeta{
			Kind:       "LVG",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      lvgName,
			Namespace: testNs,
		},
		Spec: api.LogicalVolumeGroup{
			Name:      lvgName,
			Node:      nodeID,
			Locations: []string{drive1.UUID},
			Size:      int64(1024 * 500 * util.GBYTE),
			Status:    apiV1.Created,
		},
	}

	volCRLVG = vcrd.Volume{
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
			Location:     lvgCR.Name,
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

func TestReconcile_SuccessCreatingAndRemovingLVGVolume(t *testing.T) {
	var (
		req    = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCRLVG.Name}}
		lvg    = &lvgcrd.LVG{}
		volume = &vcrd.Volume{}
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	err = vm.k8sClient.CreateCR(testCtx, volCRLVG.Name, &volCRLVG)
	assert.Nil(t, err)
	err = vm.k8sClient.CreateCR(testCtx, lvgCR.Name, &lvgCR)
	assert.Nil(t, err)

	pMock := mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.LVMBasedVolumeType: pMock})

	res, err := vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, lvgCR.Name, lvg)
	assert.Nil(t, err)
	assert.Equal(t, len(lvg.Spec.VolumeRefs), 1)
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Created)

	volume.Spec.CSIStatus = apiV1.Removing
	err = vm.k8sClient.UpdateCR(testCtx, volume)
	assert.Nil(t, err)
	// reconciled second time
	res, err = vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	err = vm.k8sClient.ReadCR(testCtx, lvgCR.Name, lvg)
	assert.True(t, k8sError.IsNotFound(err))
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Removed)
}

func TestReconcile_SuccessCreatingAndRemovingDriveVolume(t *testing.T) {
	var (
		req    = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
		volume = &vcrd.Volume{}
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	err = vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR)
	assert.Nil(t, err)

	pMock := mockProv.GetMockProvisionerSuccess("/some/path")
	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})

	res, err := vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Created)

	volume.Spec.CSIStatus = apiV1.Removing
	err = vm.k8sClient.UpdateCR(testCtx, volume)
	assert.Nil(t, err)
	// reconciled second time
	res, err = vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Removed)
}

func TestReconcile_FailedToCreateAndRemoveVolume(t *testing.T) {
	var (
		req    = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
		volume = &vcrd.Volume{}
	)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	err = vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR)
	assert.Nil(t, err)

	pMock := &mockProv.MockProvisioner{}
	pMock.On("PrepareVolume", mock.Anything).Return(fmt.Errorf("error"))
	pMock.On("ReleaseVolume", mock.Anything).Return(fmt.Errorf("error"))

	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})
	res, err := vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Failed)

	volume.Spec.CSIStatus = apiV1.Removing
	err = vm.k8sClient.UpdateCR(testCtx, volume)
	assert.Nil(t, err)
	// reconciled second time
	res, err = vm.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	err = vm.k8sClient.ReadCR(testCtx, req.Name, volume)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.CSIStatus, apiV1.Failed)
}

func TestReconcile_ReconcileDefaultStatus(t *testing.T) {
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: volCR.Name}}
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	volCR.Spec.CSIStatus = apiV1.Failed
	err = vm.k8sClient.CreateCR(testCtx, volCR.Name, &volCR)
	assert.Nil(t, err)

	pMock := mockProv.GetMockProvisionerSuccess("/some/path")

	vm.SetProvisioners(map[p.VolumeType]p.Provisioner{p.DriveBasedVolumeType: pMock})
	res, err := vm.Reconcile(req)
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

	// expect that volume cache will be empty because one drive without size
	// and GetPartitionGUID returns error for second drive
	expectedCmdOut1 := map[string]mocks.CmdOut{
		lsblkAllDevicesCmd:     mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb -i 1": {Stdout: "some output: here"},
	}
	e2 := mocks.NewMockExecutor(expectedCmdOut1)
	vm = NewVolumeManager(*hwMgrClient, e2, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	vm.listBlk.SetExecutor(e2)
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
	partOps := &mocklu.MockWrapPartition{}
	partOps.On("GetPartitionUUID", mock.Anything, mock.Anything).
		Return("uniq-guid-for-dev-sdb", nil)
	vm.partOps = partOps
	err = vm.Discover()
	assert.Nil(t, err)
	// LsblkDevWithChildren contains 2 devices with children however one of them without size
	// that because we expect one item in volumes cache
	vItems := getVolumeCRsListItems(t, vm.k8sClient)
	assert.Equal(t, 1, len(vItems))
	assert.Equal(t, "uniq-guid-for-dev-sdb", vItems[0].Spec.Id)
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

func TestVolumeManager_DiscoverAvailableCapacityNoFreeDrive(t *testing.T) {
	//expected 0 available capacity because the drive has volume
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	hwMgrClient := mocks.NewMockDriveMgrClient(nil)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	driveCR1 := vm.k8sClient.ConstructDriveCR("hasVolume", *driveMgrRespDrives[0])
	addDriveCRs(kubeClient, driveCR1)

	volumeCR := vm.k8sClient.ConstructVolumeCR("id", api.Volume{
		Id:           "id",
		NodeId:       nodeID,
		Size:         1000,
		Location:     driveMgrRespDrives[0].UUID,
		LocationType: apiV1.LocationTypeDrive,
		Mode:         apiV1.ModeFS,
		Type:         "xfs",
		Health:       driveMgrRespDrives[0].Health,
		CSIStatus:    "",
	})
	addVolumeCRs(vm.k8sClient, *volumeCR)

	err = vm.discoverAvailableCapacity(context.Background(), nodeID)
	assert.Nil(t, err)

	acList := &accrd.AvailableCapacityList{}
	err = vm.k8sClient.ReadList(context.Background(), acList)

	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))
}

func TestVolumeManager_DiscoverAvailableCapacityIgnoreLVG(t *testing.T) {
	// expected 0 available capacity because the drive is used in LVG
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	hwMgrClient := mocks.NewMockDriveMgrClient(nil)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)

	apiDrive := *driveMgrRespDrives[0]
	driveCR1 := vm.k8sClient.ConstructDriveCR(apiDrive.UUID, apiDrive)
	addDriveCRs(kubeClient, driveCR1)

	err = vm.k8sClient.CreateCR(context.Background(), lvgName, &lvgCR)
	assert.Nil(t, err)

	err = vm.discoverAvailableCapacity(context.Background(), nodeID)
	assert.Nil(t, err)

	acList := &accrd.AvailableCapacityList{}
	err = vm.k8sClient.ReadList(context.Background(), acList)

	testLogger.Infof("ACLIST: %v\n", acList.Items)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))
}

func TestVolumeManager_updatesDrivesCRs(t *testing.T) {
	hwMgrClient := mocks.NewMockDriveMgrClient(driveMgrRespDrives)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(hwMgrClient, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	assert.Nil(t, err)
	assert.Empty(t, vm.crHelper.GetDriveCRs(vm.nodeID))
	ctx := context.Background()
	vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 2)

	driveMgrRespDrives[0].Health = apiV1.HealthBad
	vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthBad)

	drives := driveMgrRespDrives[1:]
	vm.updateDrivesCRs(ctx, drives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthUnknown)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(driveMgrRespDrives[0].UUID).Spec.Status, apiV1.DriveStatusOffline)

	kubeClient, err = k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm = NewVolumeManager(hwMgrClient, nil, testLogger, kubeClient, new(mocks.NoOpRecorder), nodeID)
	assert.Nil(t, err)
	assert.Empty(t, vm.crHelper.GetDriveCRs(vm.nodeID))
	vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 2)
	driveMgrRespDrives = append(driveMgrRespDrives, &api.Drive{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd3",
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024 * 1024 * 150,
		NodeId:       nodeID,
	})
	vm.updateDrivesCRs(ctx, driveMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 3)
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
