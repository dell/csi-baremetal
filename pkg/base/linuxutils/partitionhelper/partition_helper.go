// Package partitionhelper contains code for manipulating with block device partitions and
// run such system utilites as parted, partprobe, sgdisk
package partitionhelper

import (
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// WrapPartition is the interface which encapsulates methods to work with drives' partitions
type WrapPartition interface {
	IsPartitionExists(device, partNum string) (exists bool, err error)
	GetPartitionTableType(device string) (ptType string, err error)
	CreatePartitionTable(device, partTableType string) (err error)
	CreatePartition(device, label string) (err error)
	DeletePartition(device, partNum string) (err error)
	SetPartitionUUID(device, partNum, partUUID string) error
	GetPartitionUUID(device, partNum string) (string, error)
	SyncPartitionTable(device string) error
	GetPartitionNameByUUID(device, partUUID string) (string, error)
}

const (
	// PartitionGPT is the const for GPT partition table
	PartitionGPT = "gpt"
	// parted is a name of system util
	parted = "parted "
	// partprobe is a name of system util
	partprobe = "partprobe "
	// sgdisk is a name of system util
	sgdisk = "sgdisk "

	// PartprobeDeviceCmdTmpl check that device has partition cmd
	PartprobeDeviceCmdTmpl = partprobe + "-d -s %s"
	// PartprobeCmdTmpl check device has partition with partprobe cmd
	PartprobeCmdTmpl = partprobe + "%s"

	// CreatePartitionTableCmdTmpl create partition table on provided device of provided type cmd template
	// fill device and partition table type
	CreatePartitionTableCmdTmpl = parted + "-s %s mklabel %s"
	// CreatePartitionCmdTmpl create partition on provided device cmd template, fill device and partition label
	CreatePartitionCmdTmpl = parted + "-s %s mkpart --align optimal %s 0%% 100%%"
	// DeletePartitionCmdTmpl delete partition from provided device cmd template, fill device and partition number
	DeletePartitionCmdTmpl = parted + "-s %s rm %s"

	// SetPartitionUUIDCmdTmpl command for set GUID of the partition, fill device, part number and part UUID
	SetPartitionUUIDCmdTmpl = sgdisk + "%s --partition-guid=%s:%s"
	// GetPartitionUUIDCmdTmpl command for read GUID of the first partition, fill device and part number
	GetPartitionUUIDCmdTmpl = sgdisk + "%s --info=%s"
)

// supportedTypes list of supported partition table types
var supportedTypes = []string{PartitionGPT}

// WrapPartitionImpl is the basic implementation of WrapPartition interface
type WrapPartitionImpl struct {
	e         command.CmdExecutor
	lsblkUtil lsblk.WrapLsblk
	opMutex   sync.Mutex
}

// NewWrapPartitionImpl is a constructor for WrapPartitionImpl instance
func NewWrapPartitionImpl(e command.CmdExecutor, log *logrus.Logger) *WrapPartitionImpl {
	return &WrapPartitionImpl{
		e:         e,
		lsblkUtil: lsblk.NewLSBLK(log),
	}
}

// IsPartitionExists checks if a partition exists in a provided device
// Receives path to a device to check a partition existence
// Returns partition existence status or error if something went wrong
func (p *WrapPartitionImpl) IsPartitionExists(device, partNum string) (bool, error) {
	cmd := fmt.Sprintf(PartprobeDeviceCmdTmpl, device)
	/*
		example of output:
		$ partprobe -d -s /dev/sdy
		/dev/sdy: gpt partitions 1 2
	*/

	p.opMutex.Lock()
	stdout, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

	if err != nil {
		return false, fmt.Errorf("unable to check partition %#v existence for %s", partNum, device)
	}

	stdout = strings.TrimSpace(stdout)

	s := strings.Split(stdout, "partitions")
	// after splitting partition number might appear on 2nd place in slice
	if len(s) > 1 && s[1] != "" {
		return true, nil
	}

	return false, nil
}

// CreatePartitionTable created partition table on a provided device
// Receives device path on which to create table
// Returns error if something went wrong
func (p *WrapPartitionImpl) CreatePartitionTable(device, partTableType string) error {
	if !util.ContainsString(supportedTypes, partTableType) {
		return fmt.Errorf("unable to create partition table for device %s unsupported partition table type: %#v",
			device, partTableType)
	}

	cmd := fmt.Sprintf(CreatePartitionTableCmdTmpl, device, partTableType)
	_, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return fmt.Errorf("unable to create partition table for device %s", device)
	}

	return nil
}

