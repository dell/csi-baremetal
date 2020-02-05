package base

import (
	"fmt"
	"strings"
)

type Partitioner interface {
	IsPartitionExists(device string) (exists bool, err error)
	CreatePartitionTable(device string) (err error)
	GetPartitionTableType(device string) (ptType string, err error)
	CreatePartition(device string) (err error)
	DeletePartition(device string) (err error)
	SetPartitionUUID(device, pvcUUID string) (err error)
	GetPartitionUUID(device string) (uuid string, err error)
}

const PartitionGPT = "gpt"

type Partition struct {
	e CmdExecutor
}

func (p Partition) IsPartitionExists(device string) (bool, error) {
	cmd := fmt.Sprintf("partprobe -d -s %s", device)
	stdout, _, err := p.e.RunCmd(cmd)
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

func (p Partition) CreatePartitionTable(device string) error {
	cmd := fmt.Sprintf("parted -s %s mklabel %s", device, PartitionGPT)

	_, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return fmt.Errorf("unable to create partition table for device %s", device)
	}

	return nil
}

// GetPartitionTableType returns string that represent partition table type
func (p Partition) GetPartitionTableType(device string) (string, error) {
	cmd := fmt.Sprintf("partprobe -d -s %s", device)
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

func (p Partition) CreatePartition(device string) error {
	cmd := fmt.Sprintf("parted -s %s mkpart --align optimal CSI 0%% 100%%", device)

	if _, _, err := p.e.RunCmd(cmd); err != nil {
		return err
	}
	if _, _, err := p.e.RunCmd("partprobe"); err != nil {
		return err
	}

	return nil
}

func (p Partition) DeletePartition(device string) error {
	cmd := fmt.Sprintf("parted -s %s rm 1", device)

	if _, stderr, err := p.e.RunCmd(cmd); err != nil {
		return fmt.Errorf("stderr: %s, error: %v", stderr, err)
	}
	return nil
}

func (p Partition) SetPartitionUUID(device, pvcUUID string) error {
	cmd := fmt.Sprintf("sgdisk %s --partition-guid=1:%s", device, pvcUUID)

	if _, _, err := p.e.RunCmd(cmd); err != nil {
		return err
	}

	return nil
}

func (p Partition) GetPartitionUUID(device string) (string, error) {
	cmd := fmt.Sprintf("sgdisk %s --info=1", device)
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
