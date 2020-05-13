package loopbackmgr

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
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

func TestLoopBackManager_UpdateDevicesFromLocalConfig(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)

	manager.updateDevicesFromConfig()

	assert.Equal(t, defaultNumberOfDevices, len(manager.devices))
}

func TestLoopBackManager_UpdateDevicesFromSetConfig(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)
	testSN := "testSN"
	testNodeID := "testNode"
	testConfigPath := "/tmp/config.yaml"

	config := []byte("defaultDrivePerNodeCount: 3")
	err := ioutil.WriteFile(testConfigPath, config, 0777)
	assert.Nil(t, err)
	defer func() {
		_ = os.Remove(testConfigPath)
	}()

	manager.readAndSetConfig(testConfigPath)
	manager.updateDevicesFromConfig()

	assert.Equal(t, 3, len(manager.devices))

	manager.nodeID = testNodeID
	config = []byte("nodes:\n" +
		fmt.Sprintf("- nodeID: %s\n", testNodeID) +
		fmt.Sprintf("  driveCount: %d\n", 5) +
		"  drives:\n" +
		fmt.Sprintf("  - serialNumber: %s\n", testSN))
	err = ioutil.WriteFile(testConfigPath, config, 0777)
	assert.Nil(t, err)

	manager.readAndSetConfig(testConfigPath)
	manager.updateDevicesFromConfig()

	assert.Equal(t, 5, len(manager.devices))

	found := false
	for _, device := range manager.devices {
		if device.SerialNumber == testSN {
			found = true
		}
	}

	assert.Equal(t, true, found)
}

func TestLoopBackManager_overrideDevicesFromNodeConfig(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)

	// Initialize manager with local default settings
	manager.updateDevicesFromConfig()

	assert.Equal(t, defaultNumberOfDevices, len(manager.devices))

	indexOfDeviceToOverride := 0
	newVID := "newVID"
	// The first device should be overrode
	// The second device should be added
	devices := []*LoopBackDevice{
		{SerialNumber: manager.devices[indexOfDeviceToOverride].SerialNumber, VendorID: newVID},
		{SerialNumber: "newDevice"},
	}

	manager.overrideDevicesFromNodeConfig(defaultNumberOfDevices+1, devices)

	assert.Equal(t, manager.devices[indexOfDeviceToOverride].VendorID, newVID)
	assert.Equal(t, defaultNumberOfDevices+1, len(manager.devices))
}

func TestLoopBackManager_overrideDeviceWithSizeChanging(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)
	// Initialize manager with local default settings
	manager.updateDevicesFromConfig()

	assert.Equal(t, defaultNumberOfDevices, len(manager.devices))

	indexOfDeviceToOverride := 0
	newSize := "200Mi"
	fakeDevicePath := "/dev/loop0"
	fakeFileName := "loopback.img"
	manager.devices[indexOfDeviceToOverride].devicePath = fakeDevicePath
	manager.devices[indexOfDeviceToOverride].fileName = fakeFileName

	mockexec.On("RunCmd", fmt.Sprintf(detachLoopBackDeviceCmdTmpl, fakeDevicePath)).
		Return("", "", nil)
	mockexec.On("RunCmd", fmt.Sprintf(deleteFileCmdTmpl, fakeFileName)).
		Return("", "", nil)

	// Change size of device to override
	devices := []*LoopBackDevice{
		{SerialNumber: manager.devices[indexOfDeviceToOverride].SerialNumber, Size: newSize},
	}

	manager.overrideDevicesFromNodeConfig(defaultNumberOfDevices, devices)
	assert.Equal(t, manager.devices[indexOfDeviceToOverride].Size, newSize)
}

func TestLoopBackManager_GetDrivesList(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	var manager = NewLoopBackManager(mockexec, logger)
	fakeDevicePath := "/dev/loop"

	manager.updateDevicesFromConfig()
	for i, device := range manager.devices {
		device.devicePath = fmt.Sprintf(fakeDevicePath+"%d", i)
	}
	indexOfDriveToOffline := 0
	manager.devices[indexOfDriveToOffline].Removed = true
	drives, err := manager.GetDrivesList()

	assert.Nil(t, err)
	assert.Equal(t, defaultNumberOfDevices, len(drives))
	assert.Equal(t, apiV1.DriveStatusOffline, drives[indexOfDriveToOffline].Status)
}
