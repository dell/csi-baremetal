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

// Package fs contains code for communicating with system file system utils such as mkdri/mkfs and so on
package fs

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// FileSystem is type for storing FS string representation
type FileSystem string

const (
	// XFS file system
	XFS FileSystem = "xfs"
	// EXT4 file system
	EXT4 FileSystem = "ext4"
	// EXT3 file system
	EXT3 FileSystem = "ext3"

	// wipefs is a system utility
	wipefs = "wipefs "
	// CheckSpaceCmdImpl cmd for getting space on the mounted FS, produce output in megabytes (--block-size=M)
	CheckSpaceCmdImpl = "df %s --output=target,avail --block-size=M" // add mounted fs part
	// MkFSCmdTmpl mkfs command template
	MkFSCmdTmpl = "mkfs.%s %s" // add fs type and device/path
	// SpeedUpFsCreationOpts options that could be used for speeds up creation of ext3 and ext4 FS
	SpeedUpFsCreationOpts = " -E lazy_journal_init=1,lazy_itable_init=1,discard"
	// MkDirCmdTmpl mkdir template
	MkDirCmdTmpl = "mkdir -p %s"
	// RmDirCmdTmpl rm template
	RmDirCmdTmpl = "rm -rf %s"
	// WipeFSCmdTmpl cmd for wiping FS on device
	WipeFSCmdTmpl = wipefs + "-af %s" //
	// GetFSTypeCmdTmpl cmd for detecting FS on device
	GetFSTypeCmdTmpl = "lsblk %s --output FSTYPE --noheadings"
	// MountInfoFile "/proc/mounts" path
	MountInfoFile = "/proc/self/mountinfo"
	// FindMntCmdTmpl find source device for target mount path cmd
	FindMntCmdTmpl = "findmnt --target %s --output SOURCE --noheadings" // add target path
	// MountCmdTmpl mount cmd template, add "src" "dst" and "opts" (could be omitted)
	MountCmdTmpl = "mount %s %s %s"
	// UnmountCmdTmpl unmount path template
	UnmountCmdTmpl = "umount %s"
	// BindOption option for mount operation
	BindOption = "--bind"
	// MountOptionsFlag flag to set mount options
	MountOptionsFlag = "-o"
)

// WrapFS is an interface that encapsulates operation with file systems
type WrapFS interface {
	GetFSSpace(src string) (int64, error)
	MkDir(src string) error
	MkFile(src string) error
	RmDir(src string) error
	CreateFS(fsType FileSystem, device string) error
	WipeFS(device string) error
	GetFSType(device string) (string, error)
	// Mount operations
	IsMounted(src string) (bool, error)
	FindMountPoint(target string) (string, error)
	Mount(src, dst string, opts ...string) error
	Unmount(src string) error
}

// WrapFSImpl is a WrapFS implementer
type WrapFSImpl struct {
	e       command.CmdExecutor
	opMutex sync.Mutex
}

// NewFSImpl is a constructor for WrapFSImpl struct
func NewFSImpl(e command.CmdExecutor) *WrapFSImpl {
	return &WrapFSImpl{e: e}
}

// GetFSSpace calls df command and return available space on the provided file system (src)
// Returns free bytes as int64 or error if something went wrong
func (h *WrapFSImpl) GetFSSpace(src string) (int64, error) {
	/*
		Example of output:
			~# df /dev --output=target,avail --block-size=M
				Mounted on Avail
				/dev       7982M
	*/

	stdout, _, err := h.e.RunCmd(fmt.Sprintf(CheckSpaceCmdImpl, src),
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(CheckSpaceCmdImpl, ""))))
	if err != nil {
		return 0, err
	}
	split := strings.Split(stdout, "\n")
	// Skip headers Mounter on and Available
	for j := 1; j < len(split); j++ {
		output := strings.Split(strings.TrimSpace(split[j]), " ")
		if len(output) > 1 {
			if strings.Contains(output[0], src) && len(output[0]) == 1 {
				// Try to get size from string, e.g. "/dev       7982M"
				sizeIdx := len(output) - 1
				freeBytes, err := util.StrToBytes(output[sizeIdx])
				if err != nil {
					return 0, err
				}
				return freeBytes, nil
			}
		}
	}
	return 0, fmt.Errorf("wrong df output %s", stdout)
}

// MkDir creates specified path using mkdir if it doesn't exist
// Receives directory path to create as a string
// Returns error if something went wrong
func (h *WrapFSImpl) MkDir(src string) error {
	cmd := fmt.Sprintf(MkDirCmdTmpl, src)

	if _, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(MkDirCmdTmpl, "")))); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", src, err)
	}
	return nil
}

