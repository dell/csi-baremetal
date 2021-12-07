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

package utilwrappers

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
)

// FSOperations is holds idempotent methods that consists of WrapFS methods
type FSOperations interface {
	// PrepareAndPerformMount composite methods which is prepare source and destination directories
	// and performs mount operation from src to dst
	PrepareAndPerformMount(src, dst string, bindMount, dstIsDir bool, mountOptions ...string) error
	// MountFakeTmpfs does attach of a temporary folder on failure
	MountFakeTmpfs(volumeID, dst string) error
	// UnmountWithCheck unmount operation
	UnmountWithCheck(path string) error
	// CreateFSIfNotExist checks FS and creates one if not exist
	CreateFSIfNotExist(fsType fs.FileSystem, device string) error
	fs.WrapFS
}

// FSOperationsImpl is a base implementation for FSOperation interface
type FSOperationsImpl struct {
	fs.WrapFS
	log *logrus.Entry
}

// NewFSOperationsImpl constructor for FSOperationsImpl and returns pointer on it
func NewFSOperationsImpl(e command.CmdExecutor, log *logrus.Logger) *FSOperationsImpl {
	return &FSOperationsImpl{
		WrapFS: fs.NewFSImpl(e),
		log:    log.WithField("component", "FSOperations"),
	}
}

// PrepareAndPerformMount (idempotent) implementation of FSOperations method
// create (if isn't exist) dst folder on node and perform mount from src to dst
// if bindMount set to true - mount operation will contain "--bind" option
// if error occurs and dst has created during current method call then dst will be removed
func (fsOp *FSOperationsImpl) PrepareAndPerformMount(src, dst string, bindMount, dstIsDir bool, mountOptions ...string) error {
	ll := fsOp.log.WithFields(logrus.Fields{
		"method": "PrepareAndPerformMount",
	})
	ll.Infof("Processing for source %s, destination %s", src, dst)

	// check whether dst path exist or no, if yes - assume that it is not a first provision for volume
	wasCreated := false
	_, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		createCMD := fsOp.MkDir
		if !dstIsDir {
			createCMD = fsOp.MkFile
		}
		if err = createCMD(dst); err != nil {
			return err
		}
		wasCreated = true // if something went wrong we will remove path that had created based on that flag
	}

	// dst folder is exist, check whether it is a mount point
	if !wasCreated {
		alreadyMounted, err := fsOp.IsMounted(dst)
		if err != nil {
			_ = fsOp.RmDir(dst)
			return fmt.Errorf("unable to determine whether %s is a mountpoint or no: %v", dst, err)
		}
		if alreadyMounted {
			ll.Infof("%s has already mounted to %s", src, dst)
			return nil
		}
	}

	var bindOpt string
	if bindMount {
		bindOpt = fs.BindOption
	}

	strMountOptions := addMountOptions(mountOptions)
	if err := fsOp.Mount(src, dst, bindOpt, strMountOptions); err != nil {
		if wasCreated {
			_ = fsOp.RmDir(dst)
		}

		if srcInfo, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				ll.Debugf("src path (%s) is not exists", src)
			} else {
				ll.Warnf("failed to get src %s stat: %s", src, err)
			}
		} else {
			ll.Debugf("Stat of src with failed mount: %s", srcInfo)
		}

		isSrcMounted, err := fsOp.IsMounted(src)
		if err != nil {
			ll.Warnf("failed to execute isMount: %s", err)
		}
		if !isSrcMounted {
			ll.Debugf("Src %s is not mounted", src)
		} else {
			if srcMount, err := fsOp.FindMountPoint(src); err != nil {
				ll.Warnf("failed to find mountPoint for src %s: %s", src, err)
			} else {
				ll.Debugf("Src mount point: %s", srcMount)
				if spaceOnMountPoint, err := fsOp.GetFSSpace(srcMount); err != nil {
					ll.Warnf("failed to get FS Space on %s, err: %s", srcMount, err)
				} else {
					ll.Debugf("FS Space on %s, is %d", srcMount, spaceOnMountPoint)
				}
			}
		}

		return fmt.Errorf("unable to mount %s to %s: %v", src, dst, err)
	}
	return nil
}

// UnmountWithCheck idempotent implemetation of unmount operation
// check whether path is mounted and only if yes - try to unmount
func (fsOp *FSOperationsImpl) UnmountWithCheck(path string) error {
	isMounted, err := fsOp.IsMounted(path)
	if err != nil {
		return fmt.Errorf("unable to check wthether path mounted or no: %v", err)
	}
	if !isMounted {
		fsOp.log.WithField("method", "Unmount").Infof("Path %s is not mounted", path)
		return nil
	}

	return fsOp.Unmount(path)
}

// MountFakeTmpfs does attach of temp folder in read only mode
func (fsOp *FSOperationsImpl) MountFakeTmpfs(volumeID, dst string) error {
	/*
		CMD example:
			mount -t tmpfs -o size=1M,ro <volumeID> <dst>
	*/
	ll := fsOp.log.WithFields(logrus.Fields{
		"method": "MountFakeTmpfs",
	})

	ll.Warningf("Simulate attachment")
	_, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		createCMD := fsOp.MkDir
		if err = createCMD(dst); err != nil {
			return err
		}
	}

	return fsOp.Mount(volumeID, dst, "-t tmpfs -o size=1M,ro")
}

// CreateFSIfNotExist checks FS and creates one if not exist
/*
	CMD example:
		lsblk <device> --output FSTYPE --noheadings
		# Check output

		mkfs.<fsType> <device>
*/
func (fsOp *FSOperationsImpl) CreateFSIfNotExist(fsType fs.FileSystem, device string) error {
	ll := fsOp.log.WithFields(logrus.Fields{
		"method": "CreateFSIfNotExist",
	})

	// check FS
	existingFS, err := fsOp.GetFSType(device)
	if err != nil {
		ll.Errorf("Unable to check FS type on %s: %v", device, err)
		return err
	}
	if fs.FileSystem(existingFS) == fsType {
		ll.Warnf("FS on %s with type %s is already exist, skip creating", device, fs.FileSystem(existingFS))
		return nil
	}
	if existingFS != "" {
		ll.Errorf("device %s is not empty. Existing FS - %s", device, fsType)
		return fmt.Errorf("device %s is not empty. Existing FS - %s", device, fsType)
	}

	// create FS
	err = fsOp.CreateFS(fsType, device)
	if err != nil {
		ll.Errorf("Unable to create FS type %s on %s: %v", fsType, device, err)
		return err
	}

	return nil
}

// Add options to mount command
// Example: <mount> -o option1,option2 ...
func addMountOptions(mountOptions []string) (opts string) {
	if len(mountOptions) > 0 {
		opts += fs.MountOptionsFlag + " "
		for i, opt := range mountOptions {
			opts += opt
			if i != len(mountOptions)-1 {
				opts += ","
			}
		}
	}

	return
}
