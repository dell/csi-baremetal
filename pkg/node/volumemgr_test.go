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
	crdV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
)

var vmLogger = logrus.New()

const (
	testNs     = "default"
	nodeId     = "node"
	testID     = "uuid-1111"
	volLVGName = "volume-lvg"
	lvgName    = "lvg-cr-1"
)

var hwMgrRespDrives = []*api.Drive{
	{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd1",
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         1024 * 1024 * 1024 * 50,
		NodeId:       nodeId,
	},
	{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd2",
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         1024 * 1024 * 1024 * 150,
		NodeId:       nodeId,
	},
}
var (
	drive1 = &api.Drive{SerialNumber: "hdd1", Size: 1024 * 1024 * 1024 * 500, NodeId: nodeId} // /dev/sda in LsblkTwoDevices
	drive2 = &api.Drive{SerialNumber: "hdd2", Size: 1024 * 1024 * 1024 * 200, NodeId: nodeId} // /dev/sdb in LsblkTwoDevices

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
			StorageClass: api.StorageClass_HDD,
			Location:     "",
			Status:       api.OperationalStatus_Creating,
			Owner:        nodeId,
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
			Node:      nodeId,
			Locations: []string{drive1.UUID},
			Size:      int64(1024 * 500 * base.GBYTE),
			Status:    api.OperationalStatus_Created,
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
			StorageClass: api.StorageClass_HDDLVG,
			Location:     lvgCR.Name,
			Status:       api.OperationalStatus_Creating,
			Owner:        nodeId,
		},
	}
)

func TestVolumeManager_NewVolumeManager(t *testing.T) {
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, vmLogger, kubeClient, "nodeId")
	assert.NotNil(t, vm)
	assert.Nil(t, vm.hWMgrClient)
	assert.NotNil(t, vm.linuxUtils)
	assert.Equal(t, len(vm.volumesCache), 0)
}

func TestNewVolumeManager_SetExecutor(t *testing.T) {
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, mocks.EmptyExecutorSuccess{}, logrus.New(), kubeClient, "nodeId")
	vm.SetExecutor(mocks.EmptyExecutorFail{})
	res, err := vm.linuxUtils.Lsblk("disk")
	assert.Nil(t, res)
	assert.NotNil(t, err)
}

func TestVolumeManager_SetLinuxUtilsExecutor(t *testing.T) {
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(nil, e, vmLogger, kubeClient, "nodeId")

	out, err := vm.linuxUtils.Lsblk(base.DriveTypeDisk)
	assert.NotNil(t, out)
	if out != nil {
		assert.Equal(t, len(*out), 2)
	}
	assert.Nil(t, err)
}

func TestVolumeManager_GetAvailableCapacitySuccess(t *testing.T) {
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger, kubeClient, "nodeId")
	assert.Empty(t, vm.drivesCache)
	err = vm.Discover()
	assert.Nil(t, err)
	ac, err := vm.GetAvailableCapacity(context.Background(), &api.AvailableCapacityRequest{NodeId: nodeId})
	if ac != nil {
		assert.NotNil(t, ac.GetAvailableCapacity())
		assert.Equal(t, 2, len(ac.GetAvailableCapacity()))
	}

	assert.Nil(t, err)
}

func TestVolumeManager_DrivesNotInUse(t *testing.T) {
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, vmLogger, kubeClient, "nodeId")

	vm.drivesCache["hdd1"] = vm.k8sclient.ConstructDriveCR("hdd1", api.Drive{UUID: "hdd1", SerialNumber: "hdd1", Type: api.DriveType_HDD})
	vm.drivesCache["nvme1"] = vm.k8sclient.ConstructDriveCR("nvme1", api.Drive{UUID: "nvme1", SerialNumber: "nvme1", Type: api.DriveType_NVMe})

	volume := api.Volume{
		LocationType: api.LocationType_Drive,
		Location:     "hdd1",
	}

	drivesNotInUse := vm.drivesAreNotUsed()
	// empty volumes cache, method should return all drives
	assert.NotNil(t, drivesNotInUse)
	assert.Equal(t, 2, len(drivesNotInUse))

	vm.volumesCache[volume.Id] = &volume

	// expect that nvme drive is not used
	drivesNotInUse = vm.drivesAreNotUsed()
	assert.Equal(t, 1, len(drivesNotInUse))
	assert.Equal(t, "nvme1", drivesNotInUse[0].Spec.UUID)
}

