package loopbackmgr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var logger = logrus.New()

func TestLoopBackManager_getLoopBackDeviceName(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)

	file := "/tmp/test"
	loop := "/dev/loop18"
	mockexec.On("RunCmd", fmt.Sprintf(checkLoopBackDeviceCmdTmpl, file)).
		Return(loop+": []: ("+file+")", "", nil)
	device, err := manager.GetLoopBackDeviceName(file)

	assert.Equal(t, "/dev/loop18", device)
	assert.Nil(t, err)
}

func TestLoopBackManager_getLoopBackDeviceName_NotFound(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)

	file := "/tmp/test"
	mockexec.On("RunCmd", fmt.Sprintf(checkLoopBackDeviceCmdTmpl, file)).
		Return("", "", nil)
	device, err := manager.GetLoopBackDeviceName(file)
	assert.Equal(t, "", device)
	assert.Nil(t, err)
}

func TestLoopBackManager_getLoopBackDeviceName_Fail(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)

	file := "/tmp/test"
	error := errors.New("losetup: command not found")
	mockexec.On("RunCmd", fmt.Sprintf(checkLoopBackDeviceCmdTmpl, file)).
		Return("", "", error)
	device, err := manager.GetLoopBackDeviceName(file)
	assert.Equal(t, "", device)
	assert.Equal(t, error, err)
}

func TestLoopBackManager_CleanupLoopDevices(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)

	for _, device := range manager.devices {
		mockexec.On("RunCmd", fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device.devicePath)).
			Return("", "", nil)
	}

	manager.CleanupLoopDevices()
}
