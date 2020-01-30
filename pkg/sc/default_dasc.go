package sc

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

// DefaultDASC is a default implementation of StorageClassImplementer interface
// for directly attached drives like HDD and SSD
type DefaultDASC struct {
	executor base.CmdExecutor
}

// TODO: do not return error here
func (d DefaultDASC) CreateFileSystem(fsType FileSystem, device string) (bool, error) {
	var cmd string
	switch fsType {
	case XFS:
		cmd = fmt.Sprintf("mkfs.xfs -f %s", device)
	default:
		return false, errors.New("unknown file system")
	}
	_, stderr, err := d.executor.RunCmd(cmd)
	if err != nil {
		logrus.Infof("Failed to create file system, error - %s", stderr)
		return false, err
	}
	return true, nil
}

func (d DefaultDASC) DeleteFileSystem(device string) (bool, error) {
	_, stderr, err := d.executor.RunCmd(fmt.Sprintf("wipefs -af %s", device))
	if err != nil {
		logrus.Infof("Failed to wipe file system in %s with error - %s", device, stderr)
		return false, err
	}
	return true, nil
}

func (d DefaultDASC) CreateTargetPath(path string) (bool, error) {
	_, stderr, err := d.executor.RunCmd(fmt.Sprintf("mkdir -p %s", path))
	if err != nil {
		logrus.Infof("Failed to create target mount path %s with error - %s", path, stderr)
		return false, err
	}
	return true, nil
}

func (d DefaultDASC) DeleteTargetPath(path string) (bool, error) {
	_, stderr, err := d.executor.RunCmd(fmt.Sprintf("rm -rf %s", path))
	if err != nil {
		logrus.Infof("Failed to delete target mount path %s with error - %s", path, stderr)
		return false, err
	}
	return true, nil
}

func (d DefaultDASC) IsMounted(device, targetPath string) (bool, error) {
	stdout, stderr, err := d.executor.RunCmd(fmt.Sprintf("lsblk -d -n -o MOUNTPOINT %s", device))
	if err != nil {
		logrus.Infof("Failed to check mount point of %s with error - %s", device, stderr)
		return false, err
	}

	mountPoint := strings.TrimSpace(stdout)

	logrus.Infof("Mount point for %s is %s", device, mountPoint)

	return mountPoint == targetPath, nil
}

func (d DefaultDASC) Mount(device, dir string) (bool, error) {
	_, stderr, err := d.executor.RunCmd(fmt.Sprintf("mount %s %s", device, dir))
	if err != nil {
		logrus.Infof("Failed to mount %s drive with error - %s", device, stderr)
		return false, err
	}
	return true, nil
}

func (d DefaultDASC) Unmount(path string) bool {
	_, _, err := d.executor.RunCmd(fmt.Sprintf("umount %s", path))
	if err != nil {
		logrus.Infof("Unable to unmount path %s", path)
		return false
	}
	return true
}