func TestVolumeManager_DiscoverFail(t *testing.T) {
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, nil, vmLogger, kubeClient, "nodeId")

	// expect: hwMgrClient request fail with error
	vm.hWMgrClient = mocks.MockHWMgrClientFail{}
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "MockHWMgrClientFail: Error", err.Error())

	// expect: lsblk fail with error
	vm = NewVolumeManager(mocks.MockHWMgrClient{}, mocks.EmptyExecutorFail{}, vmLogger, kubeClient, "nodeId")
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error")

}
func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger, kubeClient, "nodeId")

	// expect that cache is empty because of all drives has not children
	assert.Empty(t, vm.volumesCache)
	err = vm.Discover()
	assert.Nil(t, err)
	assert.Empty(t, vm.volumesCache)

	// expect that volume cache will be empty because one drive without size
	// and GetPartitionGUID returns error for second drive
	expectedCmdOut1 := map[string]mocks.CmdOut{
		base.LsblkCmd:          mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb -i 1": {Stdout: "some output: here"},
	}
	e2 := mocks.NewMockExecutor(expectedCmdOut1)
	vm = NewVolumeManager(*hwMgrClient, e2, vmLogger, kubeClient, "nodeId")
	err = vm.Discover()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(vm.volumesCache))

	// expect that one volume will appear in cache
	expectedCmdOut2 := map[string]mocks.CmdOut{
		base.LsblkCmd:              mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb --info=1": {Stdout: "Partition unique GUID: uniq-guid-for-dev-sdb"},
	}
	e3 := mocks.NewMockExecutor(expectedCmdOut2)
	vm = NewVolumeManager(*hwMgrClient, e3, vmLogger, kubeClient, "nodeId")
	err = vm.Discover()
	assert.Nil(t, err)
	// LsblkDevWithChildren contains 2 devices with children however one of them without size
	// that because we expect one item in volumes cache
	assert.Equal(t, 1, len(vm.volumesCache))
	_, ok := vm.volumesCache["uniq-guid-for-dev-sdb"]
	assert.True(t, ok)

}

func TestVolumeManager_DiscoverAvailableCapacity(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger, kubeClient, "nodeId")
	err = vm.Discover()
	assert.Nil(t, err)
	assert.Empty(t, vm.availableCapacityCache)
	err = vm.DiscoverAvailableCapacity(nodeId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(vm.availableCapacityCache))
}
func TestVolumeManager_DiscoverAvailableCapacityDriveUnhealthy(t *testing.T) {
	//expected 1 available capacity because 1 drive is unhealthy
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	hwMgrRespDrives[1].Health = api.Health_BAD
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger, kubeClient, "nodeId")
	err = vm.Discover()
	assert.Nil(t, err)
	err = vm.DiscoverAvailableCapacity(nodeId)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(vm.availableCapacityCache))
}
func TestVolumeManager_DiscoverAvailableCapacityEmptyCache(t *testing.T) {
	//empty cache because 1 drive is unhealthy and another has volume
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	kubeClient, err := base.GetFakeKubeClient(testNs)
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger, kubeClient, "nodeId")
	hwMgrRespDrives[1].Health = api.Health_BAD
	hwMgrClient.SetDrives(hwMgrRespDrives)
	err = vm.Discover()
	vm.volumesCache["id"] = &api.Volume{
		Id:           "id",
		Owner:        "pod",
		Size:         1000,
		Location:     hwMgrRespDrives[0].UUID,
		LocationType: api.LocationType_Drive,
		Mode:         api.Mode_FS,
		Type:         "xfs",
		Health:       hwMgrRespDrives[0].Health,
		Status:       api.OperationalStatus_Operative,
	}
	assert.Nil(t, err)
	err = vm.DiscoverAvailableCapacity(nodeId)
	assert.Nil(t, err)
	assert.Empty(t, vm.availableCapacityCache)
}

