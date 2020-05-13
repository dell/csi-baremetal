package command

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type cmdAndResult struct {
	cmd    interface{}
	strOut string
	strErr string
	err    error
}

func TestExecutorFromStrWithoutError(t *testing.T) {
	// here we run some real shell command that wouldn't work on windows os
	if runtime.GOOS == "windows" {
		return
	}

	var cmdPass = []cmdAndResult{
		{"echo", "\n", "", nil},
		{"echo 123", "123\n", "", nil},
		{"echo 123 asd", "123 asd\n", "", nil},
		{exec.Command("true"), "", "", nil},
	}

	e := Executor{}
	e.SetLogger(logrus.New())

	for _, test := range cmdPass {
		strOut, strErr, err := e.RunCmd(test.cmd)
		assert.Nil(t, err, fmt.Sprintf("Unexpected error for cmd: \"%s\". Got error: %v", test.cmd, err))
		assert.Equal(t, test.strOut, strOut, fmt.Sprintf("Unexpected stdout for cmd \"%s\".", test.cmd))
		assert.Equal(t, test.strErr, strErr, fmt.Sprintf("Unexpected stderr for cmd \"%s\"", test.cmd))
	}
}

func TestExecutorFromStrAndExpectError(t *testing.T) {
	// here we run some real shell command that wouldn't work on windows os
	if runtime.GOOS == "windows" {
		return
	}

	var cmdErr = []cmdAndResult{
		{"false", "", "", errors.New("exit status 1")},
		{2, "", "", errors.New("could not interpret command from 2")},
	}

	e := Executor{}
	e.SetLogger(logrus.New())

	for _, test := range cmdErr {
		strOut, strErr, err := e.RunCmd(test.cmd)
		assert.Equal(t, test.strOut, strOut, fmt.Sprintf("Unexpected stdout for cmd \"%s\".", test.cmd))
		assert.Equal(t, test.strErr, strErr, fmt.Sprintf("Unexpected stderr for cmd \"%s\"", test.cmd))
		assert.Contains(t, err.Error(), test.err.Error())
	}
}
