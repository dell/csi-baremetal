package util

import (
	"os/exec"
	"strings"
)

// HalDisk is a struct for a disk
type HalDisk struct {
	Vid  string `json:"Vid"`
	Pid  string `json:"Pid"`
	SN   string `json:"SN"`
	Path string `json:"Path"`
	// must be enum
	Internal bool `json:"Internal"`
	// HDD/SSD
	Rotational string `json:"Rotational"`
	Capacity   string `json:"Capacity"`
	// must be enum
	Health         int    `json:"Health"`
	PartitionCount int    `json:"partition_count"`
	Slot           string `json:"Slot"`
}

// AllDisks return block devices (/dev/sdXXX) without partitions from a node
func AllDisks() *[]HalDisk {
	devMask := "/dev/sd"

	cmd := exec.Command("lsblk", "-d", "-n", "-l", "-p", "-o", "TYPE,NAME", "-e", "7")
	out, err := cmd.CombinedOutput()

	if err != nil {
		logger.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	outlines := strings.Split(string(out), "\n")
	disks := make([]string, 0)

	for i := 0; i < (len(outlines) - 1); i++ {
		line := outlines[i]
		device := strings.Split(line, " ")

		if len(device) != 2 {
			logger.Error("Failed to parse line ", line)
		}

		devType := device[0]
		devName := device[1]

		if devType == "disk" && strings.Contains(devName, devMask) {
			disks = append(disks, devName)
		}
	}

	halDisks := make([]HalDisk, len(disks))

	for i := 0; i < len(disks); i++ {
		path := disks[i]
		halDisks[i].Path = path

		cmd := exec.Command("lsblk", "-d", "-n", "-o", "VENDOR", path)
		out, _ := cmd.CombinedOutput()
		halDisks[i].Vid = strings.TrimSpace(string(out))

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "MODEL", path)
		out, _ = cmd.CombinedOutput()
		halDisks[i].Pid = strings.TrimSpace(string(out))

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "SERIAL", path)
		out, _ = cmd.CombinedOutput()
		halDisks[i].SN = strings.TrimSpace(string(out))

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "SIZE", path)
		out, _ = cmd.CombinedOutput()
		halDisks[i].Capacity = strings.TrimSpace(string(out))

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "ROTA", path)
		out, _ = cmd.CombinedOutput()
		halDisks[i].Rotational = strings.TrimSpace(string(out))
		halDisks[i].PartitionCount = countPartitions(path)
	}

	return &halDisks
}

// collect path to the block devices without partitions
func AllDisksWithoutPartitions() []string { // list of path to device
	devs := make([]string, 0)
	allDisks := AllDisks()
	// create physical volume from each disk with 0 partition
	for _, d := range *allDisks {
		if d.PartitionCount == 0 {
			devs = append(devs, d.Path)
		}
	}

	return devs
}

func countPartitions(device string) int {
	cmd := exec.Command("lsblk", "-n", "-l", "-p", "-o", "TYPE,NAME", device)

	out, err := cmd.CombinedOutput()

	if err != nil {
		logger.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	output := string(out)
	logger.Info("combined out: ", output)

	outlines := strings.Split(output, "\n")
	l := len(outlines)

	count := 0

	for i := 0; i < (l - 1); i++ {
		line := outlines[i]
		//fmt.Printf("%d: %s\n", i, line)
		device := strings.Split(line, " ")

		if len(device) != 2 {
			logger.Error("Failed to parse line ", line)
		}

		devType := device[0]

		if devType == "part" {
			count++
		}
	}

	return count
}
