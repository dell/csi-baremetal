package sc

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

const (
	// MkFSCmdTmpl mkfs template
	MkFSCmdTmpl = "mkfs.xfs -f %s"
	// WipeFSCmdTmpl wipefs template
	WipeFSCmdTmpl = "wipefs -af %s"
	// MKdirCmdTmpl mkdir template
	MKdirCmdTmpl = "mkdir -p %s"
	// RMCmdTmpl rm template
	RMCmdTmpl = "rm -rf %s"
	// MountpointCmdTmpl lsblk mount point of device template
	MountpointCmdTmpl = "lsblk -d -n -o MOUNTPOINT %s"
	// MountCmdTmpl mount "from" "to" "opts" template
	MountCmdTmpl = "mount %s %s %s"
	// UnmountCmdTmpl unmount path template
	UnmountCmdTmpl = "umount %s"
	// ProcMountsFile "/proc/mounts" path
	ProcMountsFile = "/proc/mounts"
	//MountpointCmd mountpoint template
	MountpointCmd = "mountpoint %s"
	//FileSystemExistsTmpl checks file system with wipefs template
	FileSystemExistsTmpl = "wipefs %s --output TYPE"
)

// DefaultDASC is a default implementation of StorageClassImplementer interface
// for directly attached drives like HDD and SSD
type DefaultDASC struct {
	executor base.CmdExecutor
	log      *logrus.Entry
	opMutex  sync.Mutex
}

// SetLogger sets logrus logger to a DefaultDASC struct
// Receives logrus logger and component name (SSD, HDD) to use it in logrus.WithField
func (d *DefaultDASC) SetLogger(logger *logrus.Logger, componentName string) {
	d.log = logger.WithField("component", componentName)
}

// CreateFileSystem creates specified file system on the provided device using mkfs
// Receives file system as a var of FileSystem type and path of the device as a string
// Returns error if something went wrong
// TODO: do not return error here
func (d *DefaultDASC) CreateFileSystem(fsType FileSystem, device string) error {
	var cmd string
	switch fsType {
	case XFS:
		cmd = fmt.Sprintf(MkFSCmdTmpl, device)
	default:
		return errors.New("unknown file system")
	}

	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	cmdFSExist := fmt.Sprintf(FileSystemExistsTmpl, device)

	var exist bool
	/*output example
	TYPE
	ext4
	*/
	stdout, _, err := d.executor.RunCmd(cmdFSExist)
	if err != nil {
		return err
	}
	if len(stdout) != 0 {
		if !strings.Contains(stdout, string(fsType)) {
			return fmt.Errorf("partition has another fsType")
		}
		exist = true
	}

	if exist {
		d.log.Infof("File system already exist for device %s", device)
		return nil
	}
	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to create file system on %s", device)
		return err
	}
	return nil
}

// DeleteFileSystem deletes file system from the provided device using wipefs
// Receives file path of the device as a string
// Returns error if something went wrong
func (d *DefaultDASC) DeleteFileSystem(device string) error {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	if _, _, err := d.executor.RunCmd(fmt.Sprintf(WipeFSCmdTmpl, device)); err != nil {
		d.log.Errorf("failed to wipe file system on %s ", device)
		return err
	}
	return nil
}

// CreateTargetPath creates specified path using mkdir if it doesn't exist
// Receives directory path to create as a string
// Returns error if something went wrong
func (d *DefaultDASC) CreateTargetPath(path string) error {
	cmd := fmt.Sprintf(MKdirCmdTmpl, path)

	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to create target mount path %s", path)
		return err
	}
	return nil
}

// DeleteTargetPath deletes specified path using rm
// Receives directory path to delete as a string
// Returns error if something went wrong
func (d *DefaultDASC) DeleteTargetPath(path string) error {
	cmd := fmt.Sprintf(RMCmdTmpl, path)

	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to delete target mount path %s", path)
		return err
	}
	return nil
}

// IsMounted checks if the partition of device mounted
// Receives partition path as a string
// Returns bool that represents mount status or error if something went wrong
func (d *DefaultDASC) IsMounted(partition string) (bool, error) {
	ll := d.log.WithField("method", "IsMounted")

	procMounts, err := base.ConsistentRead(ProcMountsFile, 5, time.Millisecond)
	if err != nil || len(procMounts) == 0 {
		if err != nil {
			ll.Errorf("%s is not consistent, error: %v", ProcMountsFile, err)
		}
		return false, fmt.Errorf("unable to check whether %s mounted or no", partition)
	}

	// parse /proc/mounts content and search partition entry
	for _, line := range strings.Split(string(procMounts), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == partition {
			ll.Infof("There is a fstab entry for %s, entry: %s", partition, line)
			return true, nil
		}
	}

	return false, nil
}

// Mount mounts source path to the destination directory
// Receives source path and destination dir and also opts parameters that are used for mount command for example --bind
// Returns error if something went wrong
func (d *DefaultDASC) Mount(src, dir string, opts ...string) error {
	cmd := fmt.Sprintf(MountCmdTmpl, strings.Join(opts, " "), src, dir)
	d.opMutex.Lock()
	defer d.opMutex.Unlock()
	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to mount %s drive", src)
		return err
	}
	return nil
}

// Unmount unmounts device from the specified path
// Receives path where the device is mounted
// Returns error if something went wrong
func (d *DefaultDASC) Unmount(path string) error {
	cmd := fmt.Sprintf(UnmountCmdTmpl, path)

	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	if _, stderr, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Infof("%s has already unmounted", path)
		if strings.Contains(stderr, "not mounted") {
			return nil
		}
		d.log.Errorf("unable to unmount path %s", path)
		return err
	}
	return nil
}

// IsMountPoint checks if the specified path is mount point
// Receives path that should be checked
// Returns bool that is true if path is the mount point and error if something went wrong
func (d *DefaultDASC) IsMountPoint(path string) (bool, error) {
	cmd := fmt.Sprintf(MountpointCmd, path)

	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	if stdout, _, err := d.executor.RunCmd(cmd); err != nil {
		if strings.Contains(stdout, "not a mountpoint") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// PrepareVolume is a function for preparing a volume in NodePublish() call
// Receives device that the volume should be based on and a targetPath where the device should be mounted
// Returns rollBacked status (if error occurs, then we try to rollback successful steps). Returns (false, nil) if success
func (d *DefaultDASC) PrepareVolume(device, targetPath string) (rollBacked bool, err error) {
	rollBacked = true

	err = d.CreateFileSystem(XFS, device)
	if err != nil {
		return false, err
	}

	err = d.CreateTargetPath(targetPath)
	if err != nil {
		err = d.DeleteFileSystem(device)
		if err != nil {
			return false, err
		}
		return
	}

	err = d.Mount(device, targetPath)
	if err != nil {
		err = d.DeleteTargetPath(targetPath)
		if err != nil {
			return false, err
		}
		err = d.DeleteFileSystem(device)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
