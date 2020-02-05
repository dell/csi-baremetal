package base

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var executor = mocks.NewMockExecutor(mocks.DiskCommands)

var partition = &Partition{
	e: executor,
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
	assert.Equal(t, errors.New("unable to check partition existence for /dev/sdd"), err)
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

	expectedError := errors.New("partprobe failed")
	newCmd := "partprobe"
	mocks.DiskCommands[newCmd] = mocks.CmdOut{
		Stdout: "",
		Stderr: "",
		Err:    expectedError,
	}
	err = partition.CreatePartition("/dev/sde")
	assert.NotNil(t, err)
	assert.Equal(t, expectedError, err)
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
