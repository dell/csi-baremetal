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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	crdV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
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

	hwMgrRespDrives = []*api.Drive{drive1, drive2}

	// todo don't hardcode device name
	lsblkSingleDeviceCmd = fmt.Sprintf(lsblk.CmdTmpl, "/dev/sda")

	volCR = vcrd.Volume{
		TypeMeta: v1.TypeMeta{Kind: "Volume", APIVersion: crdV1.APIV1Version},
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
			CSIStatus:    crdV1.Creating,
			NodeId:       nodeID,
			Mode:         apiV1.ModeFS,
			Type:         string(sc.XFS),
		},
	}

	lvgCR = lvgcrd.LVG{
		TypeMeta: v1.TypeMeta{
			Kind:       "LVG",
			APIVersion: crdV1.APIV1Version,
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
			Status:    crdV1.Created,
		},
	}

	volCRLVG = vcrd.Volume{
		TypeMeta: v1.TypeMeta{Kind: "Volume", APIVersion: crdV1.APIV1Version},
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
			CSIStatus:    crdV1.Creating,
			NodeId:       nodeID,
			Mode:         apiV1.ModeFS,
			Type:         string(sc.XFS),
		},
	}

	acCR = accrd.AvailableCapacity{
		TypeMeta:   v1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
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
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, nodeID)
	assert.NotNil(t, vm)
	assert.Nil(t, vm.hWMgrClient)
	assert.NotNil(t, vm.linuxUtils)
}

func TestNewVolumeManager_SetExecutor(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, mocks.EmptyExecutorSuccess{}, logrus.New(), kubeClient, nodeID)
	vm.SetExecutor(mocks.EmptyExecutorFail{})
	res, err := vm.linuxUtils.GetBlockDevices("")
	assert.Nil(t, res)
	assert.NotNil(t, err)
}

func TestVolumeManager_SetLinuxUtilsExecutor(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(nil, e, testLogger, kubeClient, nodeID)

	out, err := vm.linuxUtils.GetBlockDevices("")
	assert.NotNil(t, out)
	if out != nil {
		assert.Equal(t, len(out), 2)
	}
	assert.Nil(t, err)
}

func TestVolumeManager_DrivesNotInUse(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, nodeID)

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
	vm := NewVolumeManager(nil, nil, testLogger, kubeClient, nodeID)

	// expect: hwMgrClient request fail with error
	vm.hWMgrClient = mocks.MockHWMgrClientFail{}
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "hwmgr error", err.Error())

	// expect: lsblk fail with error
	vm = NewVolumeManager(mocks.MockHWMgrClient{}, mocks.EmptyExecutorFail{}, testLogger, kubeClient, nodeID)
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error")

}
func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, nodeID)

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
	vm = NewVolumeManager(*hwMgrClient, e2, testLogger, kubeClient, nodeID)
	err = vm.Discover()
	assert.Nil(t, err)
	assertLenVListItemsEqualsTo(t, vm.k8sClient, 0)

	// expect that one volume will appear in cache
	expectedCmdOut2 := map[string]mocks.CmdOut{
		lsblkAllDevicesCmd:         mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb --info=1": {Stdout: "Partition unique GUID: uniq-guid-for-dev-sdb"},
	}
	e3 := mocks.NewMockExecutor(expectedCmdOut2)
	vm = NewVolumeManager(*hwMgrClient, e3, testLogger, kubeClient, nodeID)
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
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, nodeID)

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
	hwMgrDrivesWithBad := hwMgrRespDrives
	hwMgrDrivesWithBad[1].Health = apiV1.HealthBad
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrDrivesWithBad)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, nodeID)

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
	hwMgrClient := mocks.NewMockHWMgrClient(nil)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, nodeID)

	driveCR1 := vm.k8sClient.ConstructDriveCR("hasVolume", *hwMgrRespDrives[0])
	addDriveCRs(kubeClient, driveCR1)

	volumeCR := vm.k8sClient.ConstructVolumeCR("id", api.Volume{
		Id:           "id",
		NodeId:       nodeID,
		Size:         1000,
		Location:     hwMgrRespDrives[0].UUID,
		LocationType: apiV1.LocationTypeDrive,
		Mode:         apiV1.ModeFS,
		Type:         "xfs",
		Health:       hwMgrRespDrives[0].Health,
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
	hwMgrClient := mocks.NewMockHWMgrClient(nil)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, testLogger, kubeClient, nodeID)

	apiDrive := *hwMgrRespDrives[0]
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
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm := NewVolumeManager(hwMgrClient, nil, testLogger, kubeClient, nodeID)
	assert.Nil(t, err)
	assert.Empty(t, vm.crHelper.GetDriveCRs(vm.nodeID))
	ctx := context.Background()
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 2)

	hwMgrRespDrives[0].Health = apiV1.HealthBad
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(hwMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthBad)

	drives := hwMgrRespDrives[1:]
	vm.updateDrivesCRs(ctx, drives)
	assert.Nil(t, err)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(hwMgrRespDrives[0].UUID).Spec.Health, apiV1.HealthUnknown)
	assert.Equal(t, vm.crHelper.GetDriveCRByUUID(hwMgrRespDrives[0].UUID).Spec.Status, apiV1.DriveStatusOffline)

	kubeClient, err = k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm = NewVolumeManager(hwMgrClient, nil, testLogger, kubeClient, nodeID)
	assert.Nil(t, err)
	assert.Empty(t, vm.crHelper.GetDriveCRs(vm.nodeID))
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 2)
	hwMgrRespDrives = append(hwMgrRespDrives, &api.Drive{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd3",
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024 * 1024 * 150,
		NodeId:       nodeID,
	})
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.crHelper.GetDriveCRs(vm.nodeID)), 3)
}

