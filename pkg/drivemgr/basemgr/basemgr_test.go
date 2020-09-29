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

package basemgr

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsscsi"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/nvmecli"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/smartctl"
	"github.com/dell/csi-baremetal/pkg/mocks"
	"github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
)

var logger = logrus.New()

func TestLoopBackManager_GetNVMDevicesSuccess(t *testing.T) {
	var (
		mockexec = &mocks.GoMockExecutor{}
		manager  = NewBaseManager(mockexec, logger)
		mockNvme = &linuxutils.MockWrapNvmecli{}
	)
	nvmeDevice := make([]nvmecli.NVMDevice, 0)
	nvmeDevice = append(nvmeDevice, nvmecli.NVMDevice{
		DevicePath:   "testPath",
		Firmware:     "testFirmware",
		ModelNumber:  "testModel",
		SerialNumber: "testSN",
		Vendor:       2311,
		PhysicalSize: 1000,
		Health:       apiV1.HealthGood,
	})
	mockNvme.On("GetNVMDevices", mock.Anything).
		Return(nvmeDevice, nil).Once()

	manager.nvme = mockNvme
	devices, err := manager.GetNVMDevices()

	assert.Nil(t, err)
	assert.Equal(t, 1, len(devices))
	assert.Equal(t, "testPath", devices[0].Path)
	assert.Equal(t, "testFirmware", devices[0].Firmware)
	assert.Equal(t, "testModel", devices[0].PID)
	assert.Equal(t, "testSN", devices[0].SerialNumber)
	assert.Equal(t, int64(1000), devices[0].Size)
	assert.Equal(t, apiV1.HealthGood, devices[0].Health)
	assert.Equal(t, apiV1.DriveTypeNVMe, devices[0].Type)
	assert.Equal(t, "2311", devices[0].VID)
}

func TestLoopBackManager_GetNVMDevicesEmptyVidPidSn(t *testing.T) {
	var (
		mockexec = &mocks.GoMockExecutor{}
		manager  = NewBaseManager(mockexec, logger)
		mockNvme = &linuxutils.MockWrapNvmecli{}
	)
	nvmeDevice := make([]nvmecli.NVMDevice, 0)
	nvmeDevice = append(nvmeDevice, nvmecli.NVMDevice{
		DevicePath:   "testPath",
		Firmware:     "testFirmware",
		ModelNumber:  "",
		SerialNumber: "",
		Vendor:       0,
		PhysicalSize: 1000,
		Health:       apiV1.HealthGood,
	})
	mockNvme.On("GetNVMDevices", mock.Anything).
		Return(nvmeDevice, nil).Once()

	manager.nvme = mockNvme
	devices, err := manager.GetNVMDevices()

	assert.Nil(t, err)
	assert.Equal(t, 0, len(devices))
}

func TestLoopBackManager_GetNVMDevicesFail(t *testing.T) {
	var (
		mockexec = &mocks.GoMockExecutor{}
		manager  = NewBaseManager(mockexec, logger)
		mockNvme = &linuxutils.MockWrapNvmecli{}
	)
	mockNvme.On("GetNVMDevices", mock.Anything).
		Return([]nvmecli.NVMDevice{}, fmt.Errorf("error")).Once()

	manager.nvme = mockNvme
	_, err := manager.GetNVMDevices()

	assert.NotNil(t, err)
}

func TestLoopBackManager_GetSCSIDevices(t *testing.T) {
	var (
		mockexec     = &mocks.GoMockExecutor{}
		manager      = NewBaseManager(mockexec, logger)
		mockLsscsi   = &linuxutils.MockWrapLsscsi{}
		mockSmartctl = &linuxutils.MockWrapSmartctl{}
	)

	smart := &smartctl.DeviceSMARTInfo{
		SerialNumber: "testSN",
		SmartStatus:  make(map[string]bool),
		Rotation:     0,
	}
	smart.SmartStatus["passed"] = true
	scsiDevice := make([]*lsscsi.SCSIDevice, 0)
	scsiDevice = append(scsiDevice, &lsscsi.SCSIDevice{
		ID:       "[0:0:0:1]",
		Path:     "testPath",
		Size:     1000,
		Vendor:   "testVendor",
		Model:    "testModel",
		Firmware: "testFirmware",
	})
	mockLsscsi.On("GetSCSIDevices", mock.Anything).
		Return(scsiDevice, nil)

	mockSmartctl.On("GetDriveInfoByPath", "testPath").
		Return(smart, nil)

	manager.lsscsi = mockLsscsi
	manager.smartctl = mockSmartctl

	devices, err := manager.GetSCSIDevices()

	assert.Nil(t, err)
	assert.Equal(t, 1, len(devices))
	assert.Equal(t, "testVendor", devices[0].VID)
	assert.Equal(t, "testPath", devices[0].Path)
	assert.Equal(t, "testFirmware", devices[0].Firmware)
	assert.Equal(t, "testModel", devices[0].PID)
	assert.Equal(t, "testSN", devices[0].SerialNumber)
	assert.Equal(t, int64(1000), devices[0].Size)
	assert.Equal(t, apiV1.HealthGood, devices[0].Health)
	assert.Equal(t, apiV1.DriveTypeSSD, devices[0].Type)

	smart.SmartStatus["passed"] = false
	smart.Rotation = 7200
	devices, err = manager.GetSCSIDevices()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(devices))
	assert.Equal(t, apiV1.HealthBad, devices[0].Health)
	assert.Equal(t, apiV1.DriveTypeHDD, devices[0].Type)
}

