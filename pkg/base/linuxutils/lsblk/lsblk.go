// Package lsblk contains code for running and interpreting output of system util lsblk
package lsblk

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
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
}

// LSBLK is a wrap for system lsblk util
type LSBLK struct {
	e   command.CmdExecutor
	log *logrus.Entry
}

// NewLSBLK is a constructor for LSBLK struct
func NewLSBLK(e command.CmdExecutor, log *logrus.Logger) *LSBLK {
	return &LSBLK{
		e:   e,
		log: log.WithField("component", "LSBLK"),
	}
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
	strOut, strErr, err := l.e.RunCmd(cmd)
	if err != nil {
		l.log.Errorf("lsblk failed, stdErr: %s, Error: %v", strErr, err)
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
		l.log.Errorf("key \"%s\" is not in map %v", outputKey, rawOut)
		return nil, fmt.Errorf("unexpected lsblk output format")
	}
	for _, d := range devs {
		if d.Type != romDeviceType {
			res = append(res, d)
		}
	}
	return res, nil
}
