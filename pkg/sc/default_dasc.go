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
	MkFSCmdTmpl       = "mkfs.xfs -f %s"
	WipeFSCmdTmpl     = "wipefs -af %s"
	MKdirCmdTmpl      = "mkdir -p %s"
	RMCmdTmpl         = "rm -rf %s"
	MountpointCmdTmpl = "lsblk -d -n -o MOUNTPOINT %s"
	MountCmdTmpl      = "mount %s %s"
	BindMountCmdTmpl  = "mount -B %s %s"
	UnmountCmdTmpl    = "umount %s"
	ProcMountsFile    = "/proc/mounts"
	MountpointCmd     = "mountpoint %s"
)

// DefaultDASC is a default implementation of StorageClassImplementer interface
// for directly attached drives like HDD and SSD
type DefaultDASC struct {
	executor base.CmdExecutor
	log      *logrus.Entry
	opMutex  sync.Mutex
}

func (d *DefaultDASC) SetLogger(logger *logrus.Logger, componentName string) {
	d.log = logger.WithField("component", componentName)
}

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

	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to create file system on %s", device)
		return err
	}
	return nil
}

func (d *DefaultDASC) DeleteFileSystem(device string) error {
	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	if _, _, err := d.executor.RunCmd(fmt.Sprintf(WipeFSCmdTmpl, device)); err != nil {
		d.log.Errorf("failed to wipe file system on %s ", device)
		return err
	}
	return nil
}

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

func (d *DefaultDASC) BindMount(device, dir string, mountDevice bool) error {
	cmd := fmt.Sprintf(MountCmdTmpl, device, dir)
	if !mountDevice {
		cmd = fmt.Sprintf(BindMountCmdTmpl, device, dir)
	}
	d.opMutex.Lock()
	defer d.opMutex.Unlock()

	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to mount %s drive", device)
		return err
	}
	return nil
}

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
// if error occurs, then we try to rollback successful steps
func (d *DefaultDASC) PrepareVolume(device, targetPath string) (rollBacked bool, err error) {
	rollBacked = true

	mounted, err := d.IsMounted(device)

	if err != nil || mounted {
		// return true since mount is not successful and device should be ok after
		// since PVC has own unique target path, mounted means that everything is ok
		return
	}

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

	err = d.BindMount(device, targetPath, true)
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
