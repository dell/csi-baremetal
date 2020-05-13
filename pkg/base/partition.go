package base

import (
	"fmt"
	"sync"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	ph "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/partitionhelper"
)

// TODO: remove duplicate and refactor AK8S-640

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

// Partition is the basic implementation of Partitioner interface
type Partition struct {
	helper  ph.Partitioner
	opMutex sync.Mutex
}

// NewPartition is a constructor for Partition instance
func NewPartition(e command.CmdExecutor) *Partition {
	return &Partition{
		helper: ph.NewPartition(e),
	}
}

// TODO: run all operation with retries, without synchronization AK8S-171

// IsPartitionExists checks if a partition 1 exists in a provided device
// Receives path to a device to check a partition existence
// Returns partition existence status or error if something went wrong
func (p *Partition) IsPartitionExists(device string) (bool, error) {
	var partNum = "1" // TODO: do not hardcode it AK8S-640
	p.opMutex.Lock()
	exist, err := p.helper.IsPartitionExists(device, partNum)
	p.opMutex.Unlock()

	return exist, err
}

// CreatePartitionTable created partition table on a provided device of GPT type
// Receives device path on which to create table
// Returns error if something went wrong
func (p *Partition) CreatePartitionTable(device string) error {
	p.opMutex.Lock()
	err := p.helper.CreatePartitionTable(device, ph.PartitionGPT)
	p.opMutex.Unlock()

	return err
}

// GetPartitionTableType returns string that represent partition table type
// Receives device path from which partition table type should be got
// Returns partition table type as a string or error if something went wrong
func (p *Partition) GetPartitionTableType(device string) (string, error) {
	p.opMutex.Lock()
	res, err := p.helper.GetPartitionTableType(device)
	p.opMutex.Unlock()

	return res, err
}

// CreatePartition creates CSI partition on a provided device and checks that it was created with partprobe
// Receives device path to create a partition
// Returns error if something went wrong
func (p *Partition) CreatePartition(device string) error {
	var (
		err      error
		partName = "CSI" // TODO: do not hardcode it AK8S-640
	)

	p.opMutex.Lock()
	defer p.opMutex.Unlock()
	if err = p.helper.CreatePartition(device, partName); err != nil {
		return err
	}
	if err = p.helper.SyncPartitionTable(device); err != nil {
		return fmt.Errorf("partition was created however partprobe failed: %v", err)
	}

	return nil
}

// DeletePartition removes partition from a provided device
// Receives device path from which partition should be deleted
// Returns error if something went wrong
func (p *Partition) DeletePartition(device string) error {
	var partNum = "1" // TODO: do not hardcode it AK8S-640

	p.opMutex.Lock()
	defer p.opMutex.Unlock()

	exist, err := p.helper.IsPartitionExists(device, partNum)
	if err == nil && !exist {
		return nil
	}

	return p.helper.DeletePartition(device, partNum)
}

// SetPartitionUUID writes pvc uuid as GUID of the first partition of a device
// Receives device path and pvcUUID as strings
// Returns error if something went wrong
func (p *Partition) SetPartitionUUID(device, pvcUUID string) error {
	var (
		err     error
		partNum = "1" // TODO: do not hardcode it AK8S-640
	)

	p.opMutex.Lock()
	err = p.helper.SetPartitionUUID(device, partNum, pvcUUID)
	p.opMutex.Unlock()

	return err
}

// GetPartitionUUID reads partition unique GUID from the first partition of a provided device
// Receives device path from which to read
// Returns unique GUID as a string or error if something went wrong
func (p *Partition) GetPartitionUUID(device string) (string, error) {
	var partNum = "1" // TODO: do not hardcode it AK8S-640

	p.opMutex.Lock()
	partUUID, err := p.helper.GetPartitionUUID(device, partNum)
	p.opMutex.Unlock()

	return partUUID, err
}

// SyncPartitionTable syncs partition table for specific device
// Receives device path to sync with partprobe
// Returns error if something went wrong
func (p *Partition) SyncPartitionTable(device string) error {
	p.opMutex.Lock()
	err := p.helper.SyncPartitionTable(device)
	p.opMutex.Unlock()

	return err
}
