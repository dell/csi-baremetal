package operations

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/mocks"
)

func TestWbt_SetValue(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		var (
			output          = "75000"
			emptyErr        = ""
			device          = "sdb"
			value    uint32 = 0
			cmdCheck        = fmt.Sprintf(checkWbtValueCmd, device)
			cmdSet          = exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(setWbtValueCmd, value, device))
		)

		e := &mocks.GoMockExecutor{}
		e.On("RunCmd", cmdCheck).Return(output, emptyErr, nil)
		e.On("RunCmd", cmdSet).Return("", emptyErr, nil)

		wbt := NewWbt(e)
		err := wbt.SetValue(device, value)
		assert.Nil(t, err)
	})
	t.Run("passed value equals to current", func(t *testing.T) {
		var (
			output          = "75000"
			emptyErr        = ""
			device          = "sdb"
			value    uint32 = 75000
			cmdCheck        = fmt.Sprintf(checkWbtValueCmd, device)
		)

		e := &mocks.GoMockExecutor{}
		e.On("RunCmd", cmdCheck).Return(output, emptyErr, nil)

		wbt := NewWbt(e)
		err := wbt.SetValue(device, value)
		assert.Nil(t, err)
	})
	t.Run("check fail", func(t *testing.T) {
		var (
			output          = "75000"
			errStr          = "some error"
			device          = "sdb"
			value    uint32 = 0
			cmdCheck        = fmt.Sprintf(checkWbtValueCmd, device)
		)

		e := &mocks.GoMockExecutor{}
		e.On("RunCmd", cmdCheck).Return(output, errStr, fmt.Errorf("err: %s", errStr))

		wbt := NewWbt(e)
		err := wbt.SetValue(device, value)
		assert.NotNil(t, err)
	})
	t.Run("check fail with invalid arg", func(t *testing.T) {
		var (
			output          = "75000"
			errStr          = invalidArgError
			emptyErr        = ""
			device          = "sdb"
			value    uint32 = 0
			cmdCheck        = fmt.Sprintf(checkWbtValueCmd, device)
			cmdSet          = exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(setWbtValueCmd, value, device))
		)

		e := &mocks.GoMockExecutor{}
		e.On("RunCmd", cmdCheck).Return(output, errStr, fmt.Errorf("err: %s", errStr))
		e.On("RunCmd", cmdSet).Return("", emptyErr, nil)

		wbt := NewWbt(e)
		err := wbt.SetValue(device, value)
		assert.Nil(t, err)
	})
	t.Run("set fail", func(t *testing.T) {
		var (
			output          = "75000"
			errStr          = "some error"
			emptyErr        = ""
			device          = "sdb"
			value    uint32 = 0
			cmdCheck        = fmt.Sprintf(checkWbtValueCmd, device)
			cmdSet          = exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(setWbtValueCmd, value, device))
		)

		e := &mocks.GoMockExecutor{}
		e.On("RunCmd", cmdCheck).Return(output, emptyErr, nil)
		e.On("RunCmd", cmdSet).Return("", errStr, fmt.Errorf("err: %s", errStr))

		wbt := NewWbt(e)
		err := wbt.SetValue(device, value)
		assert.NotNil(t, err)
	})
}

func TestWbt_RestoreDefault(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		var (
			emptyErr = ""
			device   = "sdb"
			cmdSet   = exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(restoreWbtValueCmd, device))
		)

		e := &mocks.GoMockExecutor{}
		e.On("RunCmd", cmdSet).Return("", emptyErr, nil)

		wbt := NewWbt(e)
		err := wbt.RestoreDefault(device)
		assert.Nil(t, err)
	})
	t.Run("fail", func(t *testing.T) {
		var (
			errStr = "some error"
			device = "sdb"
			cmdSet = exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(restoreWbtValueCmd, device))
		)

		e := &mocks.GoMockExecutor{}
		e.On("RunCmd", cmdSet).Return("", errStr, fmt.Errorf("err: %s", errStr))

		wbt := NewWbt(e)
		err := wbt.RestoreDefault(device)
		assert.NotNil(t, err)
	})
}
