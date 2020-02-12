package sc

import (
	"errors"
	"fmt"
	"strings"

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
	UnmountCmdTmpl    = "umount %s"
)

// DefaultDASC is a default implementation of StorageClassImplementer interface
// for directly attached drives like HDD and SSD
type DefaultDASC struct {
	executor base.CmdExecutor
	log      *logrus.Entry
}

func (d *DefaultDASC) SetLogger(logger *logrus.Logger, componentName string) {
	d.log = logger.WithField("component", componentName)
}

// TODO: do not return error here
func (d DefaultDASC) CreateFileSystem(fsType FileSystem, device string) error {
	var cmd string
	switch fsType {
	case XFS:
		cmd = fmt.Sprintf(MkFSCmdTmpl, device)
	default:
		return errors.New("unknown file system")
	}

	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to create file system on %s", device)
		return err
	}
	return nil
}

func (d DefaultDASC) DeleteFileSystem(device string) error {
	if _, _, err := d.executor.RunCmd(fmt.Sprintf(WipeFSCmdTmpl, device)); err != nil {
		d.log.Errorf("failed to wipe file system on %s ", device)
		return err
	}
	return nil
}

func (d DefaultDASC) CreateTargetPath(path string) error {
	cmd := fmt.Sprintf(MKdirCmdTmpl, path)
	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to create target mount path %s", path)
		return err
	}
	return nil
}

func (d DefaultDASC) DeleteTargetPath(path string) error {
	cmd := fmt.Sprintf(RMCmdTmpl, path)
	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to delete target mount path %s", path)
		return err
	}
	return nil
}

func (d DefaultDASC) IsMounted(device, targetPath string) (bool, error) {
	cmd := fmt.Sprintf(MountpointCmdTmpl, device)
	stdout, _, err := d.executor.RunCmd(cmd)
	if err != nil {
		d.log.Errorf("failed to check mount point of %s", device)
		return false, err
	}

	mountPoint := strings.TrimSpace(stdout)

	d.log.Infof("mount point for %s is %s", device, mountPoint)

	return mountPoint == targetPath, nil
}

func (d DefaultDASC) Mount(device, dir string) error {
	cmd := fmt.Sprintf(MountCmdTmpl, device, dir)
	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("failed to mount %s drive", device)
		return err
	}
	return nil
}

func (d DefaultDASC) Unmount(path string) error {
	cmd := fmt.Sprintf(UnmountCmdTmpl, path)
	if _, _, err := d.executor.RunCmd(cmd); err != nil {
		d.log.Errorf("unable to unmount path %s", path)
		return err
	}
	return nil
}

// PrepareVolume is a function for preparing a volume in NodePublish() call
// if error occurs, then we try to rollback successful steps
func (d DefaultDASC) PrepareVolume(device, targetPath string) (rollBacked bool, err error) {
	rollBacked = true

	mounted, err := d.IsMounted(device, targetPath)

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
