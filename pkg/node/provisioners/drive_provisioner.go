package provisioners

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/fs"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/partitionhelper"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	uw "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node/provisioners/utilwrappers"
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
	fsOps fs.WrapFS
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
		fsOps:     fs.NewFSImpl(e),
		partOps:   uw.NewPartitionOperationsImpl(e, log),
		k8sClient: k,
		crHelper:  k8s.NewCRHelper(k, log),
		log:       log.WithField("component", "DriveProvisioner"),
	}
}

// PrepareVolume create partition and FS based on vol attributes.
// After that partition is ready for mount operations
func (d *DriveProvisioner) PrepareVolume(vol api.Volume) error {
	ll := d.log.WithFields(logrus.Fields{
		"method":   "PrepareVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %v", vol)

	var (
		drive = &drivecrd.Drive{}
		err   error
	)

	// read Drive CR based on Volume.Location (vol.Location == Drive.UUID == Drive.Name)
	if err = d.k8sClient.ReadCR(context.Background(), vol.Location, drive); err != nil {
		return fmt.Errorf("failed to read drive CR with name %s, error %v", vol.Location, err)
	}

	ll.Infof("Search device file for drive with S/N %s", drive.Spec.SerialNumber)
	device, err := d.listBlk.SearchDrivePath(drive)
	if err != nil {
		return err
	}

	partUUID, _ := util.GetVolumeUUID(vol.Id)
	part := uw.Partition{
		Device:    device,
		TableType: partitionhelper.PartitionGPT,
		Label:     DefaultPartitionLabel,
		Num:       DefaultPartitionNumber,
		PartUUID:  partUUID,
		Ephemeral: vol.Ephemeral,
	}

	ll.Infof("Create partition %v on device %s and set UUID", part, device)
	partPtr, err := d.partOps.PreparePartition(part)
	if err != nil {
		ll.Errorf("Unable to prepare partition: %v", err)
		return fmt.Errorf("unable to prepare partition for volume %v", vol)
	}
	ll.Infof("Partition was created successfully %v", partPtr)

	// create FS
	return d.fsOps.CreateFS(fs.FileSystem(vol.Type), partPtr.GetFullPath())
}

// ReleaseVolume remove FS and partition based on vol attributes.
// After that partition is completely removed
func (d *DriveProvisioner) ReleaseVolume(vol api.Volume) error {
	ll := d.log.WithFields(logrus.Fields{
		"method":   "ReleaseVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %v", vol)

	drive := d.crHelper.GetDriveCRByUUID(vol.Location)

	if drive == nil {
		return errors.New("unable to find drive by vol location")
	}
	ll.Debugf("Got drive %v", drive)

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

	if vol.Ephemeral { // TODO temporary solution because of ephemeral volumes volume id AK8S-749
		part.PartUUID, err = d.partOps.GetPartitionUUID(device, DefaultPartitionNumber)
		if err != nil {
			return fmt.Errorf("unable to determine partition UUID: %v", err)
		}
	}

	part.Name = d.partOps.SearchPartName(device, part.PartUUID)
	if part.Name == "" {
		return errors.New("unable to find partition name")
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

// GetVolumePath constructs full partition path - /dev/DEVICE_NAME+PARTITION_NAME
func (d *DriveProvisioner) GetVolumePath(vol api.Volume) (string, error) {
	ll := d.log.WithFields(logrus.Fields{
		"method":   "GetVolumePath",
		"volumeID": vol.Id,
	})

	drive := d.crHelper.GetDriveCRByUUID(vol.Location)

	if drive == nil {
		return "", fmt.Errorf("unable to find drive by location %s", vol.Location)
	}
	ll.Debugf("Got drive %v", drive)

	// get deviceFile path
	device, err := d.listBlk.SearchDrivePath(drive)
	if err != nil {
		return "", fmt.Errorf("unable to find device for drive with S/N %s: %v", vol.Location, err)
	}
	ll.Debugf("Got device %s", device)

	var volumeUUID = vol.Id
	if vol.Ephemeral { // TODO temporary solution because of ephemeral volumes volume id AK8S-749
		volumeUUID, err = d.partOps.GetPartitionUUID(device, DefaultPartitionNumber)
		if err != nil {
			return "", fmt.Errorf("unable to determine partition UUID: %v", err)
		}
	}
	volumeUUID, _ = util.GetVolumeUUID(volumeUUID)

	partNum := d.partOps.SearchPartName(device, volumeUUID)
	if partNum == "" {
		return "", fmt.Errorf("unable to find part name for device %s by uuid %s", device, volumeUUID)
	}
	return device + partNum, nil
}
