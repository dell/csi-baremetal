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
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	baseerr "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
	"github.com/dell/csi-baremetal/pkg/base/util"
	uw "github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
)

const (
	// DefaultPartitionLabel default label for each partition
	DefaultPartitionLabel = "CSI"
	// DefaultPartitionNumber partition number
	DefaultPartitionNumber = "1"
)

// DriveProvisioner is a implementation of Provisioner interface
// works with drives and partitions on them
type DriveProvisioner struct {
	listBlk lsblk.WrapLsblk
	// fsOps uses for operations with file systems
	fsOps uw.FSOperations
	// partOps uses for operations with partitions
	partOps uw.PartitionOperations

	k8sClient *k8s.KubeClient
	crHelper  *k8s.CRHelper

	log *logrus.Entry
}

// NewDriveProvisioner is a constructor for DriveProvisioner instance
func NewDriveProvisioner(
	e command.CmdExecutor,
	k *k8s.KubeClient,
	log *logrus.Logger) *DriveProvisioner {
	return &DriveProvisioner{
		listBlk:   lsblk.NewLSBLK(log),
		fsOps:     uw.NewFSOperationsImpl(e, log),
		partOps:   uw.NewPartitionOperationsImpl(e, log),
		k8sClient: k,
		crHelper:  k8s.NewCRHelper(k, log),
		log:       log.WithField("component", "DriveProvisioner"),
	}
}

// PrepareVolume create partition and FS based on vol attributes.
// After that partition is ready for mount operations
func (d *DriveProvisioner) PrepareVolume(vol *api.Volume) error {
	ll := d.log.WithFields(logrus.Fields{
		"method":   "PrepareVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %+v", *vol)

	var (
		ctxWithID = context.WithValue(context.Background(), base.RequestUUID, vol.Id)
		drive     = &drivecrd.Drive{}
		err       error
	)

	// read Drive CR based on Volume.Location (vol.Location == Drive.UUID == Drive.Name)
	if err = d.k8sClient.ReadCR(ctxWithID, vol.Location, "", drive); err != nil {
		return fmt.Errorf("failed to read drive CR with name %s, error %w", vol.Location, err)
	}

	ll.Infof("Search device file for drive with S/N %s", drive.Spec.SerialNumber)
	device, err := d.listBlk.SearchDrivePath(&drive.Spec)
	if err != nil {
		return err
	}

	if vol.Mode == apiV1.ModeRAW {
		return nil
	}

	volUUID, err := util.GetVolumeUUID(vol.Id)
	if err != nil {
		return fmt.Errorf("failed to get volume UUID %s: %w", vol.Id, err)
	}

	part := uw.Partition{
		Device:    device,
		TableType: partitionhelper.PartitionGPT,
		Label:     DefaultPartitionLabel,
		Num:       DefaultPartitionNumber,
		PartUUID:  volUUID,
	}

	ll.Infof("Create partition %v on device %s and set UUID", part, device)
	partPtr, err := d.partOps.PreparePartition(part)
	if err != nil {
		ll.Errorf("Unable to prepare partition: %v", err)
		return fmt.Errorf("unable to prepare partition for volume %v", vol)
	}
	ll.Infof("Partition was created successfully %+v", partPtr)

	if vol.Mode == apiV1.ModeRAWPART {
		return nil
	}

	return d.fsOps.CreateFSIfNotExist(fs.FileSystem(vol.Type), partPtr.GetFullPath(), volUUID)
}

// ReleaseVolume remove FS and partition based on vol attributes.
// After that partition is completely removed
func (d *DriveProvisioner) ReleaseVolume(vol *api.Volume, drive *api.Drive) error {
	ll := d.log.WithFields(logrus.Fields{
		"method":   "ReleaseVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %+v", *vol)

	// get deviceFile path
	device, err := d.listBlk.SearchDrivePath(drive)
	if err != nil {
		return fmt.Errorf("unable to find device for drive with S/N %s", vol.Location)
	}
	ll.Debugf("Got device %s", device)

	var (
		partUUID, _ = util.GetVolumeUUID(vol.Id)
		part        = uw.Partition{
			Device:   device,
			Num:      DefaultPartitionNumber,
			PartUUID: partUUID,
		}
	)

	part.Name, err = d.partOps.SearchPartName(device, part.PartUUID)
	if err != nil {
		return d.wipeDevice(device,
			fmt.Errorf("unable to find partition name for volume %s: %w", vol.Id, err), ll)
	}

	// wipe FS on partition
	if err = d.fsOps.WipeFS(part.GetFullPath()); err != nil {
		return err
	}

	err = d.partOps.ReleasePartition(part)
	if err != nil {
		return fmt.Errorf("unable to release partition: %v", err)
	}

	// wipe all superblocks (wipe partition table signature)
	return d.fsOps.WipeFS(device)
}

// wipeDevice check is there any partition on device or not,
// if there are no partition - wipe device and return nil, if any - returns error that had been provided
// device - device to check, err - error to return, ll - logger for logging
func (d *DriveProvisioner) wipeDevice(device string, err error, ll *logrus.Entry) error {
	// DriveProvisioner assumes that there could be only one partition per drive
	bdevs, sErr := d.listBlk.GetBlockDevices(device)
	if sErr == nil && (len(bdevs) == 0 || bdevs[0].Children == nil) {
		ll.Infof("No partitions found for device %s", device)
		return d.fsOps.WipeFS(device) // wipe partition table
	}
	return err
}

// GetVolumePath constructs full partition path - /dev/DEVICE_NAME+PARTITION_NAME
func (d *DriveProvisioner) GetVolumePath(vol *api.Volume) (string, error) {
	ll := d.log.WithFields(logrus.Fields{
		"method":   "GetVolumePath",
		"volumeID": vol.Id,
	})

	var (
		ctxWithID = context.WithValue(context.Background(), base.RequestUUID, vol.Id)
		drive     = &drivecrd.Drive{}
	)

	// read Drive CR based on Volume.Location (vol.Location == Drive.UUID == Drive.Name)
	if err := d.k8sClient.ReadCR(ctxWithID, vol.Location, "", drive); err != nil {
		ll.Errorf("failed to get drive CR %s: %v", vol.Location, err)
		if baseerr.IsSafeReturnError(err) {
			return "", baseerr.ErrorGetDriveFailed
		}
		return "", err
	}
	ll.Debugf("Got drive %+v", drive)

	// get deviceFile path
	device, err := d.listBlk.SearchDrivePath(&drive.Spec)
	if err != nil {
		return "", fmt.Errorf("unable to find device for drive with S/N %s: %v", vol.Location, err)
	}
	ll.Debugf("Got device %s", device)

	var volumeUUID = vol.Id
	if vol.Mode == apiV1.ModeRAW {
		return device, nil
	}
	volumeUUID, _ = util.GetVolumeUUID(volumeUUID)

	partNum, err := d.partOps.SearchPartName(device, volumeUUID)
	if err != nil {
		// on device disconnect or node reboot device name might change and we need to re-sync drive info
		return "", fmt.Errorf("unable to find part name for device %s by uuid %s: %w", device, volumeUUID, err)
	}
	return device + partNum, nil
}
