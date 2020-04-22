package base

import (
	"fmt"
	"strings"
	"sync"
)

// Partitioner is the interface which encapsulates methods to work with drives' partitions
type Partitioner interface {
	IsPartitionExists(device string) (exists bool, err error)
	CreatePartitionTable(device string) (err error)
	GetPartitionTableType(device string) (ptType string, err error)
	CreatePartition(device string) (err error)
	DeletePartition(device string) (err error)
	SetPartitionUUID(device, pvcUUID string) (err error)
	GetPartitionUUID(device string) (uuid string, err error)
	SyncPartitionTable(string) error
}

const (
	// PartitionGPT is the const for GPT partition table
	PartitionGPT = "gpt"
	// PartprobeCmd is base partprobe cmd
	PartprobeCmd = "partprobe"
	// PartprobeDeviceCmdTmpl check device has partition with partprobe -d -s cmd
	PartprobeDeviceCmdTmpl = PartprobeCmd + " -d -s %s"
	// PartprobeCmdTmpl check device has partition with partprobe cmd
	PartprobeCmdTmpl = PartprobeCmd + " %s"
	// CreatePartitionTableCmdTmpl create partition table on provided device of provided type cmd
	CreatePartitionTableCmdTmpl = "parted -s %s mklabel %s"
	// CreatePartitionCmdTmpl create CSI partition on provided device cmd
	CreatePartitionCmdTmpl = "parted -s %s mkpart --align optimal CSI 0%% 100%%"
	// DeletePartitionCmdTmpl delete the first partition from provided device cmd
	// todo get rid of hardcoded partition numbers
	DeletePartitionCmdTmpl = "parted -s %s rm 1"
	// SetPartitionUUIDCmdTmpl set GUID of the first partition as provided uuid on provided device cmd
	SetPartitionUUIDCmdTmpl = "sgdisk %s --partition-guid=1:%s"
	// GetPartitionUUIDCmdTmpl read GUID of the first partition cmd
	GetPartitionUUIDCmdTmpl = "sgdisk %s --info=1"
)

// Partition is the basic implementation of Partitioner interface
type Partition struct {
	e       CmdExecutor
	opMutex sync.Mutex
}

// TODO: run all operation with retries, without synchronization AK8S-171

// IsPartitionExists checks if a partition exists in a provided device
// Receives path to a device to check a partition existence
// Returns partition existence status or error if something went wrong
func (p *Partition) IsPartitionExists(device string) (bool, error) {
	cmd := fmt.Sprintf(PartprobeDeviceCmdTmpl, device)

	p.opMutex.Lock()
	stdout, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

	if err != nil {
		return false, fmt.Errorf("unable to check partition existence for %s", device)
	}

	stdout = strings.TrimSpace(stdout)

	// stdout with partitions contains something like - /dev/sda: msdos partitions 1
	// without partitions - /dev/sda: msdos partitions
	s := strings.Split(stdout, "partitions")
	// after splitting partition number might appear on 2nd place in slice
	if len(s) > 1 && s[1] != "" {
		return true, nil
	}

	return false, nil
}

// CreatePartitionTable created partition table on a provided device of GPT type
// Receives device path on which to create table
// Returns error if something went wrong
func (p *Partition) CreatePartitionTable(device string) error {
	cmd := fmt.Sprintf(CreatePartitionTableCmdTmpl, device, PartitionGPT)

	p.opMutex.Lock()
	_, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

	if err != nil {
		return fmt.Errorf("unable to create partition table for device %s", device)
	}

	return nil
}

// GetPartitionTableType returns string that represent partition table type
// Receives device path from which partition table type should be got
// Returns partition table type as a string or error if something went wrong
func (p *Partition) GetPartitionTableType(device string) (string, error) {
	cmd := fmt.Sprintf(PartprobeDeviceCmdTmpl, device)

	p.opMutex.Lock()
	stdout, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

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

// CreatePartition creates CSI partition on a provided device and checks that it was created with partprobe
// Receives device path to create a partition
// Returns error if something went wrong
func (p *Partition) CreatePartition(device string) error {
	cmd := fmt.Sprintf(CreatePartitionCmdTmpl, device)

	p.opMutex.Lock()
	defer p.opMutex.Unlock()
	if _, _, err := p.e.RunCmd(cmd); err != nil {
		return err
	}
	if _, _, err := p.e.RunCmd(fmt.Sprintf(PartprobeCmdTmpl, device)); err != nil {
		return err
	}

	return nil
}

// DeletePartition removes partition from a provided device
// Receives device path from which partition should be deleted
// Returns error if something went wrong
func (p *Partition) DeletePartition(device string) error {
	if exist, err := p.IsPartitionExists(device); err == nil && !exist {
		return nil
	}

	cmd := fmt.Sprintf(DeletePartitionCmdTmpl, device)
	p.opMutex.Lock()
	defer p.opMutex.Unlock()

	if _, stderr, err := p.e.RunCmd(cmd); err != nil {
		return fmt.Errorf("stderr: %s, error: %v", stderr, err)
	}

	return nil
}

// SetPartitionUUID writes pvc uuid as GUID of the first partition of a device
// Receives device path and pvcUUID as strings
// Returns error if something went wrong
func (p *Partition) SetPartitionUUID(device, pvcUUID string) error {
	cmd := fmt.Sprintf(SetPartitionUUIDCmdTmpl, device, pvcUUID)

	p.opMutex.Lock()
	defer p.opMutex.Unlock()
	if _, _, err := p.e.RunCmd(cmd); err != nil {
		return err
	}

	return nil
}

// GetPartitionUUID reads partition unique GUID from the first partition of a provided device
// Receives device path from which to read
// Returns unique GUID as a string or error if something went wrong
func (p *Partition) GetPartitionUUID(device string) (string, error) {
	cmd := fmt.Sprintf(GetPartitionUUIDCmdTmpl, device)
	partitionPresentation := "Partition unique GUID:"

	p.opMutex.Lock()
	stdout, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

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
// Receives device path to sync with partprobe
// Returns error if something went wrong
func (p *Partition) SyncPartitionTable(device string) error {
	cmd := fmt.Sprintf(PartprobeCmdTmpl, device)

	p.opMutex.Lock()
	_, _, err := p.e.RunCmd(cmd)
	p.opMutex.Unlock()

	if err != nil {
		return err
	}

	return nil
}
