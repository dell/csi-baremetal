package partitionhelper

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	mocklu "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks/linuxutils"
)

var (
	testLogger      = logrus.New()
	testPartitioner = NewWrapPartitionImpl(mocks.NewMockExecutor(mocks.DiskCommands), testLogger)
	testPartNum     = "1"
	testCSILabel    = "CSI"
	testPartUUID    = "64be631b-62a5-11e9-a756-00505680d67f"
)

func TestIsPartitionExists(t *testing.T) {
	exists, _ := testPartitioner.IsPartitionExists("/dev/sda", testPartNum)
	assert.Equal(t, false, exists)

	exists, _ = testPartitioner.IsPartitionExists("/dev/sdb", testPartNum)
	assert.Equal(t, true, exists)

	exists, _ = testPartitioner.IsPartitionExists("/dev/sdc", testPartNum)
	assert.Equal(t, false, exists)
}

func TestIsPartitionExistsFail(t *testing.T) {
	exists, err := testPartitioner.IsPartitionExists("/dev/sdd", testPartNum)
	assert.Equal(t, false, exists)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to check partition")
}

func TestCreatePartitionTable(t *testing.T) {
	err := testPartitioner.CreatePartitionTable("/dev/sda", PartitionGPT)
	assert.Nil(t, err)

	err = testPartitioner.CreatePartitionTable("/dev/sdc", PartitionGPT)
	assert.Nil(t, err)
}

func TestCreatePartitionTableFail(t *testing.T) {
	err := testPartitioner.CreatePartitionTable("/dev/sdd", PartitionGPT)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to create partition table for device")

	// unsupported partition table type
	err = testPartitioner.CreatePartitionTable("/dev/sdd", "qwerty")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unsupported partition table type")
}

func TestCreatePartition(t *testing.T) {
	err := testPartitioner.CreatePartition("/dev/sde", testCSILabel)
	assert.Nil(t, err)
}

func TestCreatePartitionFail(t *testing.T) {
	err := testPartitioner.CreatePartition("/dev/sdf", testCSILabel)
	assert.NotNil(t, err)

	err = testPartitioner.CreatePartition("/dev/sdww", testCSILabel)
	assert.NotNil(t, err)
}

func TestDeletePartition(t *testing.T) {
	err := testPartitioner.DeletePartition("/dev/sda", testPartNum)
	assert.Nil(t, err)
}

func TestDeletePartitionFail(t *testing.T) {
	err := testPartitioner.DeletePartition("/dev/sdb", testPartNum)
	assert.NotNil(t, err)
}

func TestSetPartitionUUID(t *testing.T) {
	err := testPartitioner.SetPartitionUUID("/dev/sda", testPartNum, testPartUUID)
	assert.Nil(t, err)
}

func TestSetPartitionUUIDFail(t *testing.T) {
	err := testPartitioner.SetPartitionUUID("/dev/sdb", testPartNum, testPartUUID)
	assert.NotNil(t, err)
}

func TestGetPartitionUUID(t *testing.T) {
	uuid, err := testPartitioner.GetPartitionUUID("/dev/sda", testPartNum)
	assert.Equal(t, "64be631b-62a5-11e9-a756-00505680d67f", uuid)
	assert.Nil(t, err)
}

func TestGetPartitionUUIDFail(t *testing.T) {
	uuid, err := testPartitioner.GetPartitionUUID("/dev/sdb", testPartNum)
	assert.Equal(t, "", uuid)
	assert.Equal(t, errors.New("unable to get partition GUID for device /dev/sdb"), err)

	uuid, err = testPartitioner.GetPartitionUUID("/dev/sdc", testPartNum)
	assert.NotNil(t, err)
	assert.Equal(t, "", uuid)
	assert.Equal(t, errors.New("error"), err)
}

func TestSyncPartitionTable(t *testing.T) {
	err := testPartitioner.SyncPartitionTable("/dev/sde")
	assert.Nil(t, err)
}

