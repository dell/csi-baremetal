package datadiscover

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
)

func Test_DiscoverData(t *testing.T) {
	var (
		fs           = mocklu.MockWrapFS{}
		part         = mocklu.MockWrapPartition{}
		lvm          = mocklu.MockWrapLVM{}
		discoverData = NewDataDiscover(&fs, &part, &lvm)
		device       = "/dev/sda"
		serialNumber = "test"
	)
	t.Run("Device has file system", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(true, nil).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, hasData)
	})
	t.Run("Device has partition table", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(true, nil).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, hasData)
	})
	t.Run("Device has partition", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(true, nil).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, hasData)

	})
	t.Run("Device has VG", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, nil).Times(1)
		lvm.On("DeviceHasVG", device).Return(true, nil).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, hasData)
	})
	t.Run("Device is clean", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, nil).Times(1)
		lvm.On("DeviceHasVG", device).Return(false, nil).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.Nil(t, err)
		assert.False(t, hasData)
	})
	t.Run("FS command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, errors.New("error")).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.False(t, hasData)
	})
	t.Run("Partition table command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, errors.New("error")).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.False(t, hasData)
	})
	t.Run("Partitions command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, errors.New("error")).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.False(t, hasData)
	})
	t.Run("VG command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, nil).Times(1)
		lvm.On("DeviceHasVG", device).Return(false, errors.New("error")).Times(1)
		hasData, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.False(t, hasData)
	})
}
