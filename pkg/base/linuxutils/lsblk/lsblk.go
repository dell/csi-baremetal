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

// Package lsblk contains code for running and interpreting output of system util lsblk
package lsblk

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/command"
)

const (
	// CmdTmpl adds device name, if add empty string - command will print info about all devices
	CmdTmpl = "lsblk %s --paths --json --bytes --fs " +
		"--output NAME,TYPE,SIZE,ROTA,SERIAL,WWN,VENDOR,MODEL,REV,MOUNTPOINT,FSTYPE,PARTUUID"
	// outputKey is the key to find block devices in lsblk json output
	outputKey = "blockdevices"
	// romDeviceType is the constant that represents rom devices to exclude them from lsblk output
	romDeviceType = "rom"
)

// WrapLsblk is an interface that encapsulates operation with system lsblk util
type WrapLsblk interface {
	GetBlockDevices(device string) ([]BlockDevice, error)
	SearchDrivePath(drive *drivecrd.Drive) (string, error)
}

// LSBLK is a wrap for system lsblk util
type LSBLK struct {
	e command.CmdExecutor
}

// NewLSBLK is a constructor for LSBLK struct
func NewLSBLK(log *logrus.Logger) *LSBLK {
	e := &command.Executor{}
	e.SetLogger(log)
	e.SetLevel(logrus.TraceLevel)
	return &LSBLK{e: e}
}

// BlockDevice is the struct that represents output of lsblk command for a device
type BlockDevice struct {
	Name       string        `json:"name,omitempty"`
	Type       string        `json:"type,omitempty"`
	Size       string        `json:"size,omitempty"`
	Rota       string        `json:"rota,omitempty"`
	Serial     string        `json:"serial,omitempty"`
	WWN        string        `json:"wwn,omitempty"`
	Vendor     string        `json:"vendor,omitempty"`
	Model      string        `json:"model,omitempty"`
	Rev        string        `json:"rev,omitempty"`
	MountPoint string        `json:"mountpoint,omitempty"`
	FSType     string        `json:"fstype,omitempty"`
	PartUUID   string        `json:"partuuid,omitempty"`
	Children   []BlockDevice `json:"children,omitempty"`
}

// GetBlockDevices run os lsblk command for device and construct BlockDevice struct based on output
// Receives device path. If device is empty string, info about all devices will be collected
// Returns slice of BlockDevice structs or error if something went wrong
func (l *LSBLK) GetBlockDevices(device string) ([]BlockDevice, error) {
	cmd := fmt.Sprintf(CmdTmpl, device)
	strOut, _, err := l.e.RunCmd(cmd)
	if err != nil {
		return nil, err
	}
	rawOut := make(map[string][]BlockDevice, 1)
	err = json.Unmarshal([]byte(strOut), &rawOut)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal output to BlockDevice instance, error: %v", err)
	}
	res := make([]BlockDevice, 0)
	var (
		devs []BlockDevice
		ok   bool
	)
	if devs, ok = rawOut[outputKey]; !ok {
		return nil, fmt.Errorf("unexpected lsblk output format, missing \"%s\" key", outputKey)
	}
	for _, d := range devs {
		if d.Type != romDeviceType {
			res = append(res, d)
		}
	}
	return res, nil
}

// SearchDrivePath if not defined returns drive path based on drive S/N.
// Receives an instance of drivecrd.Drive struct
// Returns drive's path based on provided drivecrd.Drive or error if something went wrong
// TODO: check VID/PID as well - https://github.com/dell/csi-baremetal/issues/82
func (l *LSBLK) SearchDrivePath(drive *drivecrd.Drive) (string, error) {
	// device path might be already set by hwmgr
	device := drive.Spec.Path
	if device != "" {
		return device, nil
	}

	// try to find it with lsblk
	lsblkOut, err := l.GetBlockDevices("")
	if err != nil {
		return "", err
	}

	// get drive serial number
	sn := drive.Spec.SerialNumber
	for _, l := range lsblkOut {
		if strings.EqualFold(l.Serial, sn) {
			device = l.Name
			break
		}
	}

	if device == "" {
		return "", fmt.Errorf("unable to find drive path by S/N %s", sn)
	}

	return device, nil
}
