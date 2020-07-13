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

//LinuxUtilManager is a drive manager based on Linux system utils
type LinuxUtilManager struct {
	exec     command.CmdExecutor
	log      *logrus.Entry
	lsscsi   lsscsi.WrapLsscsi
	smartctl smartctl.WrapSmartctl
	nvme     nvmecli.WrapNvmecli
}

//GetDrivesList gets api.Drive slice using Linux system utils
func (mgr LinuxUtilManager) GetDrivesList() ([]*api.Drive, error) {
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

//NewLinuxUtilManager is a constructor LinuxUtilManager
func NewLinuxUtilManager(exec command.CmdExecutor, logger *logrus.Logger) *LinuxUtilManager {
	return &LinuxUtilManager{
		exec:     exec,
		log:      logger.WithField("component", "LinuxUtilManager"),
		lsscsi:   lsscsi.NewLSSCSI(exec, logger),
		smartctl: smartctl.NewSMARTCTL(exec),
		nvme:     nvmecli.NewNVMECLI(exec, logger),
	}
}

//GetSCSIDevices get []*api.Drive using lsscsi system util
func (mgr *LinuxUtilManager) GetSCSIDevices() ([]*api.Drive, error) {
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
func (mgr *LinuxUtilManager) GetNVMDevices() ([]*api.Drive, error) {
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
