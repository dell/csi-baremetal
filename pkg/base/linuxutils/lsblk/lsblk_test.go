package lsblk

import (
	"errors"
	"fmt"
	"testing"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	testLogger    = logrus.New()
	allDevicesCmd = fmt.Sprintf(CmdTmpl, "")
)

func TestLinuxUtils_LsblkSuccess(t *testing.T) {

	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", allDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	l := NewLSBLK(e, testLogger)

	out, err := l.GetBlockDevices("")
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, 2, len(out))

}

func TestLinuxUtils_LsblkFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSBLK(e, testLogger)

	e.On(mocks.RunCmd, allDevicesCmd).Return("not a json", "", nil).Times(1)
	out, err := l.GetBlockDevices("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to unmarshal output to BlockDevice instance")

	expectedError := errors.New("lsblk failed")
	e.On(mocks.RunCmd, allDevicesCmd).Return("", "", expectedError).Times(1)
	out, err = l.GetBlockDevices("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Equal(t, expectedError, err)

	e.On(mocks.RunCmd, allDevicesCmd).Return(mocks.NoLsblkKeyStr, "", nil).Times(1)
	out, err = l.GetBlockDevices("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unexpected lsblk output format")
}
