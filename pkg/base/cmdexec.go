package base

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

type CmdExecutor interface {
	RunCmd(cmd interface{}) (string, string, error)
}

type Executor struct {
}

func (e *Executor) RunCmd(cmd interface{}) (string, string, error) {
	if cmdStr, ok := cmd.(string); ok {
		return e.runCmdFromStr(cmdStr)
	}
	if cmdObj, ok := cmd.(*exec.Cmd); ok {
		return e.runCmdFromCmdObj(cmdObj)
	}
	return "", "", fmt.Errorf("could not interpret command from %v", cmd)
}

// runCmdFromStr gets command as a string, like: "netstat -n -a -p" and transform it into exec.Command type
// and runs runCmdFromCmdObj(cmd)
// commands like: bash -c "something -param" are not supported
func (e *Executor) runCmdFromStr(cmd string) (string, string, error) {
	fields := strings.Fields(cmd)
	name := fields[0]
	if len(fields) > 1 {
		return e.runCmdFromCmdObj(exec.Command(name, fields[1:]...))
	}
	return e.runCmdFromCmdObj(exec.Command(name))
}

// runCmdFromCmdObj runs command and return stdout, stderr and error
func (e *Executor) runCmdFromCmdObj(cmd *exec.Cmd) (outStr string, errStr string, err error) {
	ll := logrus.WithField("cmd", strings.Join(cmd.Args, " "))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		ll.Errorf("got error: %v", err)
	}
	outStr, errStr = stdout.String(), stderr.String()
	errPart := ""
	if len(errStr) > 0 {
		errPart = fmt.Sprintf(", stderr: %s", errStr)
	}
	ll.Infof("stdout: %s%s", outStr, errPart)
	return outStr, errStr, err
}
