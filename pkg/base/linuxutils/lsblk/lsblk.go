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
	e := command.NewExecutor(log)
	e.SetLevel(logrus.TraceLevel)
	return &LSBLK{e: e}
}

// BlockDevice is the struct that represents output of lsblk (from util-linux 2.31.1) command for a device
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

// BlockDeviceV2 is the struct that represents output of lsblk (from util-linux 2.34) command for a device
type BlockDeviceV2 struct {
	Name       string          `json:"name,omitempty"`
	Type       string          `json:"type,omitempty"`
	Size       int64           `json:"size,omitempty"`
	Rota       bool            `json:"rota,omitempty"`
	Serial     string          `json:"serial,omitempty"`
	WWN        string          `json:"wwn,omitempty"`
	Vendor     string          `json:"vendor,omitempty"`
	Model      string          `json:"model,omitempty"`
	Rev        string          `json:"rev,omitempty"`
	MountPoint string          `json:"mountpoint,omitempty"`
	FSType     string          `json:"fstype,omitempty"`
	PartUUID   string          `json:"partuuid,omitempty"`
	Children   []BlockDeviceV2 `json:"children,omitempty"`
}

func convertToV1(blockV2 BlockDeviceV2) BlockDevice {
	var blockV1 = BlockDevice{}
	if blockV2.Children != nil {
		blockV1.Children = make([]BlockDevice, 0)
		for _, child := range blockV2.Children {
			blockV1.Children = append(blockV1.Children, convertToV1(child))
		}
	}

	blockV1.Name = blockV2.Name
	blockV1.Type = blockV2.Type
	blockV1.Size = fmt.Sprint(blockV2.Size)
	// convert from boolean to string
	rota := "0"
	if blockV2.Rota {
		rota = "1"
	}
	blockV1.Rota = rota
	blockV1.Serial = blockV2.Serial
	blockV1.WWN = blockV2.WWN
	blockV1.Vendor = blockV2.Vendor
	blockV1.Model = blockV2.Model
	blockV1.Rev = blockV2.Rev
	blockV1.MountPoint = blockV2.MountPoint
	blockV1.FSType = blockV2.FSType
	blockV1.PartUUID = blockV2.PartUUID

	return blockV1
}

// GetBlockDevices run os lsblk command for device and construct BlockDevice struct based on output
// Receives device path. If device is empty string, info about all devices will be collected
// Returns slice of BlockDevice structs or error if something went wrong
func (l *LSBLK) GetBlockDevices(device string) ([]BlockDevice, error) {
	cmd := fmt.Sprintf(CmdTmpl, device)
	strOut, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(CmdTmpl, ""))))
	if err != nil {
		return nil, err
	}

	isV2 := false
	var rawOut2 map[string][]BlockDeviceV2
	rawOut := make(map[string][]BlockDevice, 1)
	err = json.Unmarshal([]byte(strOut), &rawOut)
	if err != nil {
		// try version 2
		rawOut2 = make(map[string][]BlockDeviceV2, 1)
		err = json.Unmarshal([]byte(strOut), &rawOut2)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal output to BlockDevice instance, error: %v", err)
		}
		isV2 = true
	}
	res := make([]BlockDevice, 0)
	var (
		devs []BlockDevice
		ok   bool
	)

	if isV2 {
		var devsV2 []BlockDeviceV2
		if devsV2, ok = rawOut2[outputKey]; !ok {
			return nil, fmt.Errorf("unexpected lsblk output format, missing \"%s\" key", outputKey)
		}
		for _, d := range devsV2 {
			if d.Type != romDeviceType {
				res = append(res, convertToV1(d))
			}
		}
	} else {
		if devs, ok = rawOut[outputKey]; !ok {
			return nil, fmt.Errorf("unexpected lsblk output format, missing \"%s\" key", outputKey)
		}
		for _, d := range devs {
			if d.Type != romDeviceType {
				res = append(res, d)
			}
		}
	}

	return res, nil
}

// SearchDrivePath if not defined returns drive path based on drive S/N, VID and PID.
// Receives an instance of drivecrd.Drive struct
// Returns drive's path based on provided drivecrd.Drive or error if something went wrong
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
	vid := drive.Spec.VID
	pid := drive.Spec.PID
	for _, l := range lsblkOut {
		if strings.EqualFold(l.Serial, sn) && strings.EqualFold(l.Vendor, vid) &&
			strings.EqualFold(l.Model, pid) {
			device = l.Name
			break
		}
	}

	if device == "" {
		errMsg := fmt.Errorf("unable to find drive path by SN %s, VID %s, PID %s", sn, vid, pid)
		return "", errMsg
	}

	return device, nil
}
