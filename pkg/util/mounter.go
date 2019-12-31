package util

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// TODO: try to using native Kubernetes libs

// Mount is a function for mounting block devices
func Mount(source, target string) error {
	logger.Info("Running ls -lah ", source)
	cmd := exec.Command("ls", "-lah", source)
	out, err := cmd.CombinedOutput()

	if err != nil {
		logger.Fatalf("cmd.Run() failed with %s, output: %s", err, out)
	}

	logger.Info("Running [mkfs.xfs -f ", source, "]")
	cmd = exec.Command("mkfs.xfs", "-f", source)
	out, err = cmd.CombinedOutput()

	if err != nil {
		logger.Fatalf("cmd.Run() failed with %s, output: %s", err, out)
	}

	cmd = exec.Command("mkdir", "-p", target)
	out, err = cmd.CombinedOutput()

	if err != nil {
		logger.Fatalf("cmd.Run() failed with %s, output: %s", err, out)
	}

	cmd = exec.Command("mount", source, target)
	out, err = cmd.CombinedOutput()

	if err != nil {
		logger.Fatalf("cmd.Run() failed with %s, output: %s", err, out)
	}

	return nil
}

// Unmount is a function for unmounting block devices
func Unmount(target string) error {
	umountCmd := "umount"

	if target == "" {
		return errors.New("target is not specified for unmounting the volume")
	}

	logger.WithFields(logrus.Fields{
		"cmd":  umountCmd,
		"args": target,
	}).Info("executing umount command")

	out, err := exec.Command(umountCmd, target).CombinedOutput()

	if err != nil {
		return fmt.Errorf("unmounting failed: %v cmd: '%s %s' output: %q",
			err, umountCmd, target, string(out))
	}

	return nil
}

// Return true if source is mounted to target. Otherwise return false
func IsMountedBockDevice(source string, target string) bool {
	cmd := exec.Command("lsblk", "-d", "-n", "-o", "MOUNTPOINT", source)

	out, _ := cmd.CombinedOutput()

	mountPoint := strings.TrimSpace(string(out))

	logger.Infof("MOUNTPOINT for %s is %s", source, mountPoint)

	return mountPoint == target
}
