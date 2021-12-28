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

package mocks

import (
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/stretchr/testify/mock"
)

// LevelSetter is the struct to fully implement CmdExecutor interface
type LevelSetter struct {
	msgLevel logrus.Level
}

// SetLevel sets log Level to a MockExecutor
// Receives logrus Level
func (l LevelSetter) SetLevel(level logrus.Level) {
	l.msgLevel = level
}

// EmptyExecutorSuccess implements CmdExecutor interface for test purposes, each command will finish success
type EmptyExecutorSuccess struct {
	LevelSetter
}

// RunCmd simulates successful execution of a command
// Returns "" as stdout, "" as stderr and nil as error
func (e EmptyExecutorSuccess) RunCmd(interface{}, ...command.Options) (string, string, error) {
	return "", "", nil
}

// RunCmdWithAttempts simulates successful execution of a command with attempts and given timeout between attempts
// Returns "" as stdout, "" as stderr and nil as error
func (e EmptyExecutorSuccess) RunCmdWithAttempts(interface{}, int, time.Duration, ...command.Options) (string, string, error) {
	return "", "", nil
}

// EmptyExecutorFail implements CmdExecutor interface for test purposes, each command will finish with error
type EmptyExecutorFail struct {
	LevelSetter
}

// RunCmd simulates failed execution of a command
// Returns "error happened" as stdout, "error" as stderr and errors.New("error") as error
func (e EmptyExecutorFail) RunCmd(interface{}, ...command.Options) (string, string, error) {
	return "error happened", "error", errors.New("error")
}

// RunCmdWithAttempts simulates failed execution of a command with attempts and given timeout between attempts
// Returns "error happened" as stdout, "error" as stderr and errors.New("error") as error
func (e EmptyExecutorFail) RunCmdWithAttempts(interface{}, int, time.Duration, ...command.Options) (string, string, error) {
	return "error happened", "error", errors.New("error")
}

// CmdOut is the struct for command output
type CmdOut struct {
	Stdout string
	Stderr string
	Err    error
}

// MockExecutor implements CmdExecutor interface, each command will return appropriate key from cmdMap map
// there is ability to return different value for same command if it runs twice, for it
// add this command and result (that expected on second run) in SecondRun map
// when cmd runs first result gets from cmdMap,
// when cmd runs second time and so on results is searching (at first) in SecondRun map
type MockExecutor struct {
	cmdMap map[string]CmdOut
	// contains cmd and results if we run one cmd twice
	secondRun map[string]CmdOut
	// contains cmd that has already run
	runBefore []string
	// if command doesn't in cmdMap RunCmd method will fail or success with empty output
	// based on that parameter
	successIfNotFound bool

	LevelSetter
}

// NewMockExecutor is the constructor for MockExecutor struct
// Receives map which contains commands as keys and their outputs as values
// Returns an instance of MockExecutor
func NewMockExecutor(m map[string]CmdOut) *MockExecutor {
	return &MockExecutor{
		cmdMap:    m,
		secondRun: make(map[string]CmdOut),
		runBefore: make([]string, 0),
	}
}

// SetMap sets map which contains commands as keys and their outputs as values to the MockExecutor
func (e *MockExecutor) SetMap(m map[string]CmdOut) {
	e.cmdMap = m
}

// GetMap returns command map from MockExecutor
func (e *MockExecutor) GetMap() map[string]CmdOut {
	return e.cmdMap
}

// SetSuccessIfNotFound sets MockExecutor mode when it returns success output even if a command wasn't found in map
func (e *MockExecutor) SetSuccessIfNotFound(val bool) {
	e.successIfNotFound = val
}

// AddSecondRun adds command output - res for command - cmd for the second execution
func (e *MockExecutor) AddSecondRun(cmd string, res CmdOut) {
	e.secondRun[cmd] = res
}

// RunCmd simulates execution of a command. If a command is in cmdMap then return value as an output for it.
// If the command ran before then trying to return output from secondRun map if it set.
// Receives cmd as interface and cast it to a string
// Returns stdout, stderr, error for a given command
func (e *MockExecutor) RunCmd(cmd interface{}, opts ...command.Options) (string, string, error) {
	cmdStr := cmd.(string)
	if len(e.secondRun) > 0 {
		for _, c := range e.runBefore {
			if c == cmdStr {
				if _, ok := e.secondRun[c]; !ok {
					break
				}
				res := e.secondRun[c]
				return res.Stdout, res.Stderr, res.Err
			}
		}
	}
	res, ok := e.cmdMap[cmdStr]
	if !ok {
		if e.successIfNotFound {
			return "", "", nil
		}
		return "", "", fmt.Errorf("unable find results for key %s, current cmd map: %v", cmdStr, e.cmdMap)
	}
	e.runBefore = append(e.runBefore, cmdStr)
	return res.Stdout, res.Stderr, res.Err
}

// RunCmdWithAttempts simulates execution of a command. Execute RunCmd.
// Receives cmd as interface, number of attempts, timeout
// Returns stdout, stderr, error for a given command
func (e *MockExecutor) RunCmdWithAttempts(cmd interface{}, attempts int, timeout time.Duration, opts ...command.Options) (string, string, error) {
	return e.RunCmd(cmd)
}

// RunCmd is the name of CmdExecutor method name
var (
	RunCmd             = "RunCmd"
	RunCmdWithAttempts = "RunCmdWithAttempts"
)

// GoMockExecutor implements CmdExecutor based on stretchr/testify/mock
type GoMockExecutor struct {
	mock.Mock
	LevelSetter
}

// RunCmdWithAttempts simulates execution of a command with OnCommandWithAttempts where user can set what the method should return
func (g *GoMockExecutor) RunCmdWithAttempts(cmd interface{}, attempts int, timeout time.Duration, opts ...command.Options) (string, string, error) {
	args := g.Mock.Called(cmd.(string), attempts, timeout)
	return args.String(0), args.String(1), args.Error(2)
}

// RunCmd simulates execution of a command with OnCommand where user can set what the method should return
func (g *GoMockExecutor) RunCmd(cmd interface{}, opts ...command.Options) (string, string, error) {
	args := g.Mock.Called(cmd)
	return args.String(0), args.String(1), args.Error(2)
}

// OnCommand is the method of mock.Mock where user can set what to return on specified command
// For example e.OnCommand("/sbin/lvm pvcreate --yes /dev/sda").Return("", "", errors.New("pvcreate failed"))
// Returns mock.Call where need to set what to return with Return() method
func (g *GoMockExecutor) OnCommand(cmd string) *mock.Call {
	return g.On(RunCmd, cmd)
}

// OnCommandWithAttempts is the method of mock.Mock where user can set what to return on specified command
// For example e.OnCommandWithAttempts("/sbin/lvm pvcreate --yes /dev/sda", 5, time.Second).Return("", "", errors.New("pvcreate failed"))
// Returns mock.Call where need to set what to return with Return() method
func (g *GoMockExecutor) OnCommandWithAttempts(cmd string, attempts int, timeout time.Duration) *mock.Call {
	return g.On(RunCmdWithAttempts, cmd, attempts, timeout)
}
