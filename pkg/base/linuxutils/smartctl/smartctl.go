package smartctl

import (
	"encoding/json"
	"fmt"

	"github.com/dell/csi-baremetal.git/pkg/base/command"
)

const (
	//SmartctlCmdImpl is a base CMD for smartctl util
	SmartctlCmdImpl = "smartctl"
	//SmartctlDeviceInfoCmdImpl is a CMD to get basic SMART information and health about device in JSON format
	SmartctlDeviceInfoCmdImpl = SmartctlCmdImpl + " --info --health --json %s"
)

//WrapSmartctl is an interface that encapsulates operation with system smartctl util
type WrapSmartctl interface {
	GetDriveInfoByPath(path string) (*DeviceSMARTInfo, error)
}

//DeviceSMARTInfo represents SMART information about device
type DeviceSMARTInfo struct {
	SerialNumber string          `json:"serial_number"`
	SmartStatus  map[string]bool `json:"smart_status"`
	Rotation     int             `json:"rotation_rate"`
}

//SMARTCTL is a wrap for system smartctl util
type SMARTCTL struct {
	e command.CmdExecutor
}

//NewSMARTCTL is a constructor for SMARTCTL
func NewSMARTCTL(e command.CmdExecutor) *SMARTCTL {
	return &SMARTCTL{e: e}
}

//GetDriveInfoByPath gets SMART information about device by its Path using smartctl util
func (sa *SMARTCTL) GetDriveInfoByPath(path string) (*DeviceSMARTInfo, error) {
	strOut, _, err := sa.e.RunCmd(fmt.Sprintf(SmartctlDeviceInfoCmdImpl, path))
	if err != nil {
		return nil, err
	}
	var deviceInfo = &DeviceSMARTInfo{}
	bytes := []byte(strOut)
	err = json.Unmarshal(bytes, deviceInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal output to []Device instance, error: %v", err)
	}
	return deviceInfo, nil
}
