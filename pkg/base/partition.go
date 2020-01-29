package base

import (
	"errors"
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

type Partition struct {
	e CmdExecutor
}

func (p Partition) IsPartitionExists(device string) (exists bool, err error) {
	cmd := fmt.Sprintf("partprobe -d -s %s", device)
	stdout, _, err := p.e.RunCmd(cmd)
	if err != nil {
		return false, fmt.Errorf("unable to check partition existence for %s", device)
	}

	if stdout == "(no output)" {
		return false, nil
	}

	// stdout with partitions contains something like - /dev/sda: msdos partitions 1
	// without partitions - /dev/sda: msdos partitions
	s := strings.Split(stdout, "partitions")
	// after splitting slice have length=2
	if s[1] != "" {
		return true, nil
	}

	return false, nil
}

func (p Partition) CreatePartitionTable(device string) (err error) {
	label := "gpt"
	cmd := fmt.Sprintf("parted -s %s mklabel %s", device, label)

	_, _, err = p.e.RunCmd(cmd)

	if err != nil {
		return err
	}

	return nil
}

func (p Partition) GetPartitionTableType(device string) (ptType string, err error) {
	cmd := fmt.Sprintf("partprobe -d -s %s", device)
	stdout, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return "", errors.New("unable to get partition table")
	}
	// /dev/sda: msdos partitions 1
	s := strings.Split(stdout, " ")
	// partition table type is on 2nd place in slice
	return s[1], nil
}

func (p Partition) CreatePartition(device string) (err error) {
	cmd := fmt.Sprintf("parted -s %s mkpart --align optimal CSI 0%% 100%%", device)

	_, _, err = p.e.RunCmd(cmd)
	if err != nil {
		return err
	}

	_, _, err = p.e.RunCmd("partprobe")
	if err != nil {
		return err
	}

	return nil
}

func (p Partition) DeletePartition(device string) (err error) {
	cmd := fmt.Sprintf("parted -s %s rm 1", device)

	_, _, err = p.e.RunCmd(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (p Partition) SetPartitionUUID(device, pvcUUID string) (err error) {
	cmd := fmt.Sprintf("sgdisk %s -u 1:%s", device, pvcUUID)

	_, _, err = p.e.RunCmd(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (p Partition) GetPartitionUUID(device string) (uuid string, err error) {
	cmd := fmt.Sprintf("sgdisk %s -i 1", device)

	stdout, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return "", err
	}

	stdoutMap := make(map[string]string, 7)
	for _, p := range strings.Split(stdout, "\n") {
		line := strings.Split(p, ":")
		if len(line) < 2 {
			continue
		}
		stdoutMap[line[0]] = strings.TrimSpace(line[1])
	}

	if uuid, ok := stdoutMap["Partition unique GUID"]; ok {
		return strings.ToLower(uuid), nil
	}

	return "", errors.New("unable to get partition GUID")
}
