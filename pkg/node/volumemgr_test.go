package node

import (
	"context"
	"testing"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"github.com/stretchr/testify/assert"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
)

func TestVolumeManager_NewVolumeManager(t *testing.T) {
	vm := NewVolumeManager(nil)
	assert.NotNil(t, vm)
	assert.Nil(t, vm.hWMgrClient)
	assert.NotNil(t, vm.linuxUtils)
	assert.Equal(t, len(vm.volumesCache), 0)
}

func TestVolumeManager_SetLinuxUtilsExecutor(t *testing.T) {
	vm := NewVolumeManager(nil)
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: mocks.LsblkTwoDevices})
	vm.SetLinuxUtilsExecutor(e)
	out, err := vm.linuxUtils.Lsblk(base.DriveTypeDisk)
	assert.NotNil(t, out)
	assert.Equal(t, len(*out), 2)
	assert.Nil(t, err)
}

func TestVolumeManager_GetLocalVolumesSuccess(t *testing.T) {
	vm := NewVolumeManager(nil)
	lvr, err := vm.GetLocalVolumes(context.Background(), &api.VolumeRequest{})
	assert.NotNil(t, lvr)
	assert.Nil(t, err)
}

func TestVolumeManager_GetAvailableCapacitySuccess(t *testing.T) {
	vm := NewVolumeManager(nil)
	ac, err := vm.GetAvailableCapacity(context.Background(), &api.AvailableCapacityRequest{})
	assert.NotNil(t, ac)
	assert.Nil(t, err)
}

func TestVolumeManager_DrivesNotInUse(t *testing.T) {
	vm := NewVolumeManager(nil)

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
	vm := NewVolumeManager(nil)

	// expect: hwMgrClient request fail with error
	vm.hWMgrClient = mocks.MockHWMgrClientFail{}
	err := vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "MockHWMgrClientFail: Error", err.Error())

	// expect: lsblk fail with error
	vm.SetLinuxUtilsExecutor(mocks.EmptyExecutorFail{})
	vm.hWMgrClient = mocks.MockHWMgrClient{}
	err = vm.Discover()
	assert.NotNil(t, err)
	assert.Equal(t, "error", err.Error())

}
func TestVolumeManager_DiscoverSuccess(t *testing.T) {
	hwMgrRespDrives := []*api.Drive{
		{
			SerialNumber: "hdd1",
			Health:       api.Health_GOOD,
			Type:         api.DriveType_HDD,
		},
		{
			SerialNumber: "hdd2",
			Health:       api.Health_GOOD,
			Type:         api.DriveType_HDD,
		},
	}

	hwMgrClient := mocks.NewMockHWMgrClient(hwMgrRespDrives)
	executor := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: mocks.LsblkTwoDevices})
	vm := NewVolumeManager(*hwMgrClient)

	// expect that cache is empty because of all drives has not children
	vm.SetLinuxUtilsExecutor(executor)
	assert.Empty(t, vm.volumesCache)
	err := vm.Discover()
	assert.Nil(t, err)
	assert.Empty(t, vm.volumesCache)

	// expect that one volume will appear in cache
	executor.SetMap(map[string]mocks.CmdOut{base.LsblkCmd: mocks.LsblkDevWithChildren})
	vm.SetLinuxUtilsExecutor(executor)
	err = vm.Discover()
	assert.Nil(t, err)
	// LsblkDevWithChildren contains with 2 devices with children however one of them without size
	// that because we expect one item in volumes cache
	assert.Equal(t, 1, len(vm.volumesCache))

}