func TestVolumeManager_updatesDrivesCRs(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(hwMgrClient, nil, vmLogger, kubeClient, nodeId)
	assert.Nil(t, err)
	assert.Empty(t, vm.drivesCache)
	ctx := context.Background()
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.drivesCache), 2)

	hwMgrRespDrives[0].Health = api.Health_BAD
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, vm.drivesCache[hwMgrRespDrives[0].UUID].Spec.Health, api.Health_BAD)

	drives := hwMgrRespDrives[1:]
	vm.updateDrivesCRs(ctx, drives)
	assert.Nil(t, err)
	assert.Equal(t, vm.drivesCache[hwMgrRespDrives[0].UUID].Spec.Health, api.Health_UNKNOWN)
	assert.Equal(t, vm.drivesCache[hwMgrRespDrives[0].UUID].Spec.Status, api.Status_OFFLINE)

	vm = NewVolumeManager(hwMgrClient, nil, vmLogger, kubeClient, nodeId)
	assert.Nil(t, err)
	assert.Empty(t, vm.drivesCache)
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.drivesCache), 2)
	hwMgrRespDrives = append(hwMgrRespDrives, &api.Drive{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd3",
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         1024 * 1024 * 1024 * 150,
		NodeId:       nodeId,
	})
	vm.updateDrivesCRs(ctx, hwMgrRespDrives)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.drivesCache), 3)
}

func TestVolumeManager_setPartitionUUIDForDevSuccess(t *testing.T) {
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm := NewVolumeManager(nil, mocks.EmptyExecutorSuccess{}, vmLogger, kubeClient, "nodeId")

	rollBacked, err := vm.setPartitionUUIDForDev("", "")
	assert.Nil(t, err)
	assert.True(t, rollBacked)
}

func TestVolumeManager_setPartitionUUIDForDevFail(t *testing.T) {
	var (
		vm         *VolumeManager
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
	kubeClient, err := base.GetFakeKubeClient(testNs)
	assert.Nil(t, err)
	vm = NewVolumeManager(nil, mocks.EmptyExecutorFail{}, vmLogger, kubeClient, "nodeId")
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)

	// partition has already exist
	cmdRes = mocks.CmdOut{Stdout: "/dev/sda: gpt partitions 1 "}
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{
		"partprobe -d -s /dev/sda": cmdRes,
		"sgdisk /dev/sda --info=1": {Stdout: fmt.Sprintf("Partition unique GUID: %s", uuid)}})
	vm = NewVolumeManager(nil, e, vmLogger, kubeClient, "nodeId")
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.Nil(t, err)
	//assert.Contains(t, err.Error(), "partition has already exist")

	// create partition table failed
	createPTCMD := "parted -s /dev/sda mklabel gpt"
	lifecycleCMD[createPTCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("create partition table failed")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	vm = NewVolumeManager(nil, e, vmLogger, kubeClient, "nodeId")
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to create partition table for device")

	// create partition failed and partition doesn't exist
	createPartCMD := "parted -s /dev/sda mkpart --align optimal CSI 0% 100%"
	lifecycleCMD[partExistCMD] = emptyCmdOk
	lifecycleCMD[createPTCMD] = emptyCmdOk
	lifecycleCMD[createPartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("create partition failed")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	vm = NewVolumeManager(nil, e, vmLogger, kubeClient, "nodeId")
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.Equal(t, errors.New("create partition failed"), err)

	// create partition failed and partition exist and delete partition failed
	deletePartCMD := "parted -s /dev/sda rm 1"
	lifecycleCMD[deletePartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("error")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	// second time show that partition exist
	e.AddSecondRun(partExistCMD, mocks.CmdOut{Stdout: "/dev/sda: gpt partitions 1"})
	vm = NewVolumeManager(nil, e, vmLogger, kubeClient, "nodeId")
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
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
	vm = NewVolumeManager(nil, e, vmLogger, kubeClient, "nodeId")
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.False(t, false)
	assert.NotNil(t, err)
	assert.Equal(t, errors.New("set partition UUID failed"), err)

	// setPartitionUUID failed and delete part CMD pass
	lifecycleCMD[partprobeCMD] = emptyCmdOk
	lifecycleCMD[createPartCMD] = emptyCmdOk
	lifecycleCMD[setPartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("set partition UUID failed")}
	lifecycleCMD[deletePartCMD] = emptyCmdOk
	e = mocks.NewMockExecutor(lifecycleCMD)
	vm = NewVolumeManager(nil, e, vmLogger, kubeClient, "nodeId")
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)
	assert.Equal(t, errors.New("set partition UUID failed"), err)
}

func prepareSuccessVolumeManagerWithDrives(drives []*api.Drive) *VolumeManager {
	c := mocks.NewMockHWMgrClient(drives)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	e.SetSuccessIfNotFound(true)
	kubeClient, err := base.GetFakeKubeClient(testNs)
	if err != nil {
		panic(err)
	}
	nVM := NewVolumeManager(c, e, vmLogger, kubeClient, nodeId)
	// prepare drives cache
	if err := nVM.Discover(); err != nil {
		return nil
	}
	return nVM
}

func TestVolumeManager_CreateLocalVolumeHDDSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	vol := volCR.Spec
	// read Drives
	dList := &drivecrd.DriveList{}
	err := vm.k8sclient.ReadList(context.Background(), dList)
	assert.Nil(t, err)
	assert.True(t, len(dList.Items) > 0)
	vol.Location = dList.Items[0].Spec.UUID
	err = vm.CreateLocalVolume(context.Background(), &vol)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.volumesCache), 1)
}

