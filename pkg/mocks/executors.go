package mocks

import (
	"errors"
)

// Implements CmdExecutor interface, each command will finish success
type EmptyExecutorSuccess struct{}

func (e EmptyExecutorSuccess) RunCmd(interface{}) (string, string, error) {
	return "Stdout", "", nil
}

// Implements CmdExecutor interface, each command will finish with error
type EmptyExecutorFail struct{}

func (e EmptyExecutorFail) RunCmd(interface{}) (string, string, error) {
	return "error happened", "error", errors.New("error")
}

type CmdOut struct {
	Stdout string
	Stderr string
	Err    error
}

// Implements CmdExecutor interface, each command will return appropriate key from cmdMap map
type MockExecutor struct {
	cmdMap map[string]CmdOut
}

func NewMockExecutor(m map[string]CmdOut) MockExecutor {
	return MockExecutor{
		cmdMap: m,
	}
}
func (e *MockExecutor) SetMap(m map[string]CmdOut) {
	e.cmdMap = m
}
func (e MockExecutor) RunCmd(cmd interface{}) (string, string, error) {
	res := e.cmdMap[cmd.(string)]
	return res.Stdout, res.Stderr, res.Err
}