func TestVolumeManager_createPartitionAndSetUUIDSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManager()

	// todo refactor these UTs - https://jira.cec.lab.emc.com:8443/browse/AK8S-724
	// get rid of hardcoded values
	partName, _, err := vm.createPartitionAndSetUUID("/dev/sda", testID, false)

	assert.Equal(t, partName, "/dev/sda1")
	assert.Nil(t, err)
}

func TestVolumeManager_createPartitionAndSetUUIDFail(t *testing.T) {
	var (
		vm         *VolumeManager
		partName   string
		err        error
		rollBacked bool
		dev        = "/dev/sda"
		uuid       = "uuid-sda"
		cmdRes     mocks.CmdOut
		emptyCmdOk = mocks.CmdOut{Stdout: "", Stderr: "", Err: nil}
	)

	partExistCMD := "partprobe -d -s /dev/sda"
	lifecycleCMD := map[string]mocks.CmdOut{
		partExistCMD:         {"/dev/sda: gpt", "", nil},
		"partprobe /dev/sda": mocks.EmptyOutSuccess,
	}

	// unable check whether partition exist
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)
	vm = NewVolumeManager(nil, mocks.EmptyExecutorFail{}, testLogger, kubeClient, nodeID)
	partName, rollBacked, err = vm.createPartitionAndSetUUID(dev, uuid, false)
	assert.Empty(t, partName)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)

	// partition has already exist
	cmdRes = mocks.CmdOut{Stdout: "/dev/sda: gpt partitions 1 "}
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{
		"partprobe -d -s /dev/sda": cmdRes,
		"sgdisk /dev/sda --info=1": {Stdout: fmt.Sprintf("Partition unique GUID: %s", uuid)}})
	vm = NewVolumeManager(nil, e, testLogger, kubeClient, nodeID)
	partName, rollBacked, err = vm.createPartitionAndSetUUID(dev, uuid, false)
	assert.Empty(t, partName)
	assert.True(t, rollBacked)
	assert.Nil(t, err)
	//assert.Contains(t, err.Error(), "partition has already exist")

	// create partition table failed
	createPTCMD := "parted -s /dev/sda mklabel gpt"
	lifecycleCMD[createPTCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("create partition table failed")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	vm = NewVolumeManager(nil, e, testLogger, kubeClient, nodeID)
	partName, rollBacked, err = vm.createPartitionAndSetUUID(dev, uuid, false)
	assert.Empty(t, partName)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to create partition table for device")

	// create partition failed and partition doesn't exist
	createPartCMD := "parted -s /dev/sda mkpart --align optimal CSI 0% 100%"
	lifecycleCMD[partExistCMD] = emptyCmdOk
	lifecycleCMD[createPTCMD] = emptyCmdOk
	lifecycleCMD[createPartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("create partition failed")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	vm = NewVolumeManager(nil, e, testLogger, kubeClient, nodeID)
	partName, rollBacked, err = vm.createPartitionAndSetUUID(dev, uuid, false)
	assert.Empty(t, partName)
	assert.True(t, rollBacked)
	assert.Equal(t, errors.New("create partition failed"), err)

	// create partition failed and partition exist and delete partition failed
	deletePartCMD := "parted -s /dev/sda rm 1"
	lifecycleCMD[deletePartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("error")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	// second time show that partition exist
	e.AddSecondRun(partExistCMD, mocks.CmdOut{Stdout: "/dev/sda: gpt partitions 1"})
	vm = NewVolumeManager(nil, e, testLogger, kubeClient, nodeID)
	partName, rollBacked, err = vm.createPartitionAndSetUUID(dev, uuid, false)
	assert.Empty(t, partName)
	assert.False(t, rollBacked)
	assert.NotNil(t, err)
	assert.Equal(t, errors.New("create partition failed"), err)

	// setPartitionUUID failed and delete part CMD failed too
	setPartCMD := "sgdisk /dev/sda --partition-guid=1:uuid-sda"
	partprobeCMD := "partprobe"
	lifecycleCMD[partprobeCMD] = emptyCmdOk
	lifecycleCMD[createPartCMD] = emptyCmdOk
	lifecycleCMD[setPartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("set partition UUID failed")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	e.AddSecondRun(partExistCMD, mocks.CmdOut{Stdout: "/dev/sda: gpt partitions 1"})
	vm = NewVolumeManager(nil, e, testLogger, kubeClient, nodeID)
	partName, rollBacked, err = vm.createPartitionAndSetUUID(dev, uuid, false)
	assert.Empty(t, partName)
	assert.False(t, false)
	assert.NotNil(t, err)
	assert.Equal(t, errors.New("set partition UUID failed"), err)

	// setPartitionUUID failed and delete part CMD pass
	lifecycleCMD[partprobeCMD] = emptyCmdOk
	lifecycleCMD[createPartCMD] = emptyCmdOk
	lifecycleCMD[setPartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("set partition UUID failed")}
	lifecycleCMD[deletePartCMD] = emptyCmdOk
	e = mocks.NewMockExecutor(lifecycleCMD)
	vm = NewVolumeManager(nil, e, testLogger, kubeClient, nodeID)
	partName, rollBacked, err = vm.createPartitionAndSetUUID(dev, uuid, false)
	assert.Empty(t, partName)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)
	assert.Equal(t, errors.New("set partition UUID failed"), err)
}

func TestVolumeManager_CreateLocalVolumeHDDSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	vol := volCR.Spec
	// read Drives
	dList := &drivecrd.DriveList{}
	err := vm.k8sClient.ReadList(context.Background(), dList)
	assert.Nil(t, err)
	assert.True(t, len(dList.Items) > 0)
	vol.Location = dList.Items[0].Spec.UUID
	scMock := &sc.ImplementerMock{}
	scMock.On("CreateFileSystem", sc.FileSystem(vol.Type), "/dev/sda1").Return(nil).Times(1)
	vm.scMap["hdd"] = scMock
	err = vm.CreateLocalVolume(context.Background(), &vol)
	assert.Nil(t, err)
}

func TestVolumeManager_CreateLocalVolumeLVGSuccess(t *testing.T) {
	var (
		vm       = prepareSuccessVolumeManager()
		e        = &mocks.GoMockExecutor{}
		volume   = volCRLVG.Spec
		sizeStr  = fmt.Sprintf("%.2fG", float64(volume.Size)/float64(util.GBYTE))
		fullPath = fmt.Sprintf("/dev/%s/%s", volume.Location, volume.Id)
		hddlvgSC = sc.GetSSDSCInstance(testLogger)
	)

	vm.linuxUtils = linuxutils.NewLinuxUtils(e, testLogger)
	hddlvgSC.SetSDDSCExecutor(e)
	vm.scMap = map[SCName]sc.StorageClassImplementer{
		"hdd": hddlvgSC,
	}

	e.OnCommand(fmt.Sprintf("/sbin/lvm lvcreate --yes --name %s --size %s %s", volume.Id, sizeStr, volume.Location)).
		Return("", "", nil)
	e.OnCommand(fmt.Sprintf(sc.FileSystemExistsTmpl, fullPath)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(sc.MkFSCmdTmpl, sc.XFS, fullPath)).Return("", "", nil)

	err := vm.CreateLocalVolume(testCtx, &volume)
	assert.Nil(t, err)
}

func TestVolumeManager_CreateLocalVolumeHDDFail(t *testing.T) {
	// expect: createPartitionAndSetUUID fail but partition hadn't created (rollback is no needed)
	vm3 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1})
	dList := &drivecrd.DriveList{}
	err := vm3.k8sClient.ReadList(context.Background(), dList)
	assert.Nil(t, err)
	assert.True(t, len(dList.Items) > 0)

	e3 := mocks.NewMockExecutor(map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr},
		fmt.Sprintf("partprobe -d -s /dev/sda"): {Err: errors.New("partprobe -d -s /dev/sda failed")}})
	e3.SetSuccessIfNotFound(false)
	vm3.linuxUtils = linuxutils.NewLinuxUtils(e3, testLogger)
	err = vm3.Discover()
	assert.Nil(t, err)
	vol3 := &api.Volume{
		Id:           testID,
		Size:         1024 * 1024 * 1024 * 45,
		StorageClass: apiV1.StorageClassHDD,
		Location:     dList.Items[0].Spec.UUID,
	}
	err3 := vm3.CreateLocalVolume(context.Background(), vol3)
	assert.NotNil(t, err3)
	assert.Contains(t, err3.Error(), "failed to set partition UUID")

	// expect: createPartitionAndSetUUID fail partition was created and rollback was failed too
	vm4 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1})
	dList2 := &drivecrd.DriveList{}
	err = vm4.k8sClient.ReadList(context.Background(), dList2)
	assert.Nil(t, err)
	assert.True(t, len(dList2.Items) > 0)
	vID := "uuid-4444"
	setUUIDCMD := fmt.Sprintf("sgdisk /dev/sda --partition-guid=1:%s", vID) // /dev/sda - drive1
	deletePartCMD := "parted -s /dev/sda rm 1"
	eMap := map[string]mocks.CmdOut{lsblkAllDevicesCmd: {Stdout: mocks.LsblkTwoDevicesStr},
		setUUIDCMD:    mocks.EmptyOutFail,
		deletePartCMD: mocks.EmptyOutFail}
	e4 := mocks.NewMockExecutor(eMap)
	e4.SetSuccessIfNotFound(true)
	vm4.linuxUtils = linuxutils.NewLinuxUtils(e4, testLogger)

	vol4 := &api.Volume{
		Id:           vID,
		Size:         1024 * 1024 * 1024 * 45,
		StorageClass: apiV1.StorageClassHDD,
		Location:     dList2.Items[0].Spec.UUID,
	}
	err4 := vm4.CreateLocalVolume(context.Background(), vol4)
	assert.NotNil(t, err4)
	assert.Contains(t, err4.Error(), "failed to set partition UUID")
}

