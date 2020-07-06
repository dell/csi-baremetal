// Package lvm contains code for running and interpreting output of system logical volume manager utils
// such as: pvcreate/pvremove, vgcreate/vgremove, lvcreate/lvremove
package lvm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
)

const (
	// EmptyName represents empty name for PV/LV/VG
	EmptyName = " "
	// lvmPath is a path in the system to the lvm util
	lvmPath = "/sbin/lvm "
	// PVCreateCmdTmpl create PV cmd
	PVCreateCmdTmpl = lvmPath + "pvcreate --yes %s" // add PV name
	// PVRemoveCmdTmpl remove PV cmd
	PVRemoveCmdTmpl = lvmPath + "pvremove --yes %s" // add PV name
	// PVsInVGCmdTmpl print PVs in VG cmd
	PVsInVGCmdTmpl = lvmPath + "pvs --select vg_name=%s -o pv_name --noheadings" // add VG name
	// VGCreateCmdTmpl create VG on provided PVs cmd
	VGCreateCmdTmpl = lvmPath + "vgcreate --yes %s %s" // add VG name and PV names
	// VGRemoveCmdTmpl remove VG cmd
	VGRemoveCmdTmpl = lvmPath + "vgremove --yes %s" // add VG name
	// VGByLVCmdTmpl find VG by LV cmd
	VGByLVCmdTmpl = lvmPath + "lvs %s --options vg_name --noheadings" // add LV name
	// VGFreeSpaceCmdTmpl check VG free space cmd
	VGFreeSpaceCmdTmpl = "vgs %s --options vg_free --units b --noheadings" // add VG name
	// LVCreateCmdTmpl create LV on provided VG cmd
	LVCreateCmdTmpl = lvmPath + "lvcreate --yes --name %s --size %s %s" // add LV name, size and VG name
	// LVRemoveCmdTmpl remove LV cmd
	LVRemoveCmdTmpl = lvmPath + "lvremove --yes %s" // add full LV name
	// LVsInVGCmdTmpl print LVs in VG cmd
	LVsInVGCmdTmpl = lvmPath + "lvs --select vg_name=%s -o lv_name --noheadings" // add VG name
)

// WrapLVM is an interface that encapsulates operation with system logical volume manager (/sbin/lvm)
type WrapLVM interface {
	PVCreate(dev string) error
	PVRemove(name string) error
	VGCreate(name string, pvs ...string) error
	VGRemove(name string) error
	LVCreate(name, size, vgName string) error
	LVRemove(fullLVName string) error
	IsVGContainsLVs(vgName string) bool
	RemoveOrphanPVs() error
	FindVgNameByLvNameIfExists(lvName string) (string, error)
	GetVgFreeSpace(vgName string) (int64, error)
}

// LVM is an implementation of WrapLVM interface and is a wrap for system /sbin/lvm util in
type LVM struct {
	e   command.CmdExecutor
	log *logrus.Entry
}

// NewLVM is a constructor for LVM struct
func NewLVM(e command.CmdExecutor, l *logrus.Logger) *LVM {
	return &LVM{
		e:   e,
		log: l.WithField("component", "LVM"),
	}
}

// PVCreate creates physical volume based on provided device or partition
// Receives device path
// Returns error if something went wrong
func (l *LVM) PVCreate(dev string) error {
	cmd := fmt.Sprintf(PVCreateCmdTmpl, dev)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// PVRemove removes physical volumes, ignore error if PV doesn't exist
// Receives name of a physical volume to delete
// Returns error if something went wrong
func (l *LVM) PVRemove(name string) error {
	cmd := fmt.Sprintf(PVRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "No PV label found") {
		return nil
	}
	return err
}

// VGCreate creates volume group and based on provided physical volumes (pvs). Ignore error if VG already exists
// Receives name of VG to create and names of physical volumes which VG should based on
// Returns error if something went wrong
func (l *LVM) VGCreate(name string, pvs ...string) error {
	cmd := fmt.Sprintf(VGCreateCmdTmpl, name, strings.Join(pvs, " "))
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "already exists") {
		return nil
	}
	return err
}

