package util

import (
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
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

// DisksCache is a slice for storing disks
var DisksCache []HalDisk

// AllDisks is a function for getting all disks from a node
func AllDisks() *[]HalDisk {
	logrus.Info("Searching for local disks...")
	if len(DisksCache) > 0 {
		logrus.Info("Found disks in cache, will return them: ", DisksCache)
		return &DisksCache
	}

	// list all devices excluding loop (MAJ = 7) and look for disks
	cmd := exec.Command("lsblk", "-d", "-n", "-l", "-p", "-o", "TYPE,NAME", "-e", "7")
	out, err := cmd.CombinedOutput()

	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	output := string(out)
	logrus.Info("combined out:\n%s\n", output)

	outlines := strings.Split(output, "\n")
	l := len(outlines)

	disks := make([]string, 0)

	for i := 0; i < (l - 1); i++ {
		line := outlines[i]
		//fmt.Printf("%d: %s\n", i, line)
		device := strings.Split(line, " ")
		if len(device) != 2 {
			logrus.Error("Failed to parse line %s\n", line)
		}
		devType := device[0]
		devName := device[1]

		if devType == "disk" {
			//logrus.Info("device Path: %s\n", dev_name)
			disks = append(disks, devName)
		}
	}

	logrus.Info("Parsed: ", disks)

	disksNum := len(disks)
	halDisks := make([]HalDisk, disksNum)
	for i := 0; i < disksNum; i++ {
		path := disks[i]
		halDisks[i].Path = path

		cmd := exec.Command("lsblk", "-d", "-n", "-o", "VENDOR", path)
		out, _ := cmd.CombinedOutput()
		stringOut := string(out)
		halDisks[i].Vid = strings.TrimSpace(stringOut)

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "MODEL", path)
		out, _ = cmd.CombinedOutput()
		stringOut = string(out)
		halDisks[i].Pid = strings.TrimSpace(stringOut)

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "SERIAL", path)
		out, _ = cmd.CombinedOutput()
		stringOut = string(out)
		halDisks[i].SN = strings.TrimSpace(stringOut)

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "SIZE", path)
		out, _ = cmd.CombinedOutput()
		stringOut = string(out)
		halDisks[i].Capacity = strings.TrimSpace(stringOut)

		cmd = exec.Command("lsblk", "-d", "-n", "-o", "ROTA", path)
		out, _ = cmd.CombinedOutput()
		stringOut = string(out)
		halDisks[i].Rotational = strings.TrimSpace(stringOut)

		halDisks[i].PartitionCount = countPartitions(path)
	}

	logrus.Info("Returning disks: ", halDisks)

	return &halDisks
}

func countPartitions(device string) int {
	cmd := exec.Command("lsblk", "-n", "-l", "-p", "-o", "TYPE,NAME", device)

	out, err := cmd.CombinedOutput()

	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	output := string(out)
	logrus.Info("combined out:\n%s\n", output)

	outlines := strings.Split(output, "\n")
	l := len(outlines)

	count := 0
	for i := 0; i < (l - 1); i++ {
		line := outlines[i]
		//fmt.Printf("%d: %s\n", i, line)
		device := strings.Split(line, " ")
		if len(device) != 2 {
			logrus.Error("Failed to parse line %s\n", line)
		}
		devType := device[0]

		if devType == "part" {
			count++
		}
	}

	return count
}