func TestVolumeManager_CreateLocalVolumeLVGFail(t *testing.T) {
	// LVCReate was failed
	vm2 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume2 := volCRLVG.Spec
	e2 := &mocks.GoMockExecutor{}
	sizeStr2 := fmt.Sprintf("%.2fG", float64(volume2.Size)/float64(util.GBYTE))
	expectedErr2 := errors.New("lvcreate failed in test")
	e2.OnCommand(fmt.Sprintf("/sbin/lvm lvcreate --yes --name %s --size %s %s", volume2.Id, sizeStr2, volume2.Location)).
		Return("", "", expectedErr2)

	vm2.linuxUtils = linuxutils.NewLinuxUtils(e2, testLogger)
	err2 := vm2.k8sClient.CreateCR(testCtx, lvgName, &lvgCR)
	assert.Nil(t, err2)

	err2 = vm2.CreateLocalVolume(testCtx, &volume2)
	assert.NotNil(t, err2)
	assert.Contains(t, err2.Error(), "lvcreate failed")
}

func TestVolumeManager_ReconcileCreateVolumeSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	dList := &drivecrd.DriveList{}
	err := vm.k8sClient.ReadList(context.Background(), dList)
	assert.Nil(t, err)
	assert.True(t, len(dList.Items) > 0)
	scMock := &sc.ImplementerMock{}
	scMock.On("CreateFileSystem", sc.XFS, "/dev/sda1").Return(nil).Times(1)
	vm.scMap["hdd"] = scMock
	vol := volCR
	vol.Spec.Location = dList.Items[0].Spec.UUID
	err = vm.k8sClient.CreateCR(context.Background(), testID, &vol)
	assert.Nil(t, err)

	_, err = vm.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: testNs,
		Name:      testID,
	},
	})
	assert.Nil(t, err)

	volAfterReconcile := &vcrd.Volume{}
	err = vm.k8sClient.ReadCR(context.Background(), testID, volAfterReconcile)
	assert.Nil(t, err)

	assert.Equal(t, crdV1.Created, volAfterReconcile.Spec.CSIStatus)
}