func TestVolumeManager_CreateLocalVolumeLVGSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	e := &mocks.GoMockExecutor{}
	vm.linuxUtils = base.NewLinuxUtils(e, vmLogger)
	volume := volCRLVG.Spec
	sizeStr := fmt.Sprintf("%.2fG", float64(volume.Size)/float64(base.GBYTE))
	e.OnCommand(fmt.Sprintf("/sbin/lvm lvcreate --yes --name %s --size %s %s", volume.Id, sizeStr, volume.Location)).
		Return("", "", nil)

	err := vm.k8sclient.CreateCR(ctx, &lvgCR, lvgName)
	assert.Nil(t, err)

	err = vm.CreateLocalVolume(ctx, &volume)
	assert.Equal(t, len(vm.volumesCache), 1)
	assert.Equal(t, vm.volumesCache[volume.Id].Status, api.OperationalStatus_Created)
}

func TestVolumeManager_CreateLocalVolumeHDDFail(t *testing.T) {
	// expect: setPartitionUUIDForDev fail but partition hadn't created (rollback is no needed)
	vm3 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1})
	dList := &drivecrd.DriveList{}
	err := vm3.k8sclient.ReadList(context.Background(), dList)
	assert.Nil(t, err)
	assert.True(t, len(dList.Items) > 0)

	e3 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr},
		fmt.Sprintf("partprobe -d -s /dev/sda"): {Err: errors.New("partprobe -d -s /dev/sda failed")}})
	e3.SetSuccessIfNotFound(false)
	vm3.linuxUtils = base.NewLinuxUtils(e3, vmLogger)
	err = vm3.Discover()
	assert.Nil(t, err)
	vol3 := &api.Volume{
		Id:           testID,
		Size:         1024 * 1024 * 1024 * 45,
		StorageClass: api.StorageClass_HDD,
		Location:     dList.Items[0].Spec.UUID,
	}
	err3 := vm3.CreateLocalVolume(context.Background(), vol3)
	assert.NotNil(t, err3)
	assert.Contains(t, err3.Error(), "unable to create local volume")

	// expect: setPartitionUUIDForDev fail partition was created and rollback was failed too
	vm4 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1})
	dList2 := &drivecrd.DriveList{}
	err = vm4.k8sclient.ReadList(context.Background(), dList2)
	assert.Nil(t, err)
	assert.True(t, len(dList2.Items) > 0)
	vID := "uuid-4444"
	setUUIDCMD := fmt.Sprintf("sgdisk /dev/sda --partition-guid=1:%s", vID) // /dev/sda - drive1
	deletePartCMD := "parted -s /dev/sda rm 1"
	eMap := map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr},
		setUUIDCMD:    mocks.EmptyOutFail,
		deletePartCMD: mocks.EmptyOutFail}
	e4 := mocks.NewMockExecutor(eMap)
	e4.SetSuccessIfNotFound(true)
	vm4.linuxUtils = base.NewLinuxUtils(e4, vmLogger)

	vol4 := &api.Volume{
		Id:           vID,
		Size:         1024 * 1024 * 1024 * 45,
		StorageClass: api.StorageClass_HDD,
		Location:     dList2.Items[0].Spec.UUID,
	}
	err4 := vm4.CreateLocalVolume(context.Background(), vol4)
	assert.NotNil(t, err4)
	assert.Contains(t, err4.Error(), fmt.Sprintf("unable to create local volume %s", vID))
}

