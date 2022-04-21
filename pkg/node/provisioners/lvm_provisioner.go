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

package provisioners

import (
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/util"
	uw "github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
)

// LVMProvisioner is a implementation of Provisioner interface
// Work with volumes based on Volume Groups
type LVMProvisioner struct {
	lvmOps   lvm.WrapLVM
	fsOps    uw.FSOperations
	crHelper *k8s.CRHelper
	log      *logrus.Entry
}

// NewLVMProvisioner is a constructor for LVMProvisioner
func NewLVMProvisioner(e command.CmdExecutor, k *k8s.KubeClient, log *logrus.Logger) *LVMProvisioner {
	return &LVMProvisioner{
		lvmOps:   lvm.NewLVM(e, log),
		fsOps:    uw.NewFSOperationsImpl(e, log),
		crHelper: k8s.NewCRHelper(k, log),
		log:      log.WithField("component", "LVMProvisioner"),
	}
}

// PrepareVolume search volume group based on vol attributes, creates Logical Volume
// and create file system on it. After that Logical Volume is ready for mount operations
func (l *LVMProvisioner) PrepareVolume(vol *api.Volume) error {
	ll := l.log.WithFields(logrus.Fields{
		"method":   "PrepareVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %+v", *vol)

	var (
		vgName string
		err    error
	)

	// prepare size in megabytes for the argument
	size, _ := util.ToSizeUnit(vol.Size, util.BYTE, util.MBYTE)
	sizeStr := strconv.FormatInt(size, 10)
	sizeStr += "m"

	vgName, err = l.getVGName(vol)
	if err != nil {
		return err
	}

	volUUID, err := util.GetVolumeUUID(vol.Id)
	if err != nil {
		return fmt.Errorf("failed to get volume UUID %s: %w", vol.Id, err)
	}

	// create lv with name /dev/VG_NAME/vol.Id
	ll.Infof("Creating LV %s sizeof %s in VG %s", vol.Id, sizeStr, vgName)
	if err = l.lvmOps.LVCreate(vol.Id, sizeStr, vgName); err != nil {
		return fmt.Errorf("unable to create LV: %v", err)
	}

	deviceFile := fmt.Sprintf("/dev/%s/%s", vgName, vol.Id)
	ll.Debugf("Creating FS on %s", deviceFile)

	if vol.Mode == apiV1.ModeRAW || vol.Mode == apiV1.ModeRAWPART {
		return nil
	}

	return l.fsOps.CreateFSIfNotExist(fs.FileSystem(vol.Type), deviceFile, volUUID)
}

// ReleaseVolume search volume group based on vol attributes, remove Logical Volume
// and wipe file system on it. After that Logical Volume that had consumed by vol is completely removed
func (l *LVMProvisioner) ReleaseVolume(vol *api.Volume, _ *api.Drive) error {
	ll := logrus.WithFields(logrus.Fields{
		"method":   "ReleaseVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %v", vol)

	deviceFile, err := l.GetVolumePath(vol)
	if err != nil {
		return fmt.Errorf("unable to determine full path of the volume: %v", err)
	}

	if err := l.fsOps.WipeFS(deviceFile); err != nil {
		// check whether such LV (deviceFile) exist or not
		vgName, sErr := l.getVGName(vol)
		if sErr != nil {
			return fmt.Errorf("unable to remove LV %s: %v and unable to determine VG name: %v",
				deviceFile, err, sErr)
		}
		lvs, sErr := l.lvmOps.GetLVsInVG(vgName)
		if sErr != nil {
			return fmt.Errorf("unable to remove LV %s: %v and unable to list LVs in VG %s: %v",
				deviceFile, err, vgName, sErr)
		}
		if !util.ContainsString(lvs, vol.Id) {
			ll.Infof("LV %s has been already removed", deviceFile)
			return nil
		}
		return fmt.Errorf("failed to wipe FS on device %s: %v", deviceFile, err)
	}

	return l.lvmOps.LVRemove(deviceFile)
}

// GetVolumePath search Volume Group name by vol attributes and construct
// full path to the volume using template: /dev/VG_NAME/LV_NAME
func (l *LVMProvisioner) GetVolumePath(vol *api.Volume) (string, error) {
	ll := l.log.WithFields(logrus.Fields{
		"method":   "GetVolumePath",
		"volumeID": vol.Id,
	})
	ll.Debugf("Processing for %v", vol)

	vgName, err := l.getVGName(vol)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/dev/%s/%s", vgName, vol.Id), nil // /dev/VG_NAME/LV_NAME
}

func (l *LVMProvisioner) getVGName(vol *api.Volume) (string, error) {
	var vgName = vol.Location

	// Volume.Location is an LVG CR name, LVG CR name in general is the same as a real VG name on node,
	// however for LVG based on system disk LVG CR name is not the same as a VG name
	// we need to read appropriate LVG CR and use LVG CR.Spec.Name as VG name
	if vol.StorageClass == apiV1.StorageClassSystemLVG {
		var err error
		vgName, err = l.crHelper.GetVGNameByLVGCRName(vol.Location)
		if err != nil {
			return "", fmt.Errorf("unable to determine VG name: %v", err)
		}
	}
	return vgName, nil
}