// VGRemove removes volume group, ignore error if VG doesn't exist
// Receives name of VG to remove
// Returns error if something went wrong
func (l *LVM) VGRemove(name string) error {
	cmd := fmt.Sprintf(VGRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd)
	if strings.Contains(stdErr, "not found") {
		return nil
	}
	return err
}

// LVCreate created logical volume in volume group, ignore error if LV already exists
// Receives name of created LV, size which is a string like 1.2G, 100M and name of VG which LV should be based on
// Returns error if something went wrong
func (l *LVM) LVCreate(name, size, vgName string) error {
	cmd := fmt.Sprintf(LVCreateCmdTmpl, name, size, vgName)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "already exists") {
		return nil
	}
	return err
}

// LVRemove removes logical volume, ignore error if LV doesn't exist
// Receives fullLVName that is a path to LV
// Returns error if something went wrong
func (l *LVM) LVRemove(fullLVName string) error {
	cmd := fmt.Sprintf(LVRemoveCmdTmpl, fullLVName)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "Failed to find logical volume") {
		return nil
	}
	return err
}

// IsVGContainsLVs checks whether VG vgName contains any LVs or no
// Receives Volume Group name to check
// Returns true in case of error to prevent mistaken VG remove
func (l *LVM) IsVGContainsLVs(vgName string) bool {
	cmd := fmt.Sprintf(LVsInVGCmdTmpl, vgName)
	stdout, _, err := l.e.RunCmd(cmd)
	if err != nil {
		l.log.WithField("method", "IsVGContainsLVs").
			Errorf("Unable to check whether VG %s contains LVs or no. Assume that - yes.", vgName)
		return true
	}
	res := len(strings.TrimSpace(stdout)) > 0
	return res
}

// RemoveOrphanPVs removes PVs that do not have VG
// Returns error if something went wrong
func (l *LVM) RemoveOrphanPVs() error {
	pvsCmd := fmt.Sprintf(PVsInVGCmdTmpl, EmptyName)
	stdout, _, err := l.e.RunCmd(pvsCmd)
	if err != nil {
		return err
	}
	var wasError bool
	for _, pv := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if len(pv) == 0 {
			continue
		}
		if err := l.PVRemove(pv); err != nil {
			l.log.WithField("method", "RemoveOrphanPVs").Errorf("Unable to remove pv %s: %v", pv, err)
			wasError = true
		}
	}
	if wasError {
		return errors.New("not all PVs were removed")
	}
	return nil
}

// FindVgNameByLvNameIfExists search VG name by LV name, LV name should be full, if LVG doesn't exist, return empty string
// Receives LV name to find its VG
// Returns VG name or empty string and error
func (l *LVM) FindVgNameByLvNameIfExists(lvName string) (string, error) {
	/*
		Example of output:
		root@provo-goop:~# lvs /dev/mapper/unassigned--hostname--vg-root --options vg_name --noheadings
			  unassigned-hostname-vg
	*/
	cmd := fmt.Sprintf(VGByLVCmdTmpl, lvName)
	strOut, stdErr, err := l.e.RunCmd(cmd)
	for _, s := range strings.Split(lvName, "/") {
		if strings.Contains(stdErr, fmt.Sprintf("Volume group \"%s\" not found", s)) {
			return "", nil
		}
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strOut), nil
}

// GetVgFreeSpace returns VG free space in bytes
// Receives VG name to count ints free space
// Returns -1 in case of error and error
func (l *LVM) GetVgFreeSpace(vgName string) (int64, error) {
	/*
		Example of output:
		root@provo-goop:~# vgs --options vg_free unassigned-hostname-vg --units b --nosuffix --noheadings
			      0
	*/

	if vgName == "" {
		return -1, errors.New("VG name shouldn't be an empty string")
	}

	cmd := fmt.Sprintf(VGFreeSpaceCmdTmpl, vgName)
	strOut, _, err := l.e.RunCmd(cmd)
	if err != nil {
		return -1, err
	}

	bytes, err := util.StrToBytes(strings.TrimSpace(strOut))
	if err != nil {
		return -1, err
	}

	return bytes, nil
}