func TestVolumeManager_CreateLocalVolumeLVGFail(t *testing.T) {
	// LVCReate was failed
	vm2 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume2 := volCRLVG.Spec
	e2 := &mocks.GoMockExecutor{}
	sizeStr2 := fmt.Sprintf("%.2fG", float64(volume2.Size)/float64(base.GBYTE))
	expectedErr2 := errors.New("lvcreate failed in test")
	e2.OnCommand(fmt.Sprintf("/sbin/lvm lvcreate --yes --name %s --size %s %s", volume2.Id, sizeStr2, volume2.Location)).
		Return("", "", expectedErr2)
	vm2.linuxUtils = base.NewLinuxUtils(e2, vmLogger)

	err2 := vm2.k8sclient.CreateCR(ctx, &lvgCR, lvgName)
	assert.Nil(t, err2)

	err2 = vm2.CreateLocalVolume(ctx, &volume2)
	assert.NotNil(t, err2)
	assert.Equal(t, err2, expectedErr2)
}

func TestVolumeManager_ReconcileCreateVolumeSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	dList := &drivecrd.DriveList{}
	err := vm.k8sclient.ReadList(context.Background(), dList)
	assert.Nil(t, err)
	assert.True(t, len(dList.Items) > 0)

	vol := volCR
	vol.Spec.Location = dList.Items[0].Spec.UUID
	err = vm.k8sclient.CreateCR(context.Background(), &vol, testID)
	assert.Nil(t, err)

	_, err = vm.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: testNs,
		Name:      testID,
	},
	})
	assert.Nil(t, err)

	volAfterReconcile := &vcrd.Volume{}
	err = vm.k8sclient.ReadCR(context.Background(), testID, volAfterReconcile)
	assert.Nil(t, err)

	assert.Equal(t, api.OperationalStatus_Created, volAfterReconcile.Spec.Status)
}

func TestVolumeManager_ReconcileCreateVolumeFail(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	// Change VolumeCR size to large. Disk of that size doesn't exist.
	// So CreateLocalVolume fails
	volCRNotFound := volCR
	volCRNotFound.Spec.Size = 1024 * 1024 * 1024 * 1024
	err := vm.k8sclient.CreateCR(context.Background(), &volCRNotFound, testID)
	assert.Nil(t, err)

	_, err = vm.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{
		Namespace: testNs,
		Name:      testID,
	},
	})
	assert.Nil(t, err)

	volAfterReconcile := &vcrd.Volume{}
	err = vm.k8sclient.ReadCR(context.Background(), testID, volAfterReconcile)
	assert.Nil(t, err)

	assert.Equal(t, api.OperationalStatus_FailedToCreate, volAfterReconcile.Spec.Status)
}

func TestVolumeManager_DeleteLocalVolumeSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	scImplMock := &sc.ImplementerMock{}
	scImplMock.On("DeleteFileSystem", "/dev/sdb").Return(nil).Times(1)
	vm.scMap[SCName("hdd")] = scImplMock

	v := &api.Volume{Id: testID, Location: drive2.UUID, StorageClass: api.StorageClass_HDD}
	vm.volumesCache[testID] = v

	err := vm.DeleteLocalVolume(context.Background(), v)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(vm.volumesCache))

	// LVG SC
	v = &api.Volume{Id: testID, Location: drive2.UUID, StorageClass: api.StorageClass_HDDLVG}
	vm.volumesCache[testID] = v
	lvDev := fmt.Sprintf("/dev/%s/%s", v.Location, v.Id)

	scImplMock.On("DeleteFileSystem", lvDev).
		Return(nil).Times(1)
	mockExecutor := &mocks.GoMockExecutor{}
	mockExecutor.OnCommand(fmt.Sprintf("/sbin/lvm lvremove --yes %s", lvDev)).
		Return("", "", nil).Times(1)
	vm.linuxUtils = base.NewLinuxUtils(mockExecutor, vmLogger)
	err = vm.DeleteLocalVolume(context.Background(), v)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(vm.volumesCache))
}

