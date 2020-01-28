package base

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var executor = mocks.NewMockExecutor(mocks.DiskCommands)

var partition = &Partition{
	Executor: executor,
}

func TestIsPartitionExists(t *testing.T) {
	exists, _ := partition.IsPartitionExists("/dev/sda")
	assert.Equal(t, exists, false)

	exists, _ = partition.IsPartitionExists("/dev/sdb")
	assert.Equal(t, exists, true)

	exists, _ = partition.IsPartitionExists("/dev/sdc")
	assert.Equal(t, exists, false)
}

func TestIsPartitionExistsFail(t *testing.T) {
	exists, err := partition.IsPartitionExists("/dev/sdd")
	assert.Equal(t, exists, false)
	assert.Equal(t, err, errors.New("unable to check partition existence for /dev/sdd"))
}

func TestCreatePartitionTable(t *testing.T) {
	err := partition.CreatePartitionTable("/dev/sda")
	assert.Equal(t, err, nil)

	err = partition.CreatePartitionTable("/dev/sdc")
	assert.Equal(t, err, nil)
}

func TestCreatePartitionTableFail(t *testing.T) {
	err := partition.CreatePartitionTable("/dev/sdd")
	assert.Equal(t, err, err)
}

func TestGetPartitionTableType(t *testing.T) {
	ptType, _ := partition.GetPartitionTableType("/dev/sdb")
	assert.Equal(t, ptType, "msdos")

	ptType, _ = partition.GetPartitionTableType("/dev/sdc")
	assert.Equal(t, ptType, "msdos")
}

func TestGetPartitionTableTypeFail(t *testing.T) {
	ptType, err := partition.GetPartitionTableType("/dev/sdqwe")

	assert.Equal(t, ptType, "")
	assert.Equal(t, err, errors.New("unable to get partition table"))
}

func TestCreatePartitionType(t *testing.T) {
	err := partition.CreatePartition("/dev/sde")
	assert.Equal(t, err, nil)
}

func TestCreatePartitionTypeFail(t *testing.T) {
	err := partition.CreatePartition("/dev/sdf")
	assert.Equal(t, err, err)
}

func TestDeletePartition(t *testing.T) {
	err := partition.DeletePartition("/dev/sda")
	assert.Equal(t, err, nil)
}

func TestDeletePartitionFail(t *testing.T) {
	err := partition.DeletePartition("/dev/sdb")
	assert.Equal(t, err, err)
}

func TestSetPartitionUUID(t *testing.T) {
	err := partition.SetPartitionUUID("/dev/sda", "64be631b-62a5-11e9-a756-00505680d67f")
	assert.Equal(t, err, nil)
}

func TestSetPartitionUUIDFail(t *testing.T) {
	err := partition.SetPartitionUUID("/dev/sdb", "64be631b-62a5-11e9-a756-00505680d67f")
	assert.Equal(t, err, err)
}

func TestGetPartitionUUID(t *testing.T) {
	uuid, err := partition.GetPartitionUUID("/dev/sda")
	assert.Equal(t, uuid, "64be631b-62a5-11e9-a756-00505680d67f")
	assert.Equal(t, err, nil)
}

func TestGetPartitionUUIDFail(t *testing.T) {
	uuid, err := partition.GetPartitionUUID("/dev/sdc")
	assert.Equal(t, uuid, "")
	assert.Equal(t, err, err)

	uuid, err = partition.GetPartitionUUID("/dev/sdb")

	assert.Equal(t, uuid, "")
	assert.Equal(t, err, errors.New("unable to get partition GUID"))
}
