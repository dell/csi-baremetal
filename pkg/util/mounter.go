package util

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/sirupsen/logrus"
)

//TODO: try to using native Kubernetes libs

//func format(blockDevice string) error {
//	return nil
//}

// Mount is a function for mounting block devices
func Mount(blockDevice, source, target string) error {
	logrus.Info("Running ls -lah ", blockDevice)
	cmd := exec.Command("ls", "-lah", blockDevice)
	out, err := cmd.CombinedOutput()

	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	/*logrus.Info("Running [parted -s ", blockDevice, " mklabel gpt]")
	cmd = exec.Command("parted", "-s", blockDevice, "mklabel", "gpt")
	out, err = cmd.CombinedOutput()
	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	logrus.Info("Running [parted -s ", blockDevice,
	" mkpart --align optimal ECS:unassign:AAAAAAAAAAAAAAAAAAAAAA 0% 100%]")
	cmd = exec.Command("parted", blockDevice, "mkpart", "--align", "optimal",
	"ECS:unassign:AAAAAAAAAAAAAAAAAAAAAA", "0%", "100%")
	out, err = cmd.CombinedOutput()
	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	for i := 1; i <= 5; i++ {
		logrus.Info("Running [partprobe], attempt ", i)
		cmd = exec.Command("partprobe")
		out, err = cmd.CombinedOutput()
		if err != nil {
			logrus.Error("cmd.Run() failed with %s, output%s\n", err, out)
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}*/

	logrus.Info("Running [mkfs.xfs -f ", source, "]")
	cmd = exec.Command("mkfs.xfs", "-f", source)
	out, err = cmd.CombinedOutput()

	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	cmd = exec.Command("mkdir", "-p", target)
	out, err = cmd.CombinedOutput()

	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	cmd = exec.Command("mount", source, target)
	out, err = cmd.CombinedOutput()

	if err != nil {
		logrus.Fatalf("cmd.Run() failed with %s, output%s\n", err, out)
	}

	return nil
}

// Unmount is a function for unmounting block devices
func Unmount(target string) error {
	umountCmd := "umount"

	if target == "" {
		return errors.New("target is not specified for unmounting the volume")
	}

	umountArgs := []string{target}

	logrus.WithFields(logrus.Fields{
		"cmd":  umountCmd,
		"args": umountArgs,
	}).Info("executing umount command")

	out, err := exec.Command(umountCmd, umountArgs...).CombinedOutput()

	if err != nil {
		return fmt.Errorf("unmounting failed: %v cmd: '%s %s' output: %q",
			err, umountCmd, target, string(out))
	}

	return nil
}

//func isFormatted(source string) (bool, error) {
//	return true, nil
//}
//
//func isMounted(target string) (bool, error) {
//	return false, nil
//}
