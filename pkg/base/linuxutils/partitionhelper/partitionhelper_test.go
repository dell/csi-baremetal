package partitionhelper

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var (
	testPartitioner = NewPartition(mocks.NewMockExecutor(mocks.DiskCommands))
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
