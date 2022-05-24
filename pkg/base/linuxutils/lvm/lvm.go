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

// Package lvm contains code for running and interpreting output of system logical volume manager utils
// such as: pvcreate/pvremove, vgcreate/vgremove, lvcreate/lvremove
package lvm

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/util"
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
	// PVsListCmdTmpl print all PVs name on node
	PVsListCmdTmpl = lvmPath + "pvdisplay --short"
	// VGCreateCmdTmpl create VG on provided PVs cmd
	VGCreateCmdTmpl = lvmPath + "vgcreate --yes %s %s" // add VG name and PV names
	// VGScanCmdTmpl searches for all VGs
	VGScanCmdTmpl = lvmPath + "vgscan"
	// VGRefreshCmdTmpl reactivates an LV using the latest metadata
	VGRefreshCmdTmpl = lvmPath + "vgchange --refresh %s"
	// VGRemoveCmdTmpl remove VG cmd
	VGRemoveCmdTmpl = lvmPath + "vgremove --yes %s" // add VG name
	// AllPVsCmd returns all physical volumes on the system
	AllPVsCmd = lvmPath + "pvs --options pv_name --noheadings"
	// VGFreeSpaceCmdTmpl check VG free space cmd
	VGFreeSpaceCmdTmpl = "vgs %s --options vg_free --units b --noheadings" // add VG name
	// LVCreateCmdTmpl create LV on provided VG cmd
	LVCreateCmdTmpl = lvmPath + "lvcreate --yes --name %s --size %s %s" // add LV name, size and VG name
	// LVRemoveCmdTmpl remove LV cmd
	LVRemoveCmdTmpl = lvmPath + "lvremove --yes %s" // add full LV name
	// LVsInVGCmdTmpl print LVs in VG cmd
	LVsInVGCmdTmpl = lvmPath + "lvs --select vg_name=%s -o lv_name --noheadings" // add VG name
	// PVInfoCmdTmpl returns colon (:) separated output, where pv name on first place and vg on second
	PVInfoCmdTmpl = lvmPath + "pvdisplay %s --colon" // add PV name
	// LVExpandCmdTmpl expand LV
	LVExpandCmdTmpl = lvmPath + "lvextend --size %sb --resizefs %s" // add full LV name
	// timeoutBetweenAttempts used for RunCmdWithAttempts as a timeout between calling lvremove
	timeoutBetweenAttempts = 500 * time.Millisecond
)

// WrapLVM is an interface that encapsulates operation with system logical volume manager (/sbin/lvm)
type WrapLVM interface {
	PVCreate(dev string) error
	PVRemove(name string) error
	VGCreate(name string, pvs ...string) error
	VGScan(name string) (bool, error)
	VGReactivate(name string) error
	VGRemove(name string) error
	LVCreate(name, size, vgName string) error
	LVRemove(fullLVName string) error
	IsVGContainsLVs(vgName string) bool
	RemoveOrphanPVs() error
	GetVgFreeSpace(vgName string) (int64, error)
	GetAllPVs() ([]string, error)
	GetLVsInVG(vgName string) ([]string, error)
	GetVGNameByPVName(pvName string) (string, error)
	ExpandLV(lvName string, requiredSize int64) error
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
	_, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(PVCreateCmdTmpl, ""))))
	return err
}

// PVRemove removes physical volumes, ignore error if PV doesn't exist
// Receives name of a physical volume to delete
// Returns error if something went wrong
func (l *LVM) PVRemove(name string) error {
	cmd := fmt.Sprintf(PVRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(PVRemoveCmdTmpl, ""))))
	if err != nil && strings.Contains(stdErr, "No PV label found") {
		return nil
	}
	return err
}

// ExpandLV expand logical volume
// Receives full name of a logical volume and requiredSize to resize
// Returns error if something went wrong
func (l *LVM) ExpandLV(lvName string, requiredSize int64) error {
	cmd := fmt.Sprintf(LVExpandCmdTmpl, strconv.FormatInt(requiredSize, 10), lvName)
	_, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(LVExpandCmdTmpl, "", ""))))
	if err != nil {
		return err
	}
	return nil
}

// VGCreate creates volume group and based on provided physical volumes (pvs). Ignore error if VG already exists
// Receives name of VG to create and names of physical volumes which VG should based on
// Returns error if something went wrong
func (l *LVM) VGCreate(name string, pvs ...string) error {
	cmd := fmt.Sprintf(VGCreateCmdTmpl, name, strings.Join(pvs, " "))
	_, stdErr, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(VGCreateCmdTmpl, "", ""))))
	if err != nil && strings.Contains(stdErr, "already exists") {
		return nil
	}
	return err
}

