/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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

// Package datadiscover contains code for discovering filesystems, partitions, partition table and LVM PV on drive
package datadiscover

import (
	"fmt"

	"github.com/dell/csi-baremetal/pkg/base/linuxutils/datadiscover/types"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
)

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

// DiscoverData perform linux operation to determine if device has logical entities like filesystem on it
// It executes lsblk to find file systems and partitions, parted for partition table
// Receive device path and serial number
// Return true if device has data, false in opposite, error if something went wrong
func (w *WrapDataDiscoverImpl) DiscoverData(device, serialNumber string) (*types.DiscoverResult, error) {
	var (
		fileSystem string
		hasData    bool
		err        error
	)

	if fileSystem, err = w.fsHelper.DeviceFs(device); err != nil {
		return nil, err
	}
	if fileSystem != "" {
		return &types.DiscoverResult{
			Message: fmt.Sprintf("Drive with path %s, SN %s has filesystem %s.", device, serialNumber, fileSystem),
			HasData: true,
		}, nil
	}

	if hasData, err = w.partHelper.DeviceHasPartitionTable(device); err != nil {
		return nil, err
	}
	if hasData {
		return &types.DiscoverResult{
			Message: fmt.Sprintf("Drive with path %s, SN %s has a partition table.", device, serialNumber),
			HasData: hasData,
		}, nil
	}

	if hasData, err = w.partHelper.DeviceHasPartitions(device, serialNumber); err != nil {
		return nil, err
	}
	if hasData {
		return &types.DiscoverResult{
			Message: fmt.Sprintf("Drive with path %s, SN %s has partitions.", device, serialNumber),
			HasData: hasData,
		}, nil
	}

	return &types.DiscoverResult{
		Message: fmt.Sprintf("Drive with path %s, SN %s doesn't have filesystem, partition table, partitions or PV.", device, serialNumber),
		HasData: hasData,
	}, nil
}
