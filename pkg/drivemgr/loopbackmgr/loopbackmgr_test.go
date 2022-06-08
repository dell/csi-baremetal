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

package loopbackmgr

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

var logger = logrus.New()

func TestLoopBackManager_GetBackFileToLoopMap(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
	testData := `NAME	BACK-FILE
/dev/loop1 /root/test2.img
/dev/loop33 /root/test2.img
/dev/loop95 /root/test96.img
/dev/loop101 /foobar.img (deleted)
/dev/loop102 /foo bar.img
`
	mockexec.On("RunCmd", readLoopBackDevicesMappingCmd).
		Return(testData, "", nil)
	mapping, err := manager.GetBackFileToLoopMap()

	assert.Equal(t, []string{"/dev/loop95"}, mapping["/root/test96.img"])
	assert.Equal(t, []string{"/dev/loop1", "/dev/loop33"}, mapping["/root/test2.img"])
	assert.Equal(t, []string{"/dev/loop102"}, mapping["/foo bar.img"])
	assert.Equal(t, []string{"/dev/loop101"}, mapping["/foobar.img (deleted)"])
	assert.Nil(t, err)
}

func TestLoopBackManager_GetBackFileToLoopMap_Empty(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
	mockexec.On("RunCmd", readLoopBackDevicesMappingCmd).
		Return("", "", nil)
	mapping, err := manager.GetBackFileToLoopMap()

	assert.Empty(t, mapping)
	assert.Nil(t, err)
}

func TestLoopBackManager_GetBackFileToLoopMap_InvalidData(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
	mockexec.On("RunCmd", readLoopBackDevicesMappingCmd).
		Return("\ninvalid\ndata  data data", "", nil)
	_, err := manager.GetBackFileToLoopMap()
	assert.NotNil(t, err)
}

func TestLoopBackManager_CleanupLoopDevices(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)

	for _, device := range manager.devices {
		mockexec.On("RunCmd", fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device.devicePath)).
			Return("", "", nil)
		mockexec.On("RunCmd", fmt.Sprintf(deleteFileCmdTmpl, device.fileName)).
			Return("", "", nil)
	}

	manager.CleanupLoopDevices()
}

func TestLoopBackManager_UpdateDevicesFromLocalConfig(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)

	manager.updateDevicesFromConfig()

	assert.Equal(t, defaultNumberOfDevices, len(manager.devices))
}

func TestLoopBackManager_UpdateDevicesFromSetConfig(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
	testSN := "testSN"
	testNodeID := "testNode"
	testConfigPath := "/tmp/config.yaml"

	config := []byte("defaultDrivePerNodeCount: 3")
	err := ioutil.WriteFile(testConfigPath, config, 0o777)
	assert.Nil(t, err)
	defer func() {
		_ = os.Remove(testConfigPath)
	}()

	manager.readAndSetConfig(testConfigPath)
	manager.updateDevicesFromConfig()

	assert.Equal(t, 3, len(manager.devices))

	manager.nodeName = testNodeID
	config = []byte("nodes:\n" +
		fmt.Sprintf("- nodeID: %s\n", testNodeID) +
		fmt.Sprintf("  driveCount: %d\n", 5) +
		"  drives:\n" +
		fmt.Sprintf("  - serialNumber: %s\n", testSN))
	err = ioutil.WriteFile(testConfigPath, config, 0o777)
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

func TestLoopBackManager_updateDevicesFromSetConfigWithSize(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}

	manager := NewLoopBackManager(mockexec, "", "", logger)

	config := []byte("defaultDriveSize: 30Mi \ndefaultDrivePerNodeCount: 3")
	testConfigPath := "/tmp/config.yaml"
	err := ioutil.WriteFile(testConfigPath, config, 0o777)
	assert.Nil(t, err)

	defer func() {
		_ = os.Remove(testConfigPath)
	}()
	for _, device := range manager.devices {
		device.devicePath = "/dev/sda"
		mockexec.On("RunCmd", fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device.devicePath)).
			Return("", "", nil)
		mockexec.On("RunCmd", fmt.Sprintf(deleteFileCmdTmpl, device.fileName)).
			Return("", "", nil)
	}
	manager.readAndSetConfig(testConfigPath)
	manager.updateDevicesFromConfig()

	for _, device := range manager.devices {
		assert.Equal(t, device.Size, "30Mi")
	}
}