// VGScan scans for all VGs and checks for IO errors for specific volume group name
// Receives name of VG name to scan and check
// Return boolean (false if no IO errors detected, true otherwise) and error if command failed to execute
func (l *LVM) VGScan(name string) (bool, error) {
	// scan and check for VG errors
	var (
		stdout  string
		stderr  string
		err     error
		exp     *regexp.Regexp
		ioError = "input/output error"
	)
	// do the scan
	if stdout, stderr, err = l.e.RunCmd(VGScanCmdTmpl, command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(VGScanCmdTmpl))); err != nil {
		return false, err
	}
	// empty stdout is not expected. It must also contain VG name
	if stdout == "" || !strings.Contains(stdout, name) {
		return false, errTypes.ErrorNotFound
	}
	// find target volume group and check for IO errors
	if exp, err = regexp.Compile(".*" + name + ".*\n*"); err != nil {
		return false, err
	}
	lines := exp.FindAllString(stderr, -1)
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), ioError) {
			return true, nil
		}
	}

	return false, nil
}

// VGReactivate inactivates, scans and activates volume group to recover from disk missing scenario
// Receives name of VG to re-activate
// Returns error if something went wrong
func (l *LVM) VGReactivate(name string) error {
	// re-activate related LVs
	if _, _, err := l.e.RunCmd(fmt.Sprintf(VGRefreshCmdTmpl, name), command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(VGRefreshCmdTmpl, "")))); err != nil {
		return err
	}

	return nil
}

// VGRemove removes volume group, ignore error if VG doesn't exist
// Receives name of VG to remove
// Returns error if something went wrong
func (l *LVM) VGRemove(name string) error {
	cmd := fmt.Sprintf(VGRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(VGRemoveCmdTmpl, ""))))
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
	_, stdErr, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(LVCreateCmdTmpl, "", "", ""))))
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
	_, stdErr, err := l.e.RunCmdWithAttempts(cmd, 5, timeoutBetweenAttempts, command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(LVRemoveCmdTmpl, ""))))
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
	stdout, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(LVsInVGCmdTmpl, ""))))
	if err != nil {
		l.log.WithField("method", "IsVGContainsLVs").
			Errorf("Unable to check whether VG %s contains LVs or no. Assume that - yes.", vgName)
		return true
	}

	return len(strings.TrimSpace(stdout)) > 0
}

// GetLVsInVG collects LVs for given volume group
// Receives Volume Group name
// Returns slice of found logical volumes
func (l *LVM) GetLVsInVG(vgName string) ([]string, error) {
	cmd := fmt.Sprintf(LVsInVGCmdTmpl, vgName)
	stdout, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(LVsInVGCmdTmpl, ""))))
	if err != nil {
		return nil, err
	}

	return util.SplitAndTrimSpace(stdout, "\n"), nil
}

// RemoveOrphanPVs removes PVs that do not have VG
// Returns error if something went wrong
func (l *LVM) RemoveOrphanPVs() error {
	pvsCmd := fmt.Sprintf(PVsInVGCmdTmpl, EmptyName)
	stdout, _, err := l.e.RunCmd(pvsCmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(PVsInVGCmdTmpl, ""))))
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
	strOut, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(VGFreeSpaceCmdTmpl, ""))))
	if err != nil {
		return -1, err
	}

	bytes, err := util.StrToBytes(strings.TrimSpace(strOut))
	if err != nil {
		return -1, err
	}

	return bytes, nil
}

// GetAllPVs returns slice with names of all physical volumes in the system
func (l *LVM) GetAllPVs() ([]string, error) {
	stdOut, _, err := l.e.RunCmd(AllPVsCmd,
		command.UseMetrics(true),
		command.CmdName(AllPVsCmd))
	if err != nil {
		return nil, err
	}

	return util.SplitAndTrimSpace(stdOut, "\n"), nil
}

// GetVGNameByPVName finds out volume group name based on physical volume name
func (l *LVM) GetVGNameByPVName(pvName string) (string, error) {
	cmd := fmt.Sprintf(PVInfoCmdTmpl, pvName)

	stdOut, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(PVInfoCmdTmpl, ""))))
	if err != nil {
		return "", err
	}

	// stdOut will have one of two formats:
	// if PV is in some VG format will be:
	//			/dev/sdy2:root-vg:936701952:-1:8:8:-1:4096:114343:77478:36865:H3rxE6-2iAg-1REQ-rOeX-7bz3-iPrh-YBXgxN
	// PV name on first place and VG on second
	// if PV is orphan (VG for which PV was corresponding was removed) format will be:
	// 		"/dev/sda" is a new physical volume of "<7.28 TiB"
	//  	/dev/sda::15628053168:-1:0:0:-1:0:0:0:0:2DdQcG-u5gq-mqa0-awmS-EsDP-GQFB-sfPHTw
	// parse stdOut:
	trimmed := strings.TrimSpace(stdOut)
	if len(strings.Split(trimmed, "\n")) > 1 {
		return "", fmt.Errorf("PV %s isn't related to any VG", pvName)
	}

	splitted := strings.Split(trimmed, ":")
	if len(splitted) < 2 {
		return "", fmt.Errorf("unable to find VG name for PV %s in output %s: ", pvName, trimmed)
	}

	return splitted[1], nil
}
