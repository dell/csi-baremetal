package datadiscover

import (
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
)

// WrapDataDiscover is the interface which encapsulates method to discover data on drives
type WrapDataDiscover interface {
	DiscoverData(device, serialNumber string) (bool, error)
}

// WrapDataDiscoverImpl is the basic implementation of WrapDataDiscover interface
type WrapDataDiscoverImpl struct {
	e          command.CmdExecutor
	lsblkHlper lsblk.WrapLsblk
	fsHelper   fs.WrapFS
	partHelper partitionhelper.WrapPartition
	lvmHelper  lvm.WrapLVM
}

func (w *WrapDataDiscoverImpl) DiscoverData(device, serialNumber string) (bool, error) {
	var (
		data bool
		err  error
	)
	if data, err = w.fsHelper.IsContainFs(device); err != nil {
		return false, err
	}
	if data == true {
		return data, err
	}

	if data, err = w.partHelper.IsContainPartitionTable(device); err != nil {
		return false, err
	}
	if data == true {
		return data, err
	}

	if data, err = w.partHelper.IsContainPartition(device, serialNumber); err != nil {
		return false, err
	}
	if data == true {
		return data, err
	}

	if data, err = w.lvmHelper.IsDevicePV(device); err != nil {
		return false, err
	}
	if data == true {
		return data, err
	}
	return false, nil
}

type Builder struct {
	e     command.CmdExecutor
	lsblk lsblk.WrapLsblk
	fs    fs.WrapFS
	part  partitionhelper.WrapPartition
	lvm   lvm.WrapLVM
}

func (b *Builder) WithExecutor(e command.CmdExecutor) {
	b.e = e
}

func (b *Builder) WithLsblk(lsblk lsblk.WrapLsblk) {
	b.lsblk = lsblk
}

func (b *Builder) WithFsHelper(fs fs.WrapFS) {
	b.fs = fs
}

func (b *Builder) WithPartHelper(part partitionhelper.WrapPartition) {
	b.part = part
}

func (b *Builder) WithLVM(lvm lvm.WrapLVM) {
	b.lvm = lvm
}

func (b *Builder) Build() WrapDataDiscover {
	dataDiscover := WrapDataDiscoverImpl{
		e:          b.e,
		lsblkHlper: b.lsblk,
		fsHelper:   b.fs,
		partHelper: b.part,
		lvmHelper:  b.lvm,
	}
	return &dataDiscover
}
