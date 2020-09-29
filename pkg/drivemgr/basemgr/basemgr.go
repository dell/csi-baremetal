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
	"strconv"

	"github.com/sirupsen/logrus"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsscsi"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/nvmecli"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/smartctl"
)

//BaseManager is a drive manager based on Linux system utils
type BaseManager struct {
	exec     command.CmdExecutor
	log      *logrus.Entry
	lsscsi   lsscsi.WrapLsscsi
	smartctl smartctl.WrapSmartctl
	nvme     nvmecli.WrapNvmecli
}

//GetDrivesList gets api.Drive slice using Linux system utils
func (mgr BaseManager) GetDrivesList() ([]*api.Drive, error) {
	ll := mgr.log.WithField("method", "GetDrivesList")
	var (
		devices    []*api.Drive
		nvmDevices []*api.Drive
		err        error
	)
	if devices, err = mgr.GetSCSIDevices(); err != nil {
		ll.Errorf("Failed to initialize devices, Error: %v", err)
	}
	if nvmDevices, err = mgr.GetNVMDevices(); err != nil {
		ll.Errorf("Failed to initialize devices, Error: %v", err)
	}
	devices = append(devices, nvmDevices...)
	return devices, nil
}

//NewBaseManager is a constructor BaseManager
func NewBaseManager(exec command.CmdExecutor, logger *logrus.Logger) *BaseManager {
	return &BaseManager{
		exec:     exec,
		log:      logger.WithField("component", "BaseManager"),
		lsscsi:   lsscsi.NewLSSCSI(exec, logger),
		smartctl: smartctl.NewSMARTCTL(exec),
		nvme:     nvmecli.NewNVMECLI(exec, logger),
	}
}

//GetSCSIDevices get []*api.Drive using lsscsi system util
func (mgr *BaseManager) GetSCSIDevices() ([]*api.Drive, error) {
	ll := mgr.log.WithField("method", "GetSCSIDevices")
	allDevices := make([]*api.Drive, 0)
	scsiDevices, err := mgr.lsscsi.GetSCSIDevices()
	if err != nil {
		ll.Errorf("Failed to get SCSI allDevices, Error: %v", err)
		return nil, err
	}
	for _, device := range scsiDevices {
		allDevices = append(allDevices, &api.Drive{
			Path:     device.Path,
			Firmware: device.Firmware,
			VID:      device.Vendor,
			PID:      device.Model,
			Size:     device.Size,
		})
	}
	devices := make([]*api.Drive, 0)
	for i, device := range allDevices {
		smartInfo, err := mgr.smartctl.GetDriveInfoByPath(device.Path)
		if err != nil {
			//We don't fail whole drivemgr because of error with just one device, we don't add it in allDevices slice
			ll.Errorf("Failed to get SMART information for Device %v, Error: %v", allDevices[i], err)
		} else {
			allDevices[i].SerialNumber = smartInfo.SerialNumber
			if allDevices[i].SerialNumber != "" && allDevices[i].VID != "" && allDevices[i].PID != "" {
				if smartInfo.Rotation > 0 {
					allDevices[i].Type = apiV1.DriveTypeHDD
				} else {
					allDevices[i].Type = apiV1.DriveTypeSSD
				}
				if smartInfo.SmartStatus["passed"] {
					allDevices[i].Health = apiV1.HealthGood
				} else {
					allDevices[i].Health = apiV1.HealthBad
				}
				devices = append(devices, allDevices[i])
			} else {
				ll.Errorf("Device has empty VID, PID or SN field: %v", allDevices[i])
			}
		}
	}
	return devices, nil
}

//GetNVMDevices get []*api.Drive using nvme_cli system util
func (mgr *BaseManager) GetNVMDevices() ([]*api.Drive, error) {
	ll := mgr.log.WithField("method", "GetNVMDevices")
	devices := make([]*api.Drive, 0)
	nvmeDevices, err := mgr.nvme.GetNVMDevices()
	if err != nil {
		ll.Errorf("Failed to get NVMe devices, Error: %v", err)
		return nil, err
	}
	for _, device := range nvmeDevices {
		if device.Vendor != 0 && device.ModelNumber != "" && device.SerialNumber != "" {
			devices = append(devices, &api.Drive{
				Health:       device.Health,
				PID:          device.ModelNumber,
				VID:          strconv.Itoa(device.Vendor),
				SerialNumber: device.SerialNumber,
				Type:         apiV1.DriveTypeNVMe,
				Size:         device.PhysicalSize,
				Firmware:     device.Firmware,
				Path:         device.DevicePath,
			})
		} else {
			ll.Errorf("Device has empty VID, PID or SN field: %v", device)
		}
	}
	return devices, nil
}
