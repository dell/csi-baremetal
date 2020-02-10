package node

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"github.com/stretchr/testify/assert"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

var vmLogger = logrus.New()

var hwMgrRespDrives = []*api.Drive{
	{
		SerialNumber: "hdd1",
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         1024 * 1024 * 1024 * 50,
	},
	{
		SerialNumber: "hdd2",
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         1024 * 1024 * 1024 * 150,
	},
}

func TestVolumeManager_NewVolumeManager(t *testing.T) {
	vm := NewVolumeManager(nil, nil, vmLogger)
	assert.NotNil(t, vm)
	assert.Nil(t, vm.hWMgrClient)
	assert.NotNil(t, vm.linuxUtils)
	assert.Equal(t, len(vm.volumesCache), 0)
}

func TestVolumeManager_SetLinuxUtilsExecutor(t *testing.T) {
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(nil, e, vmLogger)

	out, err := vm.linuxUtils.Lsblk(base.DriveTypeDisk)
	assert.NotNil(t, out)
	assert.Equal(t, len(*out), 2)
	assert.Nil(t, err)
}

func TestVolumeManager_GetLocalVolumesSuccess(t *testing.T) {
	vm := NewVolumeManager(nil, nil, vmLogger)
	vm.volumesCache["id1"] = &api.Volume{Id: "id1", Owner: "test"}
	vm.volumesCache["id2"] = &api.Volume{Id: "id2", Owner: "test"}
	lvr, err := vm.GetLocalVolumes(context.Background(), &api.VolumeRequest{})
	assert.NotNil(t, lvr)
	assert.Nil(t, err)
	assert.Equal(t, len(vm.volumesCache), len(lvr.Volumes))
}

func TestVolumeManager_GetAvailableCapacitySuccess(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger)
	err := vm.Discover()
	assert.Nil(t, err)
	const nodeId = "node"
	ac, err := vm.GetAvailableCapacity(context.Background(), &api.AvailableCapacityRequest{NodeId: nodeId})
	assert.NotNil(t, ac.GetAvailableCapacity())
	assert.Equal(t, 2, len(ac.GetAvailableCapacity()))
	assert.Nil(t, err)
}

func TestVolumeManager_DrivesNotInUse(t *testing.T) {
	vm := NewVolumeManager(nil, nil, vmLogger)

	vm.drivesCache["hdd1"] = &api.Drive{SerialNumber: "hdd1", Type: api.DriveType_HDD}
	vm.drivesCache["nvme1"] = &api.Drive{SerialNumber: "nvme1", Type: api.DriveType_NVMe}

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
	assert.Equal(t, "nvme1", drivesNotInUse[0].SerialNumber)
}

func TestVolumeManager_DiscoverFail(t *testing.T) {
	vm := NewVolumeManager(nil, nil, vmLogger)

	// expect: hwMgrClient request fail with error
	vm.hWMgrClient = mocks.MockHWMgrClientFail{}
	err := vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "MockHWMgrClientFail: Error", err.Error())

	// expect: lsblk fail with error
	vm = NewVolumeManager(mocks.MockHWMgrClient{}, mocks.EmptyExecutorFail{}, vmLogger)
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error")

}
func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger)

	// expect that cache is empty because of all drives has not children
	assert.Empty(t, vm.volumesCache)
	err := vm.Discover()
	assert.Nil(t, err)
	assert.Empty(t, vm.volumesCache)

	// expect that volume cache will be empty because one drive without size
	// and GetPartitionGUID returns error for second drive
	expectedCmdOut1 := map[string]mocks.CmdOut{
		base.LsblkCmd:          mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb -i 1": {Stdout: "some output: here"},
	}
	e2 := mocks.NewMockExecutor(expectedCmdOut1)
	vm = NewVolumeManager(*hwMgrClient, e2, vmLogger)
	err = vm.Discover()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(vm.volumesCache))

	// expect that one volume will appear in cache
	expectedCmdOut2 := map[string]mocks.CmdOut{
		base.LsblkCmd:              mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb --info=1": {Stdout: "Partition unique GUID: uniq-guid-for-dev-sdb"},
	}
	e3 := mocks.NewMockExecutor(expectedCmdOut2)
	vm = NewVolumeManager(*hwMgrClient, e3, vmLogger)
	err = vm.Discover()
	assert.Nil(t, err)
	// LsblkDevWithChildren contains 2 devices with children however one of them without size
	// that because we expect one item in volumes cache
	assert.Equal(t, 1, len(vm.volumesCache))
	_, ok := vm.volumesCache["uniq-guid-for-dev-sdb"]
	assert.True(t, ok)

}

