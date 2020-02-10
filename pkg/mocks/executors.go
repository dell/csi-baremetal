package mocks

import (
	"errors"
	"fmt"

	"github.com/stretchr/testify/mock"

	"github.com/sirupsen/logrus"
)

type LoggerSetter struct {
	log *logrus.Entry
}

func (l LoggerSetter) SetLogger(logger *logrus.Logger) {
	l.log = logger.WithField("component", "MockExecutor")
}

// Implements CmdExecutor interface, each command will finish success
type EmptyExecutorSuccess struct {
	LoggerSetter
}

func (e EmptyExecutorSuccess) RunCmd(interface{}) (string, string, error) {
	return "Stdout", "", nil
}

// Implements CmdExecutor interface, each command will finish with error
type EmptyExecutorFail struct {
	LoggerSetter
}

func (e EmptyExecutorFail) RunCmd(interface{}) (string, string, error) {
	return "error happened", "error", errors.New("error")
}

type CmdOut struct {
	Stdout string
	Stderr string
	Err    error
}

// Implements CmdExecutor interface, each command will return appropriate key from cmdMap map
// there is ability to return different value for same command if it runs twice, for it
// add this command and result (that expected on second run) in SecondRun map
// when cmd runs first result gets from cmdMap,
// when cmd runs second time and so on results is searching (at first) in SecondRun map
type MockExecutor struct {
	cmdMap map[string]CmdOut
	LoggerSetter
	// contains cmd and results if we run one cmd twice
	secondRun map[string]CmdOut
	// contains cmd that has already run
	runBefore []string
	// if command doesn't in cmdMap RunCmd method will fail or success with empty output
	// based on that parameter
	successIfNotFound bool
}

func NewMockExecutor(m map[string]CmdOut) *MockExecutor {
	return &MockExecutor{
		cmdMap:    m,
		secondRun: make(map[string]CmdOut),
		runBefore: make([]string, 0),
	}
}

func (e *MockExecutor) SetMap(m map[string]CmdOut) {
	e.cmdMap = m
}

func (e *MockExecutor) GetMap() map[string]CmdOut {
	return e.cmdMap
}

func (e *MockExecutor) SetSuccessIfNotFound(val bool) {
	e.successIfNotFound = val
}

func (e *MockExecutor) AddSecondRun(cmd string, res CmdOut) {
	e.secondRun[cmd] = res
}

func (e *MockExecutor) RunCmd(cmd interface{}) (string, string, error) {
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

// RunCmd is the name of CmdExecutor method name
var RunCmd = "RunCmd"

// GoMockExecutor implement CmdExecutor
type GoMockExecutor struct {
	mock.Mock
	LoggerSetter
}

func (g *GoMockExecutor) RunCmd(cmd interface{}) (string, string, error) {
	args := g.Mock.Called(cmd.(string))
	return args.String(0), args.String(1), args.Error(2)
}

func (g *GoMockExecutor) OnCommand(cmd string) *mock.Call {
	return g.On(RunCmd, cmd)
}
