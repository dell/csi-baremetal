package datadiscover

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
)

func Test_DiscoverData(t *testing.T) {
	var (
		device       = "/dev/sda"
		serialNumber = "test"
	)
	t.Run("Device has file system", func(t *testing.T) {
		var (
			fs           = mocklu.MockWrapFS{}
			part         = mocklu.MockWrapPartition{}
			lvm          = mocklu.MockWrapLVM{}
			discoverData = NewDataDiscover(&fs, &part, &lvm)
		)
		fs.On("DeviceFs", device).Return("xfs", nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, discoverResult.HasData)
	})
	t.Run("Device has partition table", func(t *testing.T) {
		var (
			fs           = mocklu.MockWrapFS{}
			part         = mocklu.MockWrapPartition{}
			lvm          = mocklu.MockWrapLVM{}
			discoverData = NewDataDiscover(&fs, &part, &lvm)
		)
		fs.On("DeviceFs", device).Return(" ", nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(true, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, discoverResult.HasData)
	})
	t.Run("Device has partition", func(t *testing.T) {
		var (
			fs           = mocklu.MockWrapFS{}
			part         = mocklu.MockWrapPartition{}
			lvm          = mocklu.MockWrapLVM{}
			discoverData = NewDataDiscover(&fs, &part, &lvm)
		)
		fs.On("DeviceFs", device).Return("", nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device).Return(true, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, discoverResult.HasData)

	})
	t.Run("Device is clean", func(t *testing.T) {
		var (
			fs           = mocklu.MockWrapFS{}
			part         = mocklu.MockWrapPartition{}
			lvm          = mocklu.MockWrapLVM{}
			discoverData = NewDataDiscover(&fs, &part, &lvm)
		)
		fs.On("DeviceFs", device).Return("", nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device).Return(false, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		fmt.Println(discoverResult.HasData)
		assert.False(t, discoverResult.HasData)
	})
	t.Run("FS command failed", func(t *testing.T) {
		var (
			fs           = mocklu.MockWrapFS{}
			part         = mocklu.MockWrapPartition{}
			lvm          = mocklu.MockWrapLVM{}
			discoverData = NewDataDiscover(&fs, &part, &lvm)
		)
		fs.On("DeviceFs", device).Return("", errors.New("error")).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.Nil(t, discoverResult)
	})
	t.Run("Partition table command failed", func(t *testing.T) {
		var (
			fs           = mocklu.MockWrapFS{}
			part         = mocklu.MockWrapPartition{}
			lvm          = mocklu.MockWrapLVM{}
			discoverData = NewDataDiscover(&fs, &part, &lvm)
		)
		fs.On("DeviceFs", device).Return("", nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, errors.New("error")).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.Nil(t, discoverResult)
	})
	t.Run("Partitions command failed", func(t *testing.T) {
		var (
			fs           = mocklu.MockWrapFS{}
			part         = mocklu.MockWrapPartition{}
			lvm          = mocklu.MockWrapLVM{}
			discoverData = NewDataDiscover(&fs, &part, &lvm)
		)
		fs.On("DeviceFs", device).Return("", nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device).Return(false, errors.New("error")).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.Nil(t, discoverResult)
	})
}
