package base

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	ph "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/partitionhelper"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var partition = &Partition{
	helper: ph.NewPartition(
		mocks.NewMockExecutor(mocks.DiskCommands)),
}

func TestIsPartitionExists(t *testing.T) {
	exists, _ := partition.IsPartitionExists("/dev/sda")
	assert.Equal(t, false, exists)

	exists, _ = partition.IsPartitionExists("/dev/sdb")
	assert.Equal(t, true, exists)

	exists, _ = partition.IsPartitionExists("/dev/sdc")
	assert.Equal(t, false, exists)
}

func TestIsPartitionExistsFail(t *testing.T) {
	exists, err := partition.IsPartitionExists("/dev/sdd")
	assert.Equal(t, false, exists)
	assert.NotNil(t, err)
}

func TestCreatePartitionTable(t *testing.T) {
	err := partition.CreatePartitionTable("/dev/sda")
	assert.Nil(t, err)

	err = partition.CreatePartitionTable("/dev/sdc")
	assert.Nil(t, err)
}

func TestCreatePartitionTableFail(t *testing.T) {
	err := partition.CreatePartitionTable("/dev/sdd")
	assert.NotNil(t, err)
}

func TestGetPartitionTableType(t *testing.T) {
	ptType, _ := partition.GetPartitionTableType("/dev/sdb")
	assert.Equal(t, "msdos", ptType)

	ptType, _ = partition.GetPartitionTableType("/dev/sdc")
	assert.Equal(t, "msdos", ptType)
}

func TestGetPartitionTableTypeFail(t *testing.T) {
	ptType, err := partition.GetPartitionTableType("/dev/sdqwe")
	assert.Equal(t, "", ptType)
	assert.Equal(t, errors.New("unable to get partition table for device /dev/sdqwe"), err)

	ptType, err = partition.GetPartitionTableType("/dev/sde")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to parse output")
}

func TestCreatePartition(t *testing.T) {
	err := partition.CreatePartition("/dev/sde")
	assert.Nil(t, err)
}

func TestCreatePartitionFail(t *testing.T) {
	err := partition.CreatePartition("/dev/sdf")
	assert.NotNil(t, err)

	newCmd := "partprobe /dev/sde"
	mocks.DiskCommands[newCmd] = mocks.CmdOut{
		Stdout: "",
		Stderr: "",
		Err:    errors.New("error"),
	}
	err = partition.CreatePartition("/dev/sde")
	assert.NotNil(t, err)
	assert.NotNil(t, err)
	delete(mocks.DiskCommands, newCmd)

}

func TestDeletePartition(t *testing.T) {
	err := partition.DeletePartition("/dev/sda")
	assert.Nil(t, err)
}

func TestDeletePartitionFail(t *testing.T) {
	err := partition.DeletePartition("/dev/sdb")
	assert.NotNil(t, err)
}

func TestSetPartitionUUID(t *testing.T) {
	err := partition.SetPartitionUUID("/dev/sda", "64be631b-62a5-11e9-a756-00505680d67f")
	assert.Nil(t, err)
}

func TestSetPartitionUUIDFail(t *testing.T) {
	err := partition.SetPartitionUUID("/dev/sdb", "64be631b-62a5-11e9-a756-00505680d67f")
	assert.NotNil(t, err)
}

func TestGetPartitionUUID(t *testing.T) {
	uuid, err := partition.GetPartitionUUID("/dev/sda")
	assert.Equal(t, "64be631b-62a5-11e9-a756-00505680d67f", uuid)
	assert.Nil(t, err)
}

func TestGetPartitionUUIDFail(t *testing.T) {
	uuid, err := partition.GetPartitionUUID("/dev/sdb")
	assert.Equal(t, "", uuid)
	assert.Equal(t, errors.New("unable to get partition GUID for device /dev/sdb"), err)

	uuid, err = partition.GetPartitionUUID("/dev/sdc")
	assert.NotNil(t, err)
	assert.Equal(t, "", uuid)
	assert.Equal(t, errors.New("error"), err)
}