func TestVolumeManager_DeleteLocalVolumeFail(t *testing.T) {
	// Expect that scImpl wasn't found for volume's storage class
	vm1 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume1 := &api.Volume{Id: testID, Location: drive1.UUID, StorageClass: -1}
	err1 := vm1.DeleteLocalVolume(ctx, volume1)
	assert.NotNil(t, err1)
	assert.Contains(t, err1.Error(), "unable to determine storage class for volume")

	// expect searchDrivePathBySN will fail
	vm2 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})

	e2 := &mocks.GoMockExecutor{}
	e2.OnCommand(base.LsblkCmd).Return("{\"blockdevices\": []}", "", nil).Times(1)
	vm2.linuxUtils = base.NewLinuxUtils(e2, vmLogger)

	volume := &api.Volume{Id: testID, Location: drive1.UUID, StorageClass: api.StorageClass_HDD}
	err2 := vm2.DeleteLocalVolume(ctx, volume)
	assert.NotNil(t, err2)
	assert.Equal(t, err2.Error(), fmt.Sprintf("unable to find device for drive with S/N %s", volume.Location))

	// expect DeletePartition was failed
	vm3 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume3 := &api.Volume{Id: testID, Location: drive1.UUID, StorageClass: api.StorageClass_HDD}
	vm3.volumesCache[testID] = volume3
	disk := "/dev/sda"
	isPartitionExistCMD := fmt.Sprintf("partprobe -d -s %s", disk)
	deletePartitionCMD := fmt.Sprintf("parted -s %s1 rm 1", disk)
	e3 := mocks.NewMockExecutor(map[string]mocks.CmdOut{
		base.LsblkCmd:       {Stdout: mocks.LsblkTwoDevicesStr},
		isPartitionExistCMD: mocks.EmptyOutFail,
		deletePartitionCMD:  mocks.EmptyOutFail,
	})
	e3.SetSuccessIfNotFound(false)
	vm3.linuxUtils = base.NewLinuxUtils(e3, vmLogger)

	err3 := vm3.DeleteLocalVolume(context.Background(), volume3)
	assert.NotNil(t, err3)
	assert.Contains(t, err3.Error(), "failed to delete partition")
	assert.Equal(t, api.OperationalStatus_FailToRemove, vm3.volumesCache[testID].Status)

	// expect DeleteFileSystem was failed
	vm4 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume4 := &api.Volume{Id: testID, Location: drive1.UUID, StorageClass: api.StorageClass_HDDLVG}
	vm4.volumesCache[testID] = volume4
	device4 := fmt.Sprintf("/dev/%s/%s", volume4.Location, volume4.Id)
	scImplHdd4 := &sc.ImplementerMock{}
	scImplHdd4.On("DeleteFileSystem", device4).Return(errors.New("DeleteFileSystem failed"))
	vm4.scMap["hdd"] = scImplHdd4
	err4 := vm4.DeleteLocalVolume(ctx, volume4)
	assert.NotNil(t, err4)
	assert.Contains(t, err4.Error(), "failed to wipefs device")
	assert.Equal(t, vm4.volumesCache[volume4.Id].Status, api.OperationalStatus_FailToRemove)

	// expect LVRemove fail for Volume with SC HDDLVG
	vm5 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	volume5 := &api.Volume{Id: testID, Location: drive1.UUID, StorageClass: api.StorageClass_HDDLVG}
	vm5.volumesCache[testID] = volume5
	device5 := fmt.Sprintf("/dev/%s/%s", volume5.Location, volume5.Id)
	scImplHdd5 := &sc.ImplementerMock{}
	scImplHdd5.On("DeleteFileSystem", device5).Return(nil)
	vm5.scMap["hdd"] = scImplHdd5
	e5 := &mocks.GoMockExecutor{}
	e5.OnCommand(fmt.Sprintf("/sbin/lvm lvremove --yes %s", device5)).
		Return("", "", errors.New("lvremove failed"))
	vm5.linuxUtils = base.NewLinuxUtils(e5, vmLogger)
	err5 := vm5.DeleteLocalVolume(ctx, volume5)
	assert.NotNil(t, err5)
	assert.Contains(t, err5.Error(), "unable to remove lv")
	assert.Equal(t, vm5.volumesCache[volume5.Id].Status, api.OperationalStatus_FailToRemove)
}
