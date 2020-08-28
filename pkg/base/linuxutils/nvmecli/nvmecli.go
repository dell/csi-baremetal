package nvmecli

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/command"
)

const (
	//NVMCliCmdImpl is a base CMD for nvme_cli
	NVMCliCmdImpl = "nvme"
	//NVMeDeviceCmdImpl is a CMD for listing all nvme devices in JSON format
	NVMeDeviceCmdImpl = NVMCliCmdImpl + " list --output-format=json"
	//NVMeHealthCmdImpl is a CMD to get SMART information about NVMe device in JSON format
	NVMeHealthCmdImpl = NVMCliCmdImpl + " smart-log %s --output-format=json"
	//NVMeVendorCmdImpl is a CMD to get SMART information about NVMe device in JSON format
	NVMeVendorCmdImpl = NVMCliCmdImpl + " id-ctrl %s --output-format=json"
	//DevicesKey is the key to find NVMe devices in nvme json output
	DevicesKey = "Devices"
)

//WrapNvmecli is an interface that encapsulates operation with system nvme util
type WrapNvmecli interface {
	GetNVMDevices() ([]NVMDevice, error)
}

//NVMDevice represents devices from nvme list output
type NVMDevice struct {
	DevicePath   string `json:"DevicePath,omitempty"`
	Firmware     string `json:"Firmware,omitempty"`
	ModelNumber  string `json:"ModelNumber,omitempty"`
	SerialNumber string `json:"SerialNumber,omitempty"`
	PhysicalSize int64  `json:"PhysicalSize,omitempty"`
	//Can VID be string for nvme?
	Vendor int `json:"vid,omitempty"`
	Health string
}

//SMARTLog represents SMART information for NVMe devices
type SMARTLog struct {
	CriticalWarning int `json:"critical_warning,omitempty"`
}

//NVMECLI is a wrap for system nvem_cli util
type NVMECLI struct {
	e   command.CmdExecutor
	log *logrus.Entry
}

//NewNVMECLI is a constructor for NVMECLI
func NewNVMECLI(e command.CmdExecutor, logger *logrus.Logger) *NVMECLI {
	return &NVMECLI{e: e, log: logger.WithField("component", "NVMECLI")}
}

//GetNVMDevices gets information about NVMDevice using nvme_cli util
func (na *NVMECLI) GetNVMDevices() ([]NVMDevice, error) {
	ll := na.log.WithField("method", "GetNVMDevices")
	strOut, _, err := na.e.RunCmd(NVMeDeviceCmdImpl)
	if err != nil {
		return nil, err
	}
	rawOut := make(map[string][]NVMDevice)
	err = json.Unmarshal([]byte(strOut), &rawOut)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal output to NVMDevice instance, error: %v", err)
	}
	var (
		devs []NVMDevice
		ok   bool
	)
	if devs, ok = rawOut[DevicesKey]; !ok {
		ll.Errorf("key \"%s\" is not in map %v", DevicesKey, rawOut)
		return nil, fmt.Errorf("unexpected nvme list output format")
	}
	for i, d := range devs {
		devs[i].Health = na.getNVMDeviceHealth(d.DevicePath)
		na.fillNVMDeviceVendor(&devs[i])
	}
	return devs, nil
}

//getNVMDeviceHealth gets information about device health based on critical_warning SMART attribute using nvme_cli smart-log util
func (na *NVMECLI) getNVMDeviceHealth(path string) string {
	ll := na.log.WithField("method", "getNVMDeviceHealth")
	cmd := fmt.Sprintf(NVMeHealthCmdImpl, path)
	strOut, _, err := na.e.RunCmd(cmd)
	if err != nil {
		ll.Errorf("%s failed, set health as %s", cmd, apiV1.HealthUnknown)
		return apiV1.HealthUnknown
	}
	smartLog := &SMARTLog{}
	err = json.Unmarshal([]byte(strOut), &smartLog)
	if err != nil {
		ll.Errorf("unable to unmarshal output to SMARTLog, set health as %s", apiV1.HealthUnknown)
		return apiV1.HealthUnknown
	}
	health := smartLog.CriticalWarning
	if na.isBitSet(health, 0) || na.isBitSet(health, 3) {
		return apiV1.HealthSuspect
	}
	if na.isBitSet(health, 2) || na.isBitSet(health, 4) || na.isBitSet(health, 5) {
		return apiV1.HealthBad
	}
	return apiV1.HealthGood
}

//fillNVMDeviceVendor gets information about device vendor id
func (na *NVMECLI) fillNVMDeviceVendor(device *NVMDevice) {
	ll := na.log.WithField("method", "fillNVMDeviceVendor")
	cmd := fmt.Sprintf(NVMeVendorCmdImpl, device.DevicePath)
	strOut, _, err := na.e.RunCmd(cmd)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(strOut), &device)
	if err != nil {
		ll.Errorf("unable to unmarshal output to NVMEDevice, error: %v", err)
	}
}

//isBitSet check if bit on bitPos in value is set
func (na *NVMECLI) isBitSet(value, bitPos int) bool {
	return (value>>bitPos)&1 != 0
}
