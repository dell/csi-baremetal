package util

import (
	"bytes"
	"os/exec"

	"github.com/sirupsen/logrus"
)

// execute provided command and collect stdout and stderr separately
// returns stdout, stderr and error
// both stdout and stderr could be splitted by \n
func RunCmdAndCollectOutput(cmd *exec.Cmd) (string, error) {
	ll := logrus.WithField("operation", "exec cmd")

	var stdout, stderr bytes.Buffer
	var strOut, strErr string

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	ll.Info(cmd.Args)
	err := cmd.Run()

	strOut = stdout.String()
	strErr = stderr.String()

	ll.Infof("Output: %s", strOut)
	// some command my write something in stderr and have exit code == 0
	if err != nil {
		ll.Warnf("StdErr: %s. Error: %v", strErr, err)
	}

	return strOut, err
}

//FormatCapacity format capacity of disk:
func FormatCapacity(size float64, unit string) int64 {
	switch unit {
	case "K":
		size *= 1024
	case "M":
		size *= 1024 * 1024
	case "G":
		size *= 1024 * 1024 * 1024
	case "T":
		size *= 1024 * 1024 * 1024 * 1024
	default:
		return int64(size)
	}

	return int64(size)
}

func WipeFS(dev string) error {
	_, err := RunCmdAndCollectOutput(exec.Command("wipefs", "-af", dev)) // TODO: do not use -f
	if err != nil {
		logrus.Fatalf("failed to wipe filesystem from %s", dev)
	}

	return nil
}