func TestVolumeManager_ReconcileCreateVolumeFail(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	// Change VolumeCR size to large. Disk of that size doesn't exist.
	// So CreateLocalVolume fails
	volCRNotFound := volCR
	volCRNotFound.Spec.Size = 1024 * 1024 * 1024 * 1024
	err := vm.k8sClient.CreateCR(context.Background(), testID, &volCRNotFound)
	assert.Nil(t, err)

	_, err = vm.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: testNs,
		Name:      testID,
	},
	})
	assert.Nil(t, err)

	volAfterReconcile := &vcrd.Volume{}
	err = vm.k8sClient.ReadCR(context.Background(), testID, volAfterReconcile)
	assert.Nil(t, err)

	assert.Equal(t, crdV1.Failed, volAfterReconcile.Spec.CSIStatus)
}

func TestVolumeManager_DeleteLocalVolumeSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	scImplMock := &sc.ImplementerMock{}
	scImplMock.On("DeleteFileSystem", "/dev/sdb").Return(nil).Times(1)
	vm.scMap[SCName("hdd")] = scImplMock

	v := api.Volume{Id: testID, Location: drive2.UUID, StorageClass: apiV1.StorageClassHDD}

	err := vm.DeleteLocalVolume(context.Background(), &v)
	assert.Nil(t, err)
	assertLenVListItemsEqualsTo(t, vm.k8sClient, 0)

	// LVG SC
	v = api.Volume{Id: testID, Location: drive2.UUID, StorageClass: apiV1.StorageClassHDDLVG}
	volumeCR := vm.k8sClient.ConstructVolumeCR(v.Id, v)
	addVolumeCRs(vm.k8sClient, *volumeCR)
	lvDev := fmt.Sprintf("/dev/%s/%s", v.Location, v.Id)

	scImplMock.On("DeleteFileSystem", lvDev).
		Return(nil).Times(1)
	mockExecutor := &mocks.GoMockExecutor{}
	mockExecutor.OnCommand(fmt.Sprintf("/sbin/lvm lvremove --yes %s", lvDev)).
		Return("", "", nil).Times(1)

	vm.linuxUtils = linuxutils.NewLinuxUtils(mockExecutor, testLogger)
	err = vm.DeleteLocalVolume(context.Background(), &v)
	assert.Nil(t, err)
}

