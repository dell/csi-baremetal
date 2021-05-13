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
		fs           = mocklu.MockWrapFS{}
		part         = mocklu.MockWrapPartition{}
		lvm          = mocklu.MockWrapLVM{}
		discoverData = NewDataDiscover(&fs, &part, &lvm)
		device       = "/dev/sda"
		serialNumber = "test"
	)
	t.Run("Device has file system", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(true, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		expectedMsg := fmt.Sprintf("Drive with path %s, SN %s has filesystem", device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, discoverResult.HasData)
		assert.Equal(t, expectedMsg, discoverResult.Message)
	})
	t.Run("Device has partition table", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(true, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		expectedMsg := fmt.Sprintf("Drive with path %s, SN %s has a partition table", device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, discoverResult.HasData)
		assert.Equal(t, expectedMsg, discoverResult.Message)
	})
	t.Run("Device has partition", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(true, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		expectedMsg := fmt.Sprintf("Drive with path %s, SN %s has partitions", device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, discoverResult.HasData)
		assert.Equal(t, expectedMsg, discoverResult.Message)

	})
	t.Run("Device has PV", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, nil).Times(1)
		lvm.On("DeviceHasPV", device).Return(true, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		expectedMsg := fmt.Sprintf("Drive with path %s, SN %s has LVM PV", device, serialNumber)
		assert.Nil(t, err)
		assert.True(t, discoverResult.HasData)
		assert.Equal(t, expectedMsg, discoverResult.Message)
	})
	t.Run("Device is clean", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, nil).Times(1)
		lvm.On("DeviceHasPV", device).Return(false, nil).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		expectedMsg := fmt.Sprintf("Drive with path %s, SN %s doesn't have filesystem, partition table, partitions and PV", device, serialNumber)
		assert.Nil(t, err)
		assert.False(t, discoverResult.HasData)
		assert.Equal(t, expectedMsg, discoverResult.Message)
	})
	t.Run("FS command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, errors.New("error")).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.Nil(t, discoverResult)
	})
	t.Run("Partition table command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, errors.New("error")).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.Nil(t, discoverResult)
	})
	t.Run("Partitions command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, errors.New("error")).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.Nil(t, discoverResult)
	})
	t.Run("VG command failed", func(t *testing.T) {
		fs.On("DeviceHasFs", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitionTable", device).Return(false, nil).Times(1)
		part.On("DeviceHasPartitions", device, serialNumber).Return(false, nil).Times(1)
		lvm.On("DeviceHasPV", device).Return(false, errors.New("error")).Times(1)
		discoverResult, err := discoverData.DiscoverData(device, serialNumber)
		assert.NotNil(t, err)
		assert.Nil(t, discoverResult)
	})
}