func TestSyncPartitionTableFail(t *testing.T) {
	err := testPartitioner.SyncPartitionTable("/dev/sdXXXX")
	assert.NotNil(t, err)
}

func TestGetPartitionTableType(t *testing.T) {
	ptType, _ := testPartitioner.GetPartitionTableType("/dev/sdb")
	assert.Equal(t, "msdos", ptType)

	ptType, _ = testPartitioner.GetPartitionTableType("/dev/sdc")
	assert.Equal(t, "msdos", ptType)
}

func TestGetPartitionTableTypeFail(t *testing.T) {
	ptType, err := testPartitioner.GetPartitionTableType("/dev/sdqwe")
	assert.Equal(t, "", ptType)
	assert.Equal(t, errors.New("unable to get partition table for device /dev/sdqwe"), err)

	ptType, err = testPartitioner.GetPartitionTableType("/dev/sde")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to parse output")
}

func TestGetPartitionNameByUUIDSuccess(t *testing.T) {
	p := NewWrapPartitionImpl(&command.Executor{}, testLogger)
	mockLsblk := &mocklu.MockWrapLsblk{}
	p.lsblkUtil = mockLsblk

	var (
		device   = "/dev/sda"
		partUUID = "uuid-1111"
		partName = "p1"
	)

	blkDev1 := lsblk.BlockDevice{Children: []lsblk.BlockDevice{
		{PartUUID: partUUID, Name: device + partName},
	}}
	lsblkResults := []lsblk.BlockDevice{blkDev1}
	mockLsblk.On("GetBlockDevices", device).Return(lsblkResults, nil)

	res, err := p.GetPartitionNameByUUID(device, partUUID)
	assert.Nil(t, err)
	assert.Equal(t, partName, res)

}

func TestGetPartitionNameByUUIDFail(t *testing.T) {
	p := NewWrapPartitionImpl(&command.Executor{}, testLogger)
	mockLsblk := &mocklu.MockWrapLsblk{}
	p.lsblkUtil = mockLsblk

	var (
		res string
		err error
	)

	// device wasn't provided
	res, err = p.GetPartitionNameByUUID("", "bla")
	assert.Equal(t, "", res)
	assert.NotNil(t, err)

	// partition UUID wasn't provided
	res, err = p.GetPartitionNameByUUID("bla", "")
	assert.Equal(t, "", res)
	assert.NotNil(t, err)

	var (
		device   = "/dev/sda"
		partUUID = "uuid-1111"
	)

	// lsblk failed
	expectedErr := errors.New("lsblk error")
	mockLsblk.On("GetBlockDevices", device).
		Return([]lsblk.BlockDevice{}, expectedErr).Times(1)
	res, err = p.GetPartitionNameByUUID(device, partUUID)
	assert.Equal(t, "", res)
	assert.Equal(t, expectedErr, err)

	// partition name not detected
	blkDev1 := lsblk.BlockDevice{Children: []lsblk.BlockDevice{
		{PartUUID: partUUID, Name: ""},
	}}
	lsblkResults := []lsblk.BlockDevice{blkDev1}
	mockLsblk.On("GetBlockDevices", device).
		Return(lsblkResults, nil).Times(1)
	res, err = p.GetPartitionNameByUUID(device, partUUID)
	assert.Equal(t, "", res)
	assert.NotNil(t, err)

	// partition with provided UUID wasn't found
	mockLsblk.On("GetBlockDevices", device).
		Return([]lsblk.BlockDevice{blkDev1}, nil).Times(1)
	res, err = p.GetPartitionNameByUUID(device, "anotherUUID")
	assert.Equal(t, "", res)
	assert.NotNil(t, err)

	// empty lsblk output
	// partition with provided UUID wasn't found
	mockLsblk.On("GetBlockDevices", device).
		Return([]lsblk.BlockDevice{}, nil).Times(1)
	res, err = p.GetPartitionNameByUUID(device, "anotherUUID")
	assert.Equal(t, "", res)
	assert.NotNil(t, err)
}