func TestVolumeManager_DeleteLocalVolumeFail(t *testing.T) {
	// Expect that scImpl wasn't found for volume's storage class
	vm1 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume1 := &api.Volume{Id: testID, Location: drive1.UUID, StorageClass: "random"}
	err1 := vm1.DeleteLocalVolume(testCtx, volume1)
	assert.NotNil(t, err1)
	assert.Contains(t, err1.Error(), "unable to determine storage class for volume")

	// expect searchDrivePathBySN will fail
	vm2 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	e2 := &mocks.GoMockExecutor{}
	e2.OnCommand(lsblkAllDevicesCmd).Return("{\"blockdevices\": []}", "", nil).Times(1)
	vm2.linuxUtils = linuxutils.NewLinuxUtils(e2, testLogger)

	volume := &api.Volume{Id: testID, Location: drive1.UUID, StorageClass: apiV1.StorageClassHDD}
	err2 := vm2.DeleteLocalVolume(testCtx, volume)
	assert.NotNil(t, err2)
	assert.Equal(t, fmt.Sprintf("unable to find device for drive with S/N %s", volume.Location), err2.Error())

	// expect DeletePartition was failed
	vm3 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume3 := api.Volume{Id: testID, Location: drive1.UUID, StorageClass: apiV1.StorageClassHDD}
	volumeCR := vm3.k8sClient.ConstructVolumeCR(volume3.Id, volume3)
	addVolumeCRs(vm3.k8sClient, *volumeCR)

	disk := "/dev/sda"
	isPartitionExistCMD := fmt.Sprintf("partprobe -d -s %s", disk)
	deletePartitionCMD := fmt.Sprintf("parted -s %s1 rm 1", disk)
	e3 := mocks.NewMockExecutor(map[string]mocks.CmdOut{
		lsblkAllDevicesCmd:  {Stdout: mocks.LsblkTwoDevicesStr},
		isPartitionExistCMD: mocks.EmptyOutFail,
		deletePartitionCMD:  mocks.EmptyOutFail,
	})
	e3.SetSuccessIfNotFound(false)
	vm3.linuxUtils = linuxutils.NewLinuxUtils(e3, testLogger)

	err3 := vm3.DeleteLocalVolume(context.Background(), &volume3)
	assert.NotNil(t, err3)
	assert.Contains(t, err3.Error(), "failed to delete partition")

	// expect DeleteFileSystem was failed
	vm4 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume4 := api.Volume{Id: testID, Location: drive1.UUID, StorageClass: apiV1.StorageClassHDDLVG}
	volumeCR4 := vm4.k8sClient.ConstructVolumeCR(volume4.Id, volume4)
	addVolumeCRs(vm4.k8sClient, *volumeCR4)

	device4 := fmt.Sprintf("/dev/%s/%s", volume4.Location, volume4.Id)
	scImplHdd4 := &sc.ImplementerMock{}
	scImplHdd4.On("DeleteFileSystem", device4).Return(errors.New("DeleteFileSystem failed"))
	vm4.scMap["hdd"] = scImplHdd4
	err4 := vm4.DeleteLocalVolume(testCtx, &volume4)
	assert.NotNil(t, err4)
	assert.Contains(t, err4.Error(), "failed to wipefs device")

	// expect LVRemove fail for Volume with SC HDDLVG
	vm5 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume5 := api.Volume{Id: testID, Location: drive1.UUID, StorageClass: apiV1.StorageClassHDDLVG}
	volumeCR5 := vm4.k8sClient.ConstructVolumeCR(volume5.Id, volume5)
	addVolumeCRs(vm5.k8sClient, *volumeCR5)

	device5 := fmt.Sprintf("/dev/%s/%s", volume5.Location, volume5.Id)
	scImplHdd5 := &sc.ImplementerMock{}
	scImplHdd5.On("DeleteFileSystem", device5).Return(nil)
	vm5.scMap["hdd"] = scImplHdd5
	e5 := &mocks.GoMockExecutor{}
	e5.OnCommand(fmt.Sprintf("/sbin/lvm lvremove --yes %s", device5)).
		Return("", "", errors.New("lvremove failed"))

	vm5.linuxUtils = linuxutils.NewLinuxUtils(e5, testLogger)
	err5 := vm5.DeleteLocalVolume(testCtx, &volume5)
	assert.NotNil(t, err5)
	assert.Contains(t, err5.Error(), "unable to remove lv")
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
	)

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
		lu, _   = getLinuxUtilsThatDiscoverLVG()
		m       = prepareSuccessVolumeManager()
		lvgList = lvgcrd.LVGList{}
		acList  = accrd.AvailableCapacityList{}
		err     error
	)
	m.linuxUtils = lu

	// expect success, LVG CR and AC CR was created
	err = m.discoverLVGOnSystemDrive()
	assert.Nil(t, err)

	err = m.k8sClient.ReadList(testCtx, &lvgList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(lvgList.Items))
	lvg := lvgList.Items[0]
	assert.Equal(t, 1, len(lvg.Spec.Locations))
	assert.Equal(t, base.SystemDriveAsLocation, lvg.Spec.Locations[0])
	assert.Equal(t, crdV1.Created, lvg.Spec.Status)

	err = m.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items))
	ac := acList.Items[0]
	assert.Equal(t, lvg.Name, ac.Spec.Location)
	assert.Equal(t, apiV1.StorageClassSSDLVG, ac.Spec.StorageClass)
	assert.Equal(t, lvg.Spec.Size, ac.Spec.Size)
}