func TestVolumeManager_getDrivePathBySN(t *testing.T) {
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(nil, e1, vmLogger)

	// success
	dev, err := vm.searchDrivePathBySN("hdd1")
	expectedDev := "/dev/sda"
	assert.Nil(t, err)
	assert.Equal(t, expectedDev, dev)

	// fail: dev was not found
	dev, err = vm.searchDrivePathBySN("hdd12341")
	assert.Empty(t, dev)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to find drive path")

	// fail: lsblk was failed
	e2 := mocks.EmptyExecutorFail{}
	vm = NewVolumeManager(nil, e2, vmLogger)
	dev, err = vm.searchDrivePathBySN("hdd12341")
	assert.Empty(t, dev)
	assert.NotNil(t, err)
}

func TestVolumeManager_DiscoverAvailableCapacity(t *testing.T) {
	const nodeId = "node"
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm := NewVolumeManager(*hwMgrClient, e1, vmLogger)
	err := vm.Discover()
	assert.Nil(t, err)
	assert.Empty(t, vm.availableCapacityCache)
	err = vm.DiscoverAvailableCapacity(nodeId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(vm.availableCapacityCache))

	//expected 1 available capacity because 1 drive is unhealthy
	hwMgrRespDrives[1].Health = api.Health_BAD
	vm = NewVolumeManager(*hwMgrClient, e1, vmLogger)
	err = vm.Discover()
	assert.Nil(t, err)
	err = vm.DiscoverAvailableCapacity(nodeId)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(vm.availableCapacityCache))

	//empty cache because 1 drive is unhealthy and another has volume
	vm = NewVolumeManager(*hwMgrClient, e1, vmLogger)
	hwMgrRespDrives[1].Health = api.Health_BAD
	err = vm.Discover()
	vm.volumesCache["id"] = &api.Volume{
		Id:           "id",
		Owner:        "pod",
		Size:         1000,
		Location:     hwMgrRespDrives[0].SerialNumber,
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

func TestNewVolumeManager_searchFreeDrive(t *testing.T) {
	// call to HWMgr fail
	hwMgrClient := mocks.MockHWMgrClientFail{}
	vm := NewVolumeManager(hwMgrClient, nil, vmLogger)
	drive, err := vm.searchFreeDrive(1024 * 1024 * 1024 * 100)
	assert.Nil(t, drive)
	assert.NotNil(t, err)

	// success, got second drive from hwMgrRespDrives
	hwMgrClient2 := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	vm2 := NewVolumeManager(hwMgrClient2, e, vmLogger)
	_ = vm2.Discover()
	drive2, err2 := vm2.searchFreeDrive(1024 * 1024 * 1024 * 100)
	assert.Nil(t, err2)
	assert.NotNil(t, drive2)
	assert.Equal(t, hwMgrRespDrives[1], drive2)

	// fail, unable to find suitable drive with capacity
	drive3, err3 := vm2.searchFreeDrive(1024 * 1024 * 1024 * 1024)
	assert.Nil(t, drive3)
	assert.NotNil(t, err3)
	assert.Contains(t, err3.Error(), "unable to find suitable drive with capacity")
}

func TestVolumeManager_updatesDrivesCache(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	vm := NewVolumeManager(hwMgrClient, nil, vmLogger)

	assert.Empty(t, vm.drivesCache)
	vm.updateDrivesCache(hwMgrRespDrives)
	assert.Equal(t, len(hwMgrRespDrives), len(vm.drivesCache))

	// add one more drive in response
	newDrives := append(hwMgrRespDrives, &api.Drive{SerialNumber: "hdd_new"})
	vm.updateDrivesCache(newDrives)
	assert.Equal(t, 3, len(vm.drivesCache))

	// hw response will contain 2 drives but in cache are 3 drives
	vm.updateDrivesCache(hwMgrRespDrives)
	assert.Equal(t, api.Status_OFFLINE, vm.drivesCache["hdd_new"].Status)
}

func TestVolumeManager_setPartitionUUIDForDevSuccess(t *testing.T) {
	vm := NewVolumeManager(nil, mocks.EmptyExecutorSuccess{}, vmLogger)

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
		partExistCMD: {"/dev/sda: gpt", "", nil},
	}

	// unable check whether partition exist
	vm = NewVolumeManager(nil, mocks.EmptyExecutorFail{}, vmLogger)
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)

	// partition has already exist
	cmdRes = mocks.CmdOut{Stdout: "/dev/sda: gpt partitions 1 "}
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{"partprobe -d -s /dev/sda": cmdRes})
	vm = NewVolumeManager(nil, e, vmLogger)
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "partition has already exist")

	// create partition table failed
	createPTCMD := "parted -s /dev/sda mklabel gpt"
	lifecycleCMD[createPTCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("create partition table failed")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	vm = NewVolumeManager(nil, e, vmLogger)
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
	vm = NewVolumeManager(nil, e, vmLogger)
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.Equal(t, errors.New("create partition failed"), err)

	// create partition failed and partition exist and delete partition failed
	deletePartCMD := "parted -s /dev/sda rm 1"
	lifecycleCMD[deletePartCMD] = mocks.CmdOut{Stdout: "", Stderr: "", Err: errors.New("error")}
	e = mocks.NewMockExecutor(lifecycleCMD)
	// second time show that partition exist
	e.AddSecondRun(partExistCMD, mocks.CmdOut{Stdout: "/dev/sda: gpt partitions 1"})
	vm = NewVolumeManager(nil, e, vmLogger)
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
	vm = NewVolumeManager(nil, e, vmLogger)
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
	vm = NewVolumeManager(nil, e, vmLogger)
	rollBacked, err = vm.setPartitionUUIDForDev(dev, uuid)
	assert.True(t, rollBacked)
	assert.NotNil(t, err)
	assert.Equal(t, errors.New("set partition UUID failed"), err)
}

