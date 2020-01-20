package base

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// RunCmdFromStr gets command as a string, like: "netstat -n -a -p" and transform it into exec.Command type
// and runs RunCmdFromCmdObj(cmd)
// commands like: bash -c "something -param" are not supported
func RunCmdFromStr(cmd string) (string, string, error) {
	fields := strings.Fields(cmd)
	name := fields[0]
	if len(fields) > 1 {
		return RunCmdFromCmdObj(exec.Command(name, fields[1:]...))
	}
	return RunCmdFromCmdObj(exec.Command(name))
}

// RunCmdFromCmdObj runs command and return stdout, stderr and error
func RunCmdFromCmdObj(cmd *exec.Cmd) (outStr string, errStr string, err error) {
	ll := logrus.WithField("cmd", strings.Join(cmd.Args, " "))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		ll.Errorf("got error: %v", err)
	}
	outStr, errStr = stdout.String(), stderr.String()
	ll.Infof("stdout: %s, stderr: %s", outStr, errStr)
	return outStr, errStr, err
}
