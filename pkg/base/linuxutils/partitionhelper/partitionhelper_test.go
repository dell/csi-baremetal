/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package partitionhelper

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/mocks"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
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
	err := testPartitioner.CreatePartition("/dev/sde", testCSILabel, testPartUUID, true)
	assert.Nil(t, err)

	err = testPartitioner.CreatePartition("/dev/sde", testCSILabel, "", false)
	assert.Nil(t, err)
}

func TestCreatePartitionFail(t *testing.T) {
	err := testPartitioner.CreatePartition("/dev/sdf", testCSILabel, testPartUUID, true)
	assert.NotNil(t, err)

	err = testPartitioner.CreatePartition("/dev/sdww", testCSILabel, testPartUUID, true)
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

func TestLinuxUtils_DeviceHasPartitions(t *testing.T) {
	var (
		p            = NewWrapPartitionImpl(&command.Executor{}, testLogger)
		mockLsblk    = &mocklu.MockWrapLsblk{}
		device       = "/dev/sda"
		partUUID     = "uuid-1111"
		serialNumber = "test"
	)
	p.lsblkUtil = mockLsblk

	t.Run("Device has partition", func(t *testing.T) {
		blkDev1 := lsblk.BlockDevice{Serial: serialNumber, Children: []lsblk.BlockDevice{
			{PartUUID: partUUID, Name: ""},
		}}
		mockLsblk.On("GetBlockDevices", device).
			Return([]lsblk.BlockDevice{blkDev1}, nil).Times(1)
		hasPart, err := p.DeviceHasPartitions(device)
		assert.Nil(t, err)
		assert.True(t, hasPart)
	})

	t.Run("Device doesn't have partitions", func(t *testing.T) {
		blkDev1 := lsblk.BlockDevice{Serial: serialNumber}
		mockLsblk.On("GetBlockDevices", device).
			Return([]lsblk.BlockDevice{blkDev1}, nil).Times(1)
		hasPart, err := p.DeviceHasPartitions(device)
		assert.Nil(t, err)
		assert.False(t, hasPart)
	})

	t.Run("Command failed", func(t *testing.T) {
		mockLsblk.On("GetBlockDevices", device).
			Return(nil, errors.New("error")).Times(1)
		hasPart, err := p.DeviceHasPartitions(device)
		assert.NotNil(t, err)
		assert.False(t, hasPart)
	})
	t.Run("Bad output", func(t *testing.T) {
		mockLsblk.On("GetBlockDevices", device).
			Return(nil, nil).Times(1)
		hasPart, err := p.DeviceHasPartitions(device)
		assert.NotNil(t, err)
		assert.False(t, hasPart)
	})
}

func TestLinuxUtils_DeviceHasPartitionTable(t *testing.T) {
	var (
		e      = mocks.GoMockExecutor{}
		p      = NewWrapPartitionImpl(&e, testLogger)
		device = "/dev/sda"
	)
	t.Run("Device has partition table", func(t *testing.T) {
		e.On("RunCmd", fmt.Sprintf(DetectPartitionTableCmdTmpl, device)).
			Return("Disklabel type: gpt", "", nil).Times(1)
		hasPart, err := p.DeviceHasPartitionTable(device)
		assert.Nil(t, err)
		assert.True(t, hasPart)
	})
	t.Run("Device doesn't have partition table", func(t *testing.T) {
		e.On("RunCmd", fmt.Sprintf(DetectPartitionTableCmdTmpl, device)).
			Return(" ", "", nil).Times(1)
		hasPart, err := p.DeviceHasPartitionTable(device)
		assert.Nil(t, err)
		assert.False(t, hasPart)
	})
	t.Run("Command failed", func(t *testing.T) {
		e.On("RunCmd", fmt.Sprintf(DetectPartitionTableCmdTmpl, device)).
			Return("", "", errors.New("error")).Times(1)
		hasPart, err := p.DeviceHasPartitionTable(device)
		assert.NotNil(t, err)
		assert.False(t, hasPart)
	})
}
