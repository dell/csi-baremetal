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

package lsscsi

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

const (
	// LsscsiCmdImpl is a base CMD for lsscsi
	LsscsiCmdImpl = "lsscsi --no-nvme"
	// SCSIDeviceSizeCmdImpl is a CMD to get devices size by id
	SCSIDeviceSizeCmdImpl = LsscsiCmdImpl + " --brief --size %s"
	// SCSIDeviceCmdImpl is a CMD to get devices information about Vendor, Model and etc
	SCSIDeviceCmdImpl = LsscsiCmdImpl + " --classic %s"
	// SCSIType is a type of devices we search in lsscsi output
	SCSIType = "disk"
)

// WrapLsscsi is an interface that encapsulates operation with system lsscsi util
type WrapLsscsi interface {
	GetSCSIDevices() ([]*SCSIDevice, error)
}

// LSSCSI is a wrap for system lsscsi util
type LSSCSI struct {
	e   command.CmdExecutor
	log *logrus.Entry
}

// SCSIDevice represents devices in lsscsi output
type SCSIDevice struct {
	ID       string
	Path     string
	Size     int64
	Vendor   string
	Model    string
	Firmware string
}

// NewLSSCSI is a constructor for LSSCSI
func NewLSSCSI(e command.CmdExecutor, logger *logrus.Logger) *LSSCSI {
	return &LSSCSI{e: e, log: logger.WithField("component", "LSSCSI")}
}

// GetSCSIDevices gets information about SCSIDevice using lsscsi util
func (la *LSSCSI) GetSCSIDevices() ([]*SCSIDevice, error) {
	ll := la.log.WithField("method", "GetSCSIDevices")
	devices, err := la.getSCSIDevicesBasicInfo()
	if err != nil {
		return nil, err
	}
	for _, device := range devices {
		if err := la.fillDeviceSize(device); err != nil {
			ll.Errorf("lsscsi failed %v", err)
		}
		if err := la.fillDeviceInfo(device); err != nil {
			ll.Errorf("lsscsi failed %v", err)
		}
	}
	return devices, nil
}

// getSCSIDevicesBasicInfo returns information about device path and id, We call lsscsi --no-nvme.
// Using this command we can get list of all SCSI device and their Path and Id from the output of this command
// The output is easy to parse, because we know, that the Path and Id are on the last and the first positions in the output
// This command doesn't provide information about size.
// To facilitates the parsing of the output we use separate command lsscsi --no-nvme --brief --size to get information about size
func (la *LSSCSI) getSCSIDevicesBasicInfo() ([]*SCSIDevice, error) {
	//	/*Example output
	//	[0:0:0:0]    disk    VMware   Virtual disk     2.0   /dev/sda
	//	[0:0:1:0]    disk    VMware   Virtual disk     2.0   /dev/sdb
	//	[0:0:2:0]    disk    VMware   Virtual disk     2.0   /dev/sdc
	//	*/
	ll := la.log.WithField("method", "getSCSIDevicesBasicInfo")
	var devices []*SCSIDevice
	strOut, _, err := la.e.RunCmd(LsscsiCmdImpl)
	if err != nil {
		return nil, errors.New("unable to get devices basic info")
	}
	split := strings.Split(strOut, "\n")
	var re = regexp.MustCompile(`(\s+)`)
	for j := 0; j < len(split); j++ {
		s := re.ReplaceAllString(strings.TrimSpace(split[j]), " ")
		output := strings.Split(s, " ")
		if len(output) > 2 && output[1] == SCSIType {
			devices = append(devices, &SCSIDevice{ID: output[0], Path: output[len(output)-1]})
		} else {
			ll.Errorf("Unable to parse lsscsi output for line: %s", s)
		}
	}
	return devices, nil
}

// fillDeviceSize fill information about device size
// lsscsi --no-nvme --brief --size is easy to parse because size on the last position.
func (la *LSSCSI) fillDeviceSize(device *SCSIDevice) error {
	/*
	 [2:0:0:0]    /dev/sda   32.3GB
	*/
	strOut, _, err := la.e.RunCmd(fmt.Sprintf(SCSIDeviceSizeCmdImpl, device.ID),
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(SCSIDeviceSizeCmdImpl, ""))))
	if err != nil {
		return errors.New("unable to fill devices info")
	}
	var re = regexp.MustCompile(`(\s+)`)
	s := re.ReplaceAllString(strings.TrimSpace(strOut), " ")
	output := strings.Split(s, " ")
	bytes, err := util.StrToBytes(output[len(output)-1])
	if err != nil {
		return fmt.Errorf("unable to parse from %s for device %v", output[2], device)
	}
	device.Size = bytes
	return nil
}

// fillDeviceInfo returns information about device model, vendor and firmware
func (la *LSSCSI) fillDeviceInfo(device *SCSIDevice) error {
	/*
		Attached devices:
		Host: scsi0 Channel: 00 Target: 00 Lun: 00
		  Vendor: VMware   Model: Virtual disk     Rev: 2.0
		  Type:   Direct-Access                    ANSI SCSI revision: 06
	*/
	strOut, _, err := la.e.RunCmd(fmt.Sprintf(SCSIDeviceCmdImpl, device.ID),
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(SCSIDeviceCmdImpl, ""))))
	if err != nil {
		return errors.New("unable to get devices info")
	}
	split := strings.Split(strOut, "\n")
	var re = regexp.MustCompile(`(\s+)`)
	for _, line := range split {
		s := re.ReplaceAllString(strings.TrimSpace(line), " ")
		if strings.Contains(line, "Vendor:") {
			device.Vendor = la.parseLSSCSIOutput("Vendor:", s, "Model:", "Rev:")
		}
		if strings.Contains(line, "Model:") {
			device.Model = la.parseLSSCSIOutput("Model:", s, "Rev:")
		}
		if strings.Contains(line, "Rev:") {
			device.Firmware = la.parseLSSCSIOutput("Rev:", s)
		}
	}
	return nil
}

// parseLSSCSIOutput parses the output of the command. Example:
// Vendor: VMware   Model: Virtual disk     Rev: 2.0
// We do not know exactly when the name of the model, vendor and rev ends, for example, a model may consist of several words.
// Since the line contains both the vendor, the model, and the revision, we must precisely distinguish each value.
// We know that the vendor starts with the keyword Vendor:, a model with keyword Model: etc.
// Therefore, we look for the given keyword in the line, after that we take each word after the keyword,
// until we meet the next keyword.
// Everything between the keywords is the searched value, separated by spaces.
// searchString is a keyword. The value of this keyword we want to find in output. For example Vendor:
// lsscsiOutput is the command output
// keywords are the keyword we can met after the value. We use them to highlight the value between keywords.
func (la *LSSCSI) parseLSSCSIOutput(searchString string, lsscsiOutput string, keywords ...string) string {
	newLine := strings.Split(lsscsiOutput, " ")
	var idx int
	for i, str := range newLine {
		if str == searchString {
			idx = i
			break
		}
	}
	value := make([]string, 0)
	for i := idx + 1; i < len(newLine); i++ {
		var isKeyword bool
		for _, keyword := range keywords {
			if strings.Contains(newLine[i], keyword) {
				isKeyword = true
				break
			}
		}
		if isKeyword {
			break
		}
		value = append(value, newLine[i])
	}
	return strings.Join(value, " ")
}
