package base

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type cmdAndResult struct {
	cmdStr string
	strOut string
	strErr string
	err    error
}

func TestRunCmdFromStrWithoutErrors(t *testing.T) {
	var cmdPass = []cmdAndResult{
		{"echo", "\n", "", nil},
		{"echo 123", "123\n", "", nil},
		{"echo 123 asd", "123 asd\n", "", nil},
	}

	for _, test := range cmdPass {
		strOut, strErr, err := RunCmdFromStr(test.cmdStr)
		assert.Nil(t, err, fmt.Sprintf("Unexpected error for cmd: \"%s\". Got error: %v", test.cmdStr, err))
		assert.Equal(t, test.strOut, strOut, fmt.Sprintf("Unexpected stdout for cmd \"%s\".", test.cmdStr))
		assert.Equal(t, test.strErr, strErr, fmt.Sprintf("Unexpected stderr for cmd \"%s\"", test.cmdStr))
	}
}

func TestRunCmdFromStrWitErrors(t *testing.T) {
	cmdErr := cmdAndResult{
		"false",
		"",
		"",
		errors.New("exit status 1"),
	}

	strOut, strErr, err := RunCmdFromStr(cmdErr.cmdStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), cmdErr.err.Error())
	assert.Equal(t, strOut, cmdErr.strOut)
	assert.Equal(t, strErr, cmdErr.strErr)
}
