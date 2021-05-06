package datadiscover

import (
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
)

// WrapDataDiscover is the interface which encapsulates method to discover data on drives
type WrapDataDiscover interface {
	DiscoverData(device, serialNumber string) (bool, error)
}

// WrapDataDiscoverImpl is the basic implementation of WrapDataDiscover interface
type WrapDataDiscoverImpl struct {
	fsHelper   fs.WrapFS
	partHelper partitionhelper.WrapPartition
	lvmHelper  lvm.WrapLVM
}

// NewDataDiscover is a constructor for WrapDataDiscoverImpl
func NewDataDiscover(fs fs.WrapFS,
	part partitionhelper.WrapPartition,
	lvm lvm.WrapLVM) *WrapDataDiscoverImpl {
	return &WrapDataDiscoverImpl{fsHelper: fs, partHelper: part, lvmHelper: lvm}
}

// DiscoverData perform linux operation to determine if device has data on it
// It executes lsblk to find file systems and partitions, parted for partition table
// Receive device path and serial number
// Return true if device has data, false in opposite, error if something went wrong
func (w *WrapDataDiscoverImpl) DiscoverData(device, serialNumber string) (bool, error) {
	var (
		hasData bool
		err     error
	)

	if hasData, err = w.fsHelper.DeviceHasFs(device); err != nil {
		return false, err
	}
	if hasData {
		return hasData, nil
	}

	if hasData, err = w.partHelper.DeviceHasPartitionTable(device); err != nil {
		return false, err
	}
	if hasData {
		return hasData, nil
	}

	if hasData, err = w.partHelper.DeviceHasPartitions(device, serialNumber); err != nil {
		return false, err
	}
	if hasData {
		return hasData, nil
	}

	if hasData, err = w.lvmHelper.DeviceHasVG(device); err != nil {
		return false, err
	}
	if hasData {
		return hasData, nil
	}
	return false, nil
}