// GetPartitionTableType returns string that represent partition table type
// Receives device path from which partition table type should be got
// Returns partition table type as a string or error if something went wrong
func (p *WrapPartitionImpl) GetPartitionTableType(device string) (string, error) {
	cmd := fmt.Sprintf(PartprobeDeviceCmdTmpl, device)

	stdout, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return "", fmt.Errorf("unable to get partition table for device %s", device)
	}
	// /dev/sda: msdos partitions 1
	s := strings.Split(stdout, " ")
	if len(s) < 2 {
		return "", fmt.Errorf("unable to parse output '%s' for device %s", stdout, device)
	}
	// partition table type is on 2nd place in slice
	return s[1], nil
}

// CreatePartition creates partition with name partName on a device
// Receives device path to create a partition
// Returns error if something went wrong
func (p *WrapPartitionImpl) CreatePartition(device, label string) error {
	cmd := fmt.Sprintf(CreatePartitionCmdTmpl, device, label)

	p.opMutex.Lock()
	_, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

	if err != nil {
		return err
	}

	return nil
}

// DeletePartition removes partition partNum from a provided device
// Receives device path and it's partition which should be deleted
// Returns error if something went wrong
func (p *WrapPartitionImpl) DeletePartition(device, partNum string) error {
	cmd := fmt.Sprintf(DeletePartitionCmdTmpl, device, partNum)

	p.opMutex.Lock()
	_, stderr, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

	if err != nil {
		return fmt.Errorf("unable to delete partition %#v from device %s: %s, error: %v",
			partNum, device, stderr, err)
	}

	return nil
}

// SetPartitionUUID writes partUUID as GUID for the partition partNum of a provided device
// Receives device path and partUUID as strings
// Returns error if something went wrong
func (p *WrapPartitionImpl) SetPartitionUUID(device, partNum, partUUID string) error {
	cmd := fmt.Sprintf(SetPartitionUUIDCmdTmpl, device, partNum, partUUID)

	if _, _, err := p.e.RunCmd(cmd); err != nil {
		return err
	}

	return nil
}

// GetPartitionUUID reads partition unique GUID from the partition partNum of a provided device
// Receives device path from which to read
// Returns unique GUID as a string or error if something went wrong
func (p *WrapPartitionImpl) GetPartitionUUID(device, partNum string) (string, error) {
	/*
		example of command output:
		$ sgdisk /dev/sdy --info=1
		Partition GUID code: 0FC63DAF-8483-4772-8E79-3D69D8477DE4 (Linux filesystem)
		Partition unique GUID: 5209CFD8-3AB1-4720-BCEA-DFA80315EC92
		First sector: 2048 (at 1024.0 KiB)
		Last sector: 999423 (at 488.0 MiB)
		Partition size: 997376 sectors (487.0 MiB)
		Attribute flags: 0000000000000000
		Partition name: ''
	*/
	cmd := fmt.Sprintf(GetPartitionUUIDCmdTmpl, device, partNum)
	partitionPresentation := "Partition unique GUID:"

	stdout, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, partitionPresentation) {
			res := strings.Split(strings.TrimSpace(line), partitionPresentation)
			if len(res) > 1 {
				return strings.ToLower(strings.TrimSpace(res[1])), nil
			}
		}
	}

	return "", fmt.Errorf("unable to get partition GUID for device %s", device)
}

// SyncPartitionTable syncs partition table for specific device
// Receives device path to sync with partprobe, device could be an empty string (sync for all devices in the system)
// Returns error if something went wrong
func (p *WrapPartitionImpl) SyncPartitionTable(device string) error {
	cmd := fmt.Sprintf(PartprobeCmdTmpl, device)

	p.opMutex.Lock()
	_, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

	if err != nil {
		return err
	}

	return nil
}

// GetPartitionNameByUUID gets partition name by it's UUID
// for example "1" for /dev/sda1,  "1p2" for /dev/nvme1p2,  "0p3" for /dev/loopback0p3
// Receives a device path and uuid of partition to find
// Returns a partition number or error if something went wrong
func (p *WrapPartitionImpl) GetPartitionNameByUUID(device, partUUID string) (string, error) {
	if device == "" {
		return "", fmt.Errorf("unable to find partition name by UUID %#v - device name is empty", partUUID)
	}

	if partUUID == "" {
		return "", fmt.Errorf("unable to find partition name for device %#v partition UUID is empty", device)
	}

	// list partitions
	blockdevices, err := p.lsblkUtil.GetBlockDevices(device)
	if err != nil {
		return "", err
	}
	if len(blockdevices) == 0 {
		return "", fmt.Errorf("empty output for device %s", device)
	}

	// try to find partition name
	for _, id := range blockdevices[0].Children {
		// ignore cases
		if strings.EqualFold(partUUID, id.PartUUID) {
			// partition name not detected
			if id.Name == "" {
				return "", fmt.Errorf("partition %s for device %s found but name is not present",
					partUUID, device)
			}
			return strings.Replace(id.Name, device, "", 1), nil
		}
	}

	return "", fmt.Errorf("unable to find partition name by UUID %s for device %s within %v",
		partUUID, device, blockdevices)
}