var (
	drive1 = &api.Drive{SerialNumber: "hdd1", Size: 1024 * 1024 * 1024 * 500} // /dev/sda in LsblkTwoDevices
	drive2 = &api.Drive{SerialNumber: "hdd2", Size: 1024 * 1024 * 1024 * 200} // /dev/sdb in LsblkTwoDevices
)

func prepareSuccessVolumeManagerWithDrives(drives []*api.Drive) *VolumeManager {
	c := mocks.NewMockHWMgrClient(drives)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	e.SetSuccessIfNotFound(true)
	return NewVolumeManager(c, e, vmLogger)
}

func TestVolumeManager_CreateLocalVolumeSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	req := &api.CreateLocalVolumeRequest{
		PvcUUID:  "uuid-1111",
		Capacity: 1024 * 1024 * 1024 * 150,
		Sc:       "hdd",
		Location: "hdd2",
	}
	resp, err := vm.CreateLocalVolume(context.Background(), req)
	assert.NotNil(t, resp)
	assert.Nil(t, err)
	assert.True(t, resp.Ok)
	assert.Equal(t, "/dev/sdb", resp.Drive)
	assert.Equal(t, drive2.Size, resp.Capacity)

}

func TestVolumeManager_CreateLocalVolumeFail(t *testing.T) {
	// expect: searchDrivePathBySN fail
	// fmt.Errorf("unable to find drive path by S/N %s", sn)
	sn := "will-not-be-found"
	drive3 := &api.Drive{Size: 1024 * 1024 * 1024 * 50, SerialNumber: sn}
	vm2 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2, drive3})
	req2 := &api.CreateLocalVolumeRequest{
		PvcUUID:  "uuid-1111",
		Capacity: 1024 * 1024 * 1024 * 45, // expect drive3 here
		Sc:       "hdd",
		Location: "will-not-be-found",
	}
	resp2, err2 := vm2.CreateLocalVolume(context.Background(), req2)
	assert.NotNil(t, resp2)
	assert.False(t, resp2.Ok)
	assert.NotNil(t, err2)
	assert.Equal(t, fmt.Errorf("unable to find drive path by S/N %s", sn), err2)

	// expect: setPartitionUUIDForDev fail but partition hadn't created (rollback is no needed)
	vm3 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	e3 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	e3.SetSuccessIfNotFound(false)
	vm3.linuxUtils = base.NewLinuxUtils(e3, vmLogger)

	req3 := &api.CreateLocalVolumeRequest{
		PvcUUID:  "uuid-1111",
		Capacity: 1024 * 1024 * 1024 * 45,
		Sc:       "hdd",
		Location: "hdd1",
	}
	resp3, err3 := vm3.CreateLocalVolume(context.Background(), req3)
	assert.NotNil(t, resp3)
	assert.False(t, resp3.Ok)
	assert.NotNil(t, err3)
	assert.Contains(t, err3.Error(), "unable to check partition existence for")

	// expect: setPartitionUUIDForDev fail partition was created and rollback was failed too
	vm4 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	uuid := "uuid-4444"
	setUUIDCMD := fmt.Sprintf("sgdisk /dev/sdb --partition-guid=1:%s", uuid)
	deletePartCMD := "parted -s /dev/sdb rm 1"
	eMap := map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr},
		setUUIDCMD:    mocks.EmptyOutFail,
		deletePartCMD: mocks.EmptyOutFail}
	e4 := mocks.NewMockExecutor(eMap)
	e4.SetSuccessIfNotFound(true)
	vm4.linuxUtils = base.NewLinuxUtils(e4, vmLogger)

	req4 := &api.CreateLocalVolumeRequest{
		PvcUUID:  uuid,
		Capacity: 1024 * 1024 * 1024 * 45,
		Sc:       "hdd",
		Location: "hdd2",
	}
	resp4, err4 := vm4.CreateLocalVolume(context.Background(), req4)
	assert.NotNil(t, resp4)
	assert.False(t, resp4.Ok)
	assert.NotNil(t, err4)
	assert.Equal(t, mocks.EmptyOutFail.Err, err4)
	assert.Equal(t, vm4.drivesCache[drive2.SerialNumber].Status, api.Status_OFFLINE)
}