// MkFile create file with specified path
func (h *WrapFSImpl) MkFile(src string) error {
	st, err := os.Stat(src)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		err = h.MkDir(path.Dir(src))
		if err != nil {
			return fmt.Errorf("failed to create parrent dir")
		}
		file, err := os.OpenFile(src, os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		if err = file.Close(); err != nil {
			return fmt.Errorf("could not close file")
		}
		return nil
	}
	if st.IsDir() {
		return fmt.Errorf("existing path is a directory")
	}
	return nil
}

// RmDir removes specified path using rm
// Receives directory of file path to delete as a string
// Returns error if something went wrong
func (h *WrapFSImpl) RmDir(src string) error {
	cmd := fmt.Sprintf(RmDirCmdTmpl, src)

	if _, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(RmDirCmdTmpl, "")))); err != nil {
		return fmt.Errorf("failed to delete path %s: %w", src, err)
	}
	return nil
}

// CreateFS creates specified file system on the provided device using mkfs
// Receives file system as a var of FileSystem type and path of the device as a string
// Returns error if something went wrong
func (h *WrapFSImpl) CreateFS(fsType FileSystem, device string) error {
	var cmd string
	switch fsType {
	case XFS:
		cmd = fmt.Sprintf(MkFSCmdTmpl, fsType, device)
	case EXT3, EXT4:
		cmd = fmt.Sprintf(MkFSCmdTmpl, fsType, device) + SpeedUpFsCreationOpts
	default:
		return fmt.Errorf("unsupported file system %v", fsType)
	}

	if _, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(MkFSCmdTmpl, "", "")))); err != nil {
		return fmt.Errorf("failed to create file system on %s: %w", device, err)
	}
	return nil
}

// WipeFS deletes file system from the provided device using wipefs
// Receives file path of the device as a string
// Returns error if something went wrong
func (h *WrapFSImpl) WipeFS(device string) error {
	cmd := fmt.Sprintf(WipeFSCmdTmpl, device)

	if _, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(WipeFSCmdTmpl, "")))); err != nil {
		return fmt.Errorf("failed to wipe file system on %s: %w", device, err)
	}
	return nil
}

// IsMounted checks if the path is presented in /proc/self/mountinfo
// Receives path as a string
// Returns bool that represents mount status or error if something went wrong
func (h *WrapFSImpl) IsMounted(path string) (bool, error) {
	h.opMutex.Lock()
	defer h.opMutex.Unlock()

	procMounts, err := util.ConsistentRead(MountInfoFile, 5, time.Millisecond)
	if err != nil || len(procMounts) == 0 {
		return false, fmt.Errorf("unable to check whether %s mounted or no, error: %w", path, err)
	}

	// parse /proc/mounts content and search path entry
	for _, line := range strings.Split(string(procMounts), "\n") {
		if strings.Contains(line, path) {
			return true, nil
		}
	}

	return false, nil
}

// FindMountPoint returns source of mount point for target
// Receives path of a mount point as target
// Returns mount point or empty string and error
func (h *WrapFSImpl) FindMountPoint(target string) (string, error) {
	/*
		Example of output:
			~# findmnt --target / --output SOURCE --noheadings
			/dev/mapper/unassigned--hostname--vg-root
	*/

	h.opMutex.Lock()
	cmd := fmt.Sprintf(FindMntCmdTmpl, target)
	h.opMutex.Unlock()

	strOut, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(FindMntCmdTmpl, ""))))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strOut), nil
}

// Mount mounts source path to the destination directory
// Receives source path and destination dir and also opts parameters that are used for mount command for example --bind
// Returns error if something went wrong
func (h *WrapFSImpl) Mount(src, dir string, opts ...string) error {
	cmd := fmt.Sprintf(MountCmdTmpl, strings.Join(opts, " "), src, dir)
	h.opMutex.Lock()
	_, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(MountCmdTmpl, "", "", ""))))
	h.opMutex.Unlock()

	return err
}

// Unmount unmounts device from the specified path
// Receives path where the device is mounted
// Returns error if something went wrong
func (h *WrapFSImpl) Unmount(path string) error {
	cmd := fmt.Sprintf(UnmountCmdTmpl, path)

	h.opMutex.Lock()
	_, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(UnmountCmdTmpl, ""))))
	h.opMutex.Unlock()

	return err
}

// GetFSType detect FS from the provided device using lsblk --output FSTYPE
// Receives file path of the device as a string
// Returns error if something went wrong
func (h *WrapFSImpl) GetFSType(device string) (string, error) {
	var (
		cmd    = fmt.Sprintf(GetFSTypeCmdTmpl, device)
		stdout string
		err    error
	)
	if stdout, _, err = h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(fmt.Sprintf(GetFSTypeCmdTmpl, ""))); err != nil {
		return "", fmt.Errorf("failed to detect file system on %s: %w", device, err)
	}
	return strings.TrimSpace(stdout), err
}