func TestLoopBackManager_overrideDevicesFromSetConfigWithSize(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
	testSN := "testSN"
	testNodeID := "testNode"
	testConfigPath := "/tmp/config.yaml"
	config := []byte("defaultDriveSize: 30Mi \ndefaultDrivePerNodeCount: 3\nnodes:\n" +
		fmt.Sprintf("- nodeID: %s\n", testNodeID) +
		fmt.Sprintf("  driveCount: %d\n", 5) +
		"  drives:\n" +
		fmt.Sprintf("  - serialNumber: %s\n", testSN) +
		fmt.Sprintf("    size: %s\n", "40Mi"))
	err := ioutil.WriteFile(testConfigPath, config, 0o777)
	assert.Nil(t, err)

	defer func() {
		_ = os.Remove(testConfigPath)
	}()
	manager.nodeName = testNodeID
	for _, device := range manager.devices {
		mockexec.On("RunCmd", fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device.devicePath)).
			Return("", "", nil)
		mockexec.On("RunCmd", fmt.Sprintf(deleteFileCmdTmpl, device.fileName)).
			Return("", "", nil)
	}

	manager.readAndSetConfig(testConfigPath)
	manager.updateDevicesFromConfig()

	config = []byte("defaultDriveSize: 30Mi \ndefaultDrivePerNodeCount: 3\nnodes:\n" +
		fmt.Sprintf("- nodeID: %s\n", testNodeID) +
		fmt.Sprintf("  driveCount: %d\n", 5) +
		"  drives:\n" +
		fmt.Sprintf("  - serialNumber: %s\n", testSN))
	err = ioutil.WriteFile(testConfigPath, config, 0o777)
	assert.Nil(t, err)

	for _, device := range manager.devices {
		if device.SerialNumber == testSN {
			device.devicePath = "/dev/sda"
			mockexec.On("RunCmd", fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device.devicePath)).
				Return("", "", nil)
			mockexec.On("RunCmd", fmt.Sprintf(deleteFileCmdTmpl, device.fileName)).
				Return("", "", nil)
		}
	}
	manager.readAndSetConfig(testConfigPath)
	manager.updateDevicesFromConfig()

	assert.Nil(t, err)

	// resizing is not supported
	for _, device := range manager.devices {
		if device.SerialNumber == testSN {
			assert.Equal(t, device.Size, "40Mi")
		}
	}
}

func TestLoopBackManager_overrideDevicesFromNodeConfig(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)

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
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
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
	// resizing is not supported
	assert.Equal(t, defaultSize, manager.devices[indexOfDeviceToOverride].Size)
}

func TestLoopBackManager_GetDrivesList(t *testing.T) {
	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
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

func TestLoopBackManager_attemptToRecoverDevicesFromConfig(t *testing.T) {
	testImagesPath := "/tmp/images"
	err := os.Mkdir(testImagesPath, 0o777)
	assert.Nil(t, err)
	defer func() {
		// cleanup fake images
		_ = os.RemoveAll(testImagesPath)
	}()

	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
	// Clean devices after default initialization in constructor
	manager.devices = make([]*LoopBackDevice, 0)

	// image file that should be ignored during recovery
	ignoredImage := fmt.Sprintf("%s/%s", testImagesPath, "random.img")
	_, err = os.Create(ignoredImage)
	mockexec.On("RunCmd", fmt.Sprintf(deleteFileCmdTmpl, ignoredImage)).Return("", "", nil)
	assert.Nil(t, err)

	// image of device that should be recovered from default config
	testSerialNumber1 := "12345"
	_, err = os.Create(fmt.Sprintf("%s/%s-%s.img", testImagesPath, manager.nodeID, testSerialNumber1))
	assert.Nil(t, err)

	// image of device that should be recovered from node config
	testSerialNumber2 := "56789"
	nonDefaultVID := "non-default-VID"
	_, err = os.Create(fmt.Sprintf("%s/%s-%s.img", testImagesPath, manager.nodeID, testSerialNumber2))
	assert.Nil(t, err)

	// set manager's node config
	manager.config = &Config{
		DefaultDriveCount: 3,
		Nodes: []*Node{
			{
				Drives: []*LoopBackDevice{
					{
						SerialNumber: fmt.Sprintf("LOOPBACK%s", testSerialNumber2),
						VendorID:     nonDefaultVID,
					},
				},
			},
		},
	}

	manager.attemptToRecoverDevices(testImagesPath)
	assert.Equal(t, len(manager.devices), 2)

	var recoveredDeviceVID string
	for _, device := range manager.devices {
		if strings.Contains(device.SerialNumber, testSerialNumber2) {
			recoveredDeviceVID = device.VendorID
			break
		}
	}
	assert.Equal(t, recoveredDeviceVID, nonDefaultVID)
}

func TestLoopBackManager_attemptToRecoverDevicesFromDefaults(t *testing.T) {
	testImagesPath := "/tmp/images"
	err := os.Mkdir(testImagesPath, 0o777)
	assert.Nil(t, err)
	defer func() {
		// cleanup fake images
		_ = os.RemoveAll(testImagesPath)
	}()

	mockexec := &mocks.GoMockExecutor{}
	manager := NewLoopBackManager(mockexec, "", "", logger)
	// Clean devices after default initialization in constructor
	manager.devices = make([]*LoopBackDevice, 0)

	// image of device that should be recovered from default config
	testSerialNumber := "12345"
	_, err = os.Create(fmt.Sprintf("%s/%s-%s.img", testImagesPath, manager.nodeID, testSerialNumber))
	assert.Nil(t, err)

	manager.attemptToRecoverDevices(testImagesPath)
	assert.Equal(t, len(manager.devices), 1)
}
