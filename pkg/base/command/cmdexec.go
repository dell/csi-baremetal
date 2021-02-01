/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package command

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// CmdExecutor is the interface for executor that runs linux commands with RunCmd
type CmdExecutor interface {
	RunCmd(cmd interface{}) (string, string, error)
	SetLogger(logger *logrus.Logger)
	SetLevel(level logrus.Level)
	RunCmdWithAttempts(cmd interface{}, attempts int, timeout time.Duration) (string, string, error)
}

// Executor is the implementation of CmdExecutor based on os/exec package
type Executor struct {
	log      *logrus.Entry
	msgLevel logrus.Level
}

// SetLogger sets logrus logger to Executor struct
// Receives logrus logger
func (e *Executor) SetLogger(logger *logrus.Logger) {
	e.log = logger.WithField("component", "Executor")
}

// SetLevel sets logrus Level to Executor msgLevel field
// Receives logrus Level
func (e *Executor) SetLevel(level logrus.Level) {
	e.msgLevel = level
}

// RunCmdWithAttempts runs specified command on OS with given attempts and timeout between attempts
// Receives command as empty interface, It could be string or instance of exec.Cmd; number of attempts; timeout.
// Returns stdout as string, stderr as string and golang error if something went wrong
func (e *Executor) RunCmdWithAttempts(cmd interface{}, attempts int, timeout time.Duration) (string, string, error) {
	ll := e.log.WithFields(logrus.Fields{
		"method": "RunCmdWithAttempts",
	})
	var (
		stdout string
		stderr string
		err    error
	)
	for i := 0; i < attempts; i++ {
		if stdout, stderr, err := e.RunCmd(cmd); err == nil {
			return stdout, stderr, err
		}
		ll.Warnf("Unable to execute cmd: %v. Attempt %d out of %d.", err, i, attempts)
		<-time.After(timeout)
	}
	errMsg := fmt.Errorf("failed to execute command after %d attempt, error: %v", attempts, err)
	return stdout, stderr, errMsg
}

// RunCmd runs specified command on OS
// Receives command as empty interface. It could be string or instance of exec.Cmd
// Returns stdout as string, stderr as string and golang error if something went wrong
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
// Receives command as a string like: bash -c "something -param" are not supported
// Returns stdout as string, stderr as string and golang error if something went wrong
func (e *Executor) runCmdFromStr(cmd string) (string, string, error) {
	fields := strings.Fields(cmd)
	name := fields[0]
	if len(fields) > 1 {
		return e.runCmdFromCmdObj(exec.Command(name, fields[1:]...))
	}
	return e.runCmdFromCmdObj(exec.Command(name))
}

// runCmdFromCmdObj runs command based on exec.Cmd
// Receives instance of exec.Cmd
// Returns stdout as string, stderr as string and golang error if something went wrong
func (e *Executor) runCmdFromCmdObj(cmd *exec.Cmd) (outStr string, errStr string, err error) {
	var (
		level               = e.msgLevel
		stdout, stderr      bytes.Buffer
		stdErrPart, errPart string
	)
	if level == 0 {
		level = logrus.DebugLevel
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmdStartTime := time.Now()
	err = cmd.Run()
	cmdDuration := time.Since(cmdStartTime)

	outStr, errStr = stdout.String(), stderr.String()
	// construct log message based on output and error
	if len(errStr) > 0 {
		stdErrPart = fmt.Sprintf(", stderr: %s", errStr)
		level = logrus.WarnLevel
	}
	if err != nil {
		errPart = fmt.Sprintf(", Error: %v", err)
		level = logrus.ErrorLevel
	}
	e.log.WithFields(logrus.Fields{
		"cmd":         strings.Join(cmd.Args, " "),
		"duration":    cmdDuration.String(),
		"duration_ns": cmdDuration.Nanoseconds()}).
		Logf(level, "stdout: %s%s%s", outStr, stdErrPart, errPart)
	return outStr, errStr, err
}
