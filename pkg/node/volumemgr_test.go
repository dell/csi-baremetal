package node

import (
	"context"
	"testing"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"github.com/stretchr/testify/assert"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

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
	vm := NewVolumeManager(nil, nil)
	assert.NotNil(t, vm)
	assert.Nil(t, vm.hWMgrClient)
	assert.NotNil(t, vm.linuxUtils)
	assert.Equal(t, len(vm.volumesCache), 0)
}

func TestVolumeManager_SetLinuxUtilsExecutor(t *testing.T) {
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: mocks.LsblkTwoDevices})
	vm := NewVolumeManager(nil, e)

	out, err := vm.linuxUtils.Lsblk(base.DriveTypeDisk)
	assert.NotNil(t, out)
	assert.Equal(t, len(*out), 2)
	assert.Nil(t, err)
}

func TestVolumeManager_GetLocalVolumesSuccess(t *testing.T) {
	vm := NewVolumeManager(nil, nil)
	lvr, err := vm.GetLocalVolumes(context.Background(), &api.VolumeRequest{})
	assert.NotNil(t, lvr)
	assert.Nil(t, err)
}

func TestVolumeManager_GetAvailableCapacitySuccess(t *testing.T) {
	vm := NewVolumeManager(nil, nil)
	ac, err := vm.GetAvailableCapacity(context.Background(), &api.AvailableCapacityRequest{})
	assert.NotNil(t, ac)
	assert.Nil(t, err)
}

func TestVolumeManager_DrivesNotInUse(t *testing.T) {
	vm := NewVolumeManager(nil, nil)

	drives := []*api.Drive{
		{SerialNumber: "hdd1", Type: api.DriveType_HDD},
		{SerialNumber: "nvme1", Type: api.DriveType_NVMe},
	}

	volume := api.Volume{
		LocationType: api.LocationType_Drive,
		Location:     "hdd1",
	}

	drivesNotInUse := vm.drivesAreNotUsed(drives)
	// empty volumes cache, method should return all drives
	assert.NotNil(t, drivesNotInUse)
	assert.Equal(t, 2, len(drivesNotInUse))

	vm.volumesCache = append(vm.volumesCache, &volume)

	// expect that nvme drive is not used
	drivesNotInUse = vm.drivesAreNotUsed(drives)
	assert.Equal(t, 1, len(drivesNotInUse))
	assert.Equal(t, "nvme1", drivesNotInUse[0].SerialNumber)
}

func TestVolumeManager_DiscoverFail(t *testing.T) {
	vm := NewVolumeManager(nil, nil)

	// expect: hwMgrClient request fail with error
	vm.hWMgrClient = mocks.MockHWMgrClientFail{}
	err := vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "MockHWMgrClientFail: Error", err.Error())

	// expect: lsblk fail with error
	vm = NewVolumeManager(mocks.MockHWMgrClient{}, mocks.EmptyExecutorFail{})
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "error", err.Error())

}
func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: mocks.LsblkTwoDevices})
	vm := NewVolumeManager(*hwMgrClient, e1)

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
	vm = NewVolumeManager(*hwMgrClient, e2)
	err = vm.Discover()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(vm.volumesCache))

	// expect that one volume will appear in cache
	expectedCmdOut2 := map[string]mocks.CmdOut{
		base.LsblkCmd:          mocks.LsblkDevWithChildren,
		"sgdisk /dev/sdb -i 1": {Stdout: "Partition unique GUID: uniq-guid-for-dev-sdb"},
	}
	e3 := mocks.NewMockExecutor(expectedCmdOut2)
	vm = NewVolumeManager(*hwMgrClient, e3)
	err = vm.Discover()
	assert.Nil(t, err)
	// LsblkDevWithChildren contains 2 devices with children however one of them without size
	// that because we expect one item in volumes cache
	assert.Equal(t, 1, len(vm.volumesCache))
	assert.Equal(t, "uniq-guid-for-dev-sdb", vm.volumesCache[0].Id)

}

func TestVolumeManager_getDrivePathBySN(t *testing.T) {
	e1 := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: mocks.LsblkTwoDevices})
	vm := NewVolumeManager(nil, e1)

	// success
	dev, err := vm.getDrivePathBySN("hdd1")
	expectedDev := "/dev/sda"
	assert.Nil(t, err)
	assert.Equal(t, expectedDev, dev)

	// fail: dev was not found
	dev, err = vm.getDrivePathBySN("hdd12341")
	assert.Empty(t, dev)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to find drive path")

	// fail: lsblk was failed
	e2 := mocks.EmptyExecutorFail{}
	vm = NewVolumeManager(nil, e2)
	dev, err = vm.getDrivePathBySN("hdd12341")
	assert.Empty(t, dev)
	assert.NotNil(t, err)
}

func TestNewVolumeManager_searchFreeDrive(t *testing.T) {
	// call to HWMgr fail
	hwMgrClient := mocks.MockHWMgrClientFail{}
	vm := NewVolumeManager(hwMgrClient, nil)
	drive, err := vm.searchFreeDrive(1024 * 1024 * 1024 * 100)
	assert.Nil(t, drive)
	assert.NotNil(t, err)

	// success, got second drive from hwMgrRespDrives
	hwMgrClient2 := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: mocks.LsblkTwoDevices})
	vm2 := NewVolumeManager(hwMgrClient2, e)
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
