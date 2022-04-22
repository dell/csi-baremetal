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
	CreatePartition(device, label, partUUID string) (err error)
	DeletePartition(device, partNum string) (err error)
	GetPartitionUUID(device, partNum string) (string, error)
	SyncPartitionTable(device string) error
	GetPartitionNameByUUID(device, partUUID string) (string, error)
	DeviceHasPartitionTable(device string) (bool, error)
	DeviceHasPartitions(device string) (bool, error)
}

const (
	// PartitionGPT is the const for GPT partition table
	PartitionGPT = "gpt"
	// partprobe is a name of system util
	partprobe = "partprobe "
	// sgdisk is a name of system util
	sgdisk = "sgdisk "
	// fdiks is a name of system util
	fdisk = "fdisk "
	// blockdev is a name of system util
	blockdev = "blockdev "

	// PartprobeDeviceCmdTmpl check that device has partition cmd
	PartprobeDeviceCmdTmpl = partprobe + "-d -s %s"
	// BlockdevCmdTmpl synchronize the partition table
	BlockdevCmdTmpl = blockdev + "--rereadpt -v %s"

	// CreatePartitionTableCmdTmpl create partition table on provided device of provided type cmd template
	// fill device and partition table type
	CreatePartitionTableCmdTmpl = sgdisk + "%s -o"
	// CreatePartitionCmdTmpl create partition on provided device cmd template, fill device and partition label
	CreatePartitionCmdTmpl = sgdisk + "-n 1:0:0 -c 1:%s %s"
	// CreatePartitionCmdWithUUIDTmpl create partition on provided device with uuid cmd template, fill device and partition label
	CreatePartitionCmdWithUUIDTmpl = sgdisk + "-n 1:0:0 -c 1:%s -u 1:%s %s"
	// DeletePartitionCmdTmpl delete partition from provided device cmd template, fill device and partition number
	DeletePartitionCmdTmpl = sgdisk + "-d %s %s"

	// DetectPartitionTableCmdTmpl is used to print information, which contain partition table
	DetectPartitionTableCmdTmpl = fdisk + "--list %s"

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
	stdout, _, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(PartprobeDeviceCmdTmpl, ""))))
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

	cmd := fmt.Sprintf(CreatePartitionTableCmdTmpl, device)
	_, _, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(CreatePartitionTableCmdTmpl, ""))))

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

	stdout, _, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(PartprobeDeviceCmdTmpl, ""))))

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
func (p *WrapPartitionImpl) CreatePartition(device, label, partUUID string) error {
	cmd := fmt.Sprintf(CreatePartitionCmdWithUUIDTmpl, label, partUUID, device)

	p.opMutex.Lock()
	_, _, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(CreatePartitionCmdTmpl, "", ""))))
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
	cmd := fmt.Sprintf(DeletePartitionCmdTmpl, partNum, device)

	p.opMutex.Lock()
	_, stderr, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(DeletePartitionCmdTmpl, "", ""))))
	p.opMutex.Unlock()

	if err != nil {
		return fmt.Errorf("unable to delete partition %#v from device %s: %s, error: %v",
			partNum, device, stderr, err)
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

	stdout, _, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(GetPartitionUUIDCmdTmpl, "", ""))))

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
	cmd := fmt.Sprintf(BlockdevCmdTmpl, device)

	p.opMutex.Lock()
	_, _, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(BlockdevCmdTmpl, ""))))
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

// DeviceHasPartitionTable calls parted  and determine if device has partition table from output
// Receive device path
// Return true if device has partition table, false in opposite, error if something went wrong
func (p *WrapPartitionImpl) DeviceHasPartitionTable(device string) (bool, error) {
	/*
		Disk /dev/sda: 931.5 GiB, 1000204886016 bytes, 1953525168 sectors
		Units: sectors of 1 * 512 = 512 bytes
		Sector size (logical/physical): 512 bytes / 512 bytes
		I/O size (minimum/optimal): 512 bytes / 512 bytes
		Disklabel type: gpt

	*/
	labelKey := "Disklabel type"

	cmd := fmt.Sprintf(DetectPartitionTableCmdTmpl, device)

	p.opMutex.Lock()
	stdout, _, err := p.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(DetectPartitionTableCmdTmpl, ""))))
	p.opMutex.Unlock()

	if err != nil {
		return false, err
	}

	if strings.Contains(stdout, labelKey) {
		return true, nil
	}
	return false, nil
}

// DeviceHasPartitions calls lsblk and determine if device has partitions (children)
// Receive device path
// Return true if device has partitions, false in opposite, error if something went wrong
func (p *WrapPartitionImpl) DeviceHasPartitions(device string) (bool, error) {
	blockDevices, err := p.lsblkUtil.GetBlockDevices(device)
	if len(blockDevices) != 1 {
		return false, fmt.Errorf("wrong output of lsblk for %s, block devices: %v", device, blockDevices)
	}

	if err != nil {
		return false, err
	}

	// check number of partitions
	if len(blockDevices[0].Children) > 0 {
		return true, nil
	}

	return false, nil
}