func TestLoopBackManager_GetSCSIDevicesEmptyVidPidSn(t *testing.T) {
	var (
		mockexec     = &mocks.GoMockExecutor{}
		manager      = NewBaseManager(mockexec, logger)
		mockLsscsi   = &linuxutils.MockWrapLsscsi{}
		mockSmartctl = &linuxutils.MockWrapSmartctl{}
	)

	smart := &smartctl.DeviceSMARTInfo{
		SerialNumber: "",
		SmartStatus:  make(map[string]bool),
		Rotation:     0,
	}
	smart.SmartStatus["passed"] = true
	scsiDevice := make([]*lsscsi.SCSIDevice, 0)
	scsiDevice = append(scsiDevice, &lsscsi.SCSIDevice{
		ID:       "[0:0:0:1]",
		Path:     "testPath",
		Size:     1000,
		Vendor:   "",
		Model:    "",
		Firmware: "testFirmware",
	})
	mockLsscsi.On("GetSCSIDevices", mock.Anything).
		Return(scsiDevice, nil)

	mockSmartctl.On("GetDriveInfoByPath", "testPath").
		Return(smart, nil)

	manager.lsscsi = mockLsscsi
	manager.smartctl = mockSmartctl

	devices, err := manager.GetSCSIDevices()

	assert.Nil(t, err)
	assert.Equal(t, 0, len(devices))
}

func TestLoopBackManager_GetSCSIDevicesFail(t *testing.T) {
	var (
		mockexec     = &mocks.GoMockExecutor{}
		manager      = NewBaseManager(mockexec, logger)
		mockSmartctl = &linuxutils.MockWrapSmartctl{}
		mockLsscsi   = &linuxutils.MockWrapLsscsi{}
	)
	scsiDevice := []*lsscsi.SCSIDevice{{
		ID:       "[0:0:0:1]",
		Path:     "testPath",
		Size:     1000,
		Vendor:   "testVendor",
		Model:    "testModel",
		Firmware: "testFirmware",
	}}
	mockLsscsi.On("GetSCSIDevices", mock.Anything).
		Return(scsiDevice, nil)

	mockSmartctl.On("GetDriveInfoByPath", "testPath").
		Return(&smartctl.DeviceSMARTInfo{}, fmt.Errorf("error"))

	manager.smartctl = mockSmartctl
	manager.lsscsi = mockLsscsi

	devs, err := manager.GetSCSIDevices()

	assert.Nil(t, err)
	assert.Equal(t, len(devs), 0)
}

func TestLoopBackManager_GetSCSIDevicesLsscsiFail(t *testing.T) {
	var (
		mockexec   = &mocks.GoMockExecutor{}
		manager    = NewBaseManager(mockexec, logger)
		mockLsscsi = &linuxutils.MockWrapLsscsi{}
	)
	mockLsscsi.On("GetSCSIDevices", mock.Anything).
		Return([]*lsscsi.SCSIDevice{}, fmt.Errorf("error"))
	manager.lsscsi = mockLsscsi

	_, err := manager.GetSCSIDevices()

	assert.NotNil(t, err)
}

func TestLoopBackManager_GetDrivesListFail(t *testing.T) {
	var (
		mockexec   = &mocks.GoMockExecutor{}
		manager    = NewBaseManager(mockexec, logger)
		mockLsscsi = &linuxutils.MockWrapLsscsi{}
		mockNvme   = &linuxutils.MockWrapNvmecli{}
	)
	mockNvme.On("GetNVMDevices", mock.Anything).
		Return([]nvmecli.NVMDevice{}, fmt.Errorf("error"))
	mockLsscsi.On("GetSCSIDevices", mock.Anything).
		Return([]*lsscsi.SCSIDevice{}, nil)

	manager.lsscsi = mockLsscsi
	manager.nvme = mockNvme

	_, err := manager.GetDrivesList()

	assert.Nil(t, err)
}

func TestLoopBackManager_GetDrivesListLsscsiFail(t *testing.T) {
	var (
		mockexec   = &mocks.GoMockExecutor{}
		manager    = NewBaseManager(mockexec, logger)
		mockLsscsi = &linuxutils.MockWrapLsscsi{}
		mockNvme   = &linuxutils.MockWrapNvmecli{}
	)

	mockLsscsi.On("GetSCSIDevices", mock.Anything).
		Return([]*lsscsi.SCSIDevice{}, fmt.Errorf("error"))
	manager.lsscsi = mockLsscsi

	mockNvme.On("GetNVMDevices", mock.Anything).
		Return([]nvmecli.NVMDevice{}, nil)
	manager.nvme = mockNvme

	_, err := manager.GetDrivesList()

	assert.Nil(t, err)
}

func TestLoopBackManager_GetDrivesListSuccess(t *testing.T) {
	var (
		mockexec   = &mocks.GoMockExecutor{}
		manager    = NewBaseManager(mockexec, logger)
		mockLsscsi = &linuxutils.MockWrapLsscsi{}
		mockNvme   = &linuxutils.MockWrapNvmecli{}
	)
	mockNvme.On("GetNVMDevices", mock.Anything).
		Return([]nvmecli.NVMDevice{}, nil)

	mockLsscsi.On("GetSCSIDevices", mock.Anything).
		Return([]*lsscsi.SCSIDevice{}, nil)
	manager.lsscsi = mockLsscsi
	manager.nvme = mockNvme

	_, err := manager.GetDrivesList()

	assert.Nil(t, err)
}
