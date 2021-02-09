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

package smartctl

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dell/csi-baremetal/pkg/base/command"
)

const (
	// SmartctlCmdImpl is a base CMD for smartctl util
	SmartctlCmdImpl = "smartctl"
	// SmartctlDeviceInfoCmdImpl is a CMD to get basic SMART information and health about device in JSON format
	SmartctlDeviceInfoCmdImpl = SmartctlCmdImpl + " --info --json %s"
	// SmartctlHealthCmdImpl is a CMD to get  SMART status of device in JSON format
	SmartctlHealthCmdImpl = SmartctlCmdImpl + " --health --json %s"
)

// WrapSmartctl is an interface that encapsulates operation with system smartctl util
type WrapSmartctl interface {
	GetDriveInfoByPath(path string) (*DeviceSMARTInfo, error)
}

// DeviceSMARTInfo represents SMART information about device
type DeviceSMARTInfo struct {
	SerialNumber string          `json:"serial_number"`
	SmartStatus  map[string]bool `json:"smart_status"`
	Rotation     int             `json:"rotation_rate"`
}

// SMARTCTL is a wrap for system smartctl util
type SMARTCTL struct {
	e command.CmdExecutor
}

// NewSMARTCTL is a constructor for SMARTCTL
func NewSMARTCTL(e command.CmdExecutor) *SMARTCTL {
	return &SMARTCTL{e: e}
}

// GetDriveInfoByPath gets SMART information about device by its Path using smartctl util
func (sa *SMARTCTL) GetDriveInfoByPath(path string) (*DeviceSMARTInfo, error) {
	strOut, _, err := sa.e.RunCmd(fmt.Sprintf(SmartctlDeviceInfoCmdImpl, path),
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(SmartctlDeviceInfoCmdImpl, ""))))
	if err != nil {
		return nil, err
	}
	var deviceInfo = &DeviceSMARTInfo{}
	bytes := []byte(strOut)
	err = json.Unmarshal(bytes, deviceInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal output to []DeviceSMARTInfo instance, error: %v", err)
	}
	err = sa.fillSmartStatus(deviceInfo, path)
	if err != nil {
		return nil, fmt.Errorf("unable to get SMART status for device %s, error: %v", path, err)
	}
	return deviceInfo, nil
}

// fillSmartStatus fill smart_status field in DeviceSMARTInfo using smartctl command
func (sa *SMARTCTL) fillSmartStatus(dev *DeviceSMARTInfo, path string) error {
	strOut, _, err := sa.e.RunCmd(fmt.Sprintf(SmartctlHealthCmdImpl, path),
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(SmartctlHealthCmdImpl, ""))))
	if err != nil {
		return err
	}
	bytes := []byte(strOut)
	err = json.Unmarshal(bytes, dev)
	if err != nil {
		return fmt.Errorf("unable to unmarshal output to []Device instance, error: %v", err)
	}
	return nil
}