func TestVolumeManager_DeleteLocalVolumeSuccess(t *testing.T) {
	vm := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	uuid := "uuid-1111"
	vm.volumesCache[uuid] = &api.Volume{Id: uuid, Location: drive2.SerialNumber}

	req := &api.DeleteLocalVolumeRequest{
		PvcUUID: uuid,
	}

	assert.Equal(t, 1, len(vm.volumesCache))
	resp, err := vm.DeleteLocalVolume(context.Background(), req)
	assert.NotNil(t, resp)
	assert.Nil(t, err)
	assert.True(t, resp.Ok)
	assert.Equal(t, 0, len(vm.volumesCache))
}

func TestVolumeManager_DeleteLocalVolumeFail(t *testing.T) {
	// expect: volume wasn't found in volumesCache
	vm1 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	uuid := "uuid-1111"
	req1 := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}

	resp1, err1 := vm1.DeleteLocalVolume(context.Background(), req1)
	assert.NotNil(t, resp1)
	assert.NotNil(t, err1)
	assert.False(t, resp1.Ok)
	assert.Equal(t, errors.New("unable to find volume by PVC UUID in volume manager cache"), err1)

	// expect searchDrivePathBySN return error
	vm2 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	location := "fail-here"
	vm2.volumesCache[uuid] = &api.Volume{Id: uuid, Location: location}
	req2 := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}

	resp2, err2 := vm2.DeleteLocalVolume(context.Background(), req2)
	assert.NotNil(t, resp2)
	assert.NotNil(t, err2)
	assert.False(t, resp2.Ok)
	assert.Equal(t, fmt.Errorf("unable to find device for drive with S/N %s", location), err2)

	// expect DeletePartition was failed
	vm3 := prepareSuccessVolumeManagerWithDrives([]*api.Drive{drive1, drive2})
	vm3.volumesCache[uuid] = &api.Volume{Id: uuid, Location: drive1.SerialNumber}
	deletePartitionCMD := fmt.Sprintf("parted -s %s1 rm 1", "/dev/sda")
	e3 := mocks.NewMockExecutor(map[string]mocks.CmdOut{
		base.LsblkCmd:      {Stdout: mocks.LsblkTwoDevicesStr},
		deletePartitionCMD: mocks.EmptyOutFail,
	})
	e3.SetSuccessIfNotFound(false)
	vm3.linuxUtils = base.NewLinuxUtils(e3, vmLogger)

	req3 := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}

	resp3, err3 := vm3.DeleteLocalVolume(context.Background(), req3)
	assert.NotNil(t, resp3)
	assert.NotNil(t, err3)
	assert.False(t, resp3.Ok)
	assert.Contains(t, err3.Error(), "failed to delete partition")
	assert.Equal(t, api.OperationalStatus_FailToRemove, vm3.volumesCache[uuid].Status)
}
