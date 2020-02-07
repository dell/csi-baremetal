package base

import (
	"testing"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"github.com/stretchr/testify/assert"
)

var luLogger = logrus.New()

func TestLinuxUtils_LsblkSuccess(t *testing.T) {
	e := mocks.NewMockExecutor(map[string]mocks.CmdOut{LsblkCmd: mocks.LsblkTwoDevices})
	l := NewLinuxUtils(e, luLogger)

	out, err := l.Lsblk(DriveTypeDisk)
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, 2, len(*out))

}

func TestLinuxUtils_LsblkFail(t *testing.T) {
	e1 := mocks.EmptyExecutorSuccess{}
	l := NewLinuxUtils(e1, luLogger)

	out, err := l.Lsblk(DriveTypeDisk)
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "invalid character")

	e2 := mocks.EmptyExecutorFail{}
	l = NewLinuxUtils(e2, luLogger)
	out, err = l.Lsblk(DriveTypeDisk)
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error")

	e3 := mocks.NewMockExecutor(map[string]mocks.CmdOut{LsblkCmd: mocks.NoLsblkKey})
	l = NewLinuxUtils(e3, luLogger)
	out, err = l.Lsblk(DriveTypeDisk)
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unexpected lsblk output format")
}