// returns LinuxUtils and Executor that prepared for discovering system LVG imitation
func getLinuxUtilsThatDiscoverLVG() (*linuxutils.LinuxUtils, command.CmdExecutor) {
	var (
		cmdFindMnt     = fmt.Sprintf(linuxutils.FindMntCmdTmpl, base.KubeletRootPath)
		findMntRes     = "/dev/mapper/root--vg-lv_var"
		cmdLsblkDev    = fmt.Sprintf(lsblk.CmdTmpl, findMntRes)
		LsblkDevRes    = `{ "blockdevices": [{"name": "/dev/mapper/root--vg-lv_var", "type": "lvm", "size": "102399737856", "rota": "0"}]}`
		cmdFindVGByLV  = fmt.Sprintf(lvm.VGByLVCmdTmpl, findMntRes)
		findVGByLVRes  = "root-vg"
		cmdVGFreeSpace = fmt.Sprintf(lvm.VGFreeSpaceCmdTmpl, findVGByLVRes)
		VGFreeSpaceRes = "102399737856B"

		e = &mocks.GoMockExecutor{}
	)
	e.OnCommand(cmdFindMnt).Return(findMntRes, "", nil)
	e.OnCommand(cmdLsblkDev).Return(LsblkDevRes, "", nil)
	e.OnCommand(cmdFindVGByLV).Return(findVGByLVRes, "", nil)
	e.OnCommand(cmdVGFreeSpace).Return(VGFreeSpaceRes, "", nil)

	return linuxutils.NewLinuxUtils(e, logrus.New()), e
}

func prepareSuccessVolumeManager() *VolumeManager {
	c := mocks.NewMockHWMgrClient(nil)
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
	return NewVolumeManager(c, e, testLogger, kubeClient, nodeID)
}

func prepareSuccessVolumeManagerWithDrives(drives []*api.Drive) *VolumeManager {
	nVM := prepareSuccessVolumeManager()
	nVM.hWMgrClient = mocks.NewMockHWMgrClient(drives)
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
