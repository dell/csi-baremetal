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
	SetLogger(logger *logrus.Logger)
}

type Executor struct {
	log *logrus.Entry
}

func (e *Executor) SetLogger(logger *logrus.Logger) {
	e.log = logger.WithField("component", "Executor")
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
	var (
		level               = logrus.InfoLevel
		stdout, stderr      bytes.Buffer
		stdErrPart, errPart string
	)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	outStr, errStr = stdout.String(), stderr.String()
	// construct log message based on output and error
	if len(errStr) > 0 {
		stdErrPart = fmt.Sprintf(", stderr: %s", errStr)
	}
	if err != nil {
		errPart = fmt.Sprintf(", Error: %v", err)
		level = logrus.ErrorLevel
	}
	e.log.WithField("cmd", strings.Join(cmd.Args, " ")).
		Logf(level, "stdout: %s%s%s", outStr, stdErrPart, errPart)
	return outStr, errStr, err
}
