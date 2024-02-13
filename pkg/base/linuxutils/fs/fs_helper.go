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
	"strconv"
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
	// losetupCmd is base cmd to operate loop devices, losetup
	losetupCmd = "losetup "
	// CheckSpaceCmdImpl cmd for getting space on the mounted FS, produce output in megabytes (--block-size=M)
	CheckSpaceCmdImpl = "df %s --output=target,avail --block-size=M" // add mounted fs part
	// MkFSCmdTmpl mkfs command template
	MkFSCmdTmpl = "mkfs.%s %s %s" // args: 1 - fs type, 2 - device/path, 3 - fs uuid option
	// XfsUUIDOption option to set uuid for mkfs.xfs
	XfsUUIDOption = "-m uuid=%s"
	// ExtUUIDOption option to set uuid for mkfs.ext3(4)
	ExtUUIDOption = "-U %s"
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
	// GetFsUUIDCmdTmpl cmd for detecting FS on device
	GetFsUUIDCmdTmpl = "lsblk %s --output UUID --noheadings"
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
	// CreateFileByDDCmdTmpl cmd for creating file with specified size by dd command
	CreateFileByDDCmdTmpl = "dd if=/dev/zero of=%s bs=1M count=%s"
	// ReadLoopBackDeviceMappingCmd cmd for reading loopback device mapping
	ReadLoopBackDeviceMappingCmd = losetupCmd + "-O NAME,BACK-FILE %s"
	// SetupLoopBackDeviceCmdTmpl cmd for loopback device setup
	SetupLoopBackDeviceCmdTmpl = losetupCmd + "-f --show %s"
	// DetachLoopBackDeviceCmdTmpl cmd for loopback device detach
	DetachLoopBackDeviceCmdTmpl = losetupCmd + "-d %s"

	// NoSuchDeviceErrMsg is the err msg in stderr of the cmd output when specified device cannot be found
	NoSuchDeviceErrMsg = "No such device"
)

// WrapFS is an interface that encapsulates operation with file systems
type WrapFS interface {
	GetFSSpace(src string) (int64, error)
	MkDir(src string) error
	MkFile(src string) error
	RmDir(src string) error
	CreateFS(fsType FileSystem, device, uuid string) error
	WipeFS(device string) error
	GetFSType(device string) (string, error)
	GetFSUUID(device string) (string, error)
	// Mount operations
	IsMounted(src string) (bool, error)
	FindMountPoint(target string) (string, error)
	Mount(src, dst string, opts ...string) error
	Unmount(src string) error
	CreateFileWithSizeInMB(filePath string, sizeMB int) error
	ReadLoopDevice(device string) (string, error)
	CreateLoopDevice(src string) (string, error)
	RemoveLoopDevice(device string) error
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
func (h *WrapFSImpl) CreateFS(fsType FileSystem, device, uuid string) error {
	var cmd string
	switch fsType {
	case XFS:
		cmd = fmt.Sprintf(MkFSCmdTmpl, fsType, device, fmt.Sprintf(XfsUUIDOption, uuid))
	case EXT3, EXT4:
		cmd = fmt.Sprintf(MkFSCmdTmpl, fsType, device, fmt.Sprintf(ExtUUIDOption, uuid)) + SpeedUpFsCreationOpts
	default:
		return fmt.Errorf("unsupported file system %v", fsType)
	}

	if _, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(MkFSCmdTmpl, "", "", "")))); err != nil {
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

	procMounts, err := util.ConsistentRead(MountInfoFile, 20, time.Millisecond)
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

// GetFSType detect FS type from the provided device using lsblk --output FSTYPE
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

// GetFSUUID detect FS UUID from the provided device using lsblk --output UUID
// Receives file path of the device as a string
// Returns error if something went wrong
func (h *WrapFSImpl) GetFSUUID(device string) (string, error) {
	var (
		cmd    = fmt.Sprintf(GetFsUUIDCmdTmpl, device)
		stdout string
		err    error
	)
	if stdout, _, err = h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(fmt.Sprintf(GetFsUUIDCmdTmpl, ""))); err != nil {
		return "", fmt.Errorf("failed to detect file system on %s: %w", device, err)
	}
	return strings.TrimSpace(stdout), err
}

// CreateFileWithSizeInMB creates file with specified size in MB unit by dd command
// Receives the file path and size in MB unit
// Returns error if something went wrong
func (h *WrapFSImpl) CreateFileWithSizeInMB(filePath string, sizeMB int) error {
	err := h.MkDir(path.Dir(filePath))
	if err != nil {
		return fmt.Errorf("failed to create parent dir of file %s: %w", filePath, err)
	}

	cmd := fmt.Sprintf(CreateFileByDDCmdTmpl, filePath, strconv.Itoa(sizeMB))
	_, _, err = h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(fmt.Sprintf(CreateFileByDDCmdTmpl, "", "")))
	if err != nil {
		return fmt.Errorf("failed to create file %s with size %d MB: %w", filePath, sizeMB, err)
	}
	return nil
}

// ReadLoopDevice get loopback device's mapped backing file
// Receives the specified device's path
// Returns the loopback device's mapped backing file or empty string and error if something went wrong
func (h *WrapFSImpl) ReadLoopDevice(device string) (string, error) {
	cmd := fmt.Sprintf(ReadLoopBackDeviceMappingCmd, device)
	returnedErrPrefix := fmt.Sprintf("failed to read loopback device %s", device)

	stdout, stderr, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(ReadLoopBackDeviceMappingCmd, ""))))

	if err != nil {
		if strings.Contains(stderr, NoSuchDeviceErrMsg) {
			return "", nil
		}
		return "", fmt.Errorf("%s: %w", returnedErrPrefix, err)
	}

	lines := strings.Split(stdout, "\n")
	if len(lines) < 2 || len(lines[1]) == 0 {
		return "", fmt.Errorf("%s: invalid command stdout", returnedErrPrefix)
	}
	dataFields := strings.SplitN(lines[1], " ", 2)
	if len(dataFields) == 1 {
		return "", nil
	}
	return strings.TrimSpace(dataFields[1]), nil
}

// CreateLoopDevice create loopback device mapped to specified src
// Receives the file path of the specified src, whether a regular file or block device
// Returns the loopback device's file path or empty string and error if something went wrong
func (h *WrapFSImpl) CreateLoopDevice(src string) (string, error) {
	cmd := fmt.Sprintf(SetupLoopBackDeviceCmdTmpl, src)
	stdout, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(SetupLoopBackDeviceCmdTmpl, ""))))
	if err != nil {
		return "", fmt.Errorf("failed to create loopback device for %s: %w", src, err)
	}
	return strings.TrimSpace(stdout), nil
}

// RemoveLoopDevice remove the specified loopback device
// Receives the loop device path
// Returns error if something went wrong
func (h *WrapFSImpl) RemoveLoopDevice(device string) error {
	cmd := fmt.Sprintf(DetachLoopBackDeviceCmdTmpl, device)
	_, _, err := h.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(DetachLoopBackDeviceCmdTmpl, ""))))
	if err != nil {
		return fmt.Errorf("failed to remove loopback device %s: %w", device, err)
	}
	return nil
}
