package lsblk

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/command"
)

// LSBLKv2 implements lsblk version >= 2.34
type LSBLKv2 struct {
	e       command.CmdExecutor
	version Version
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
func (l *LSBLKv2) GetBlockDevices(device string) ([]BlockDevice, error) {
	// todo get rid of code duplicate
	cmd := fmt.Sprintf(CmdTmpl, device)
	strOut, _, err := l.e.RunCmd(cmd,
		command.UseMetrics(true),
		command.CmdName(strings.TrimSpace(fmt.Sprintf(CmdTmpl, ""))))
	if err != nil {
		return nil, err
	}

	rawOut := make(map[string][]BlockDeviceV2, 1)
	err = json.Unmarshal([]byte(strOut), &rawOut)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal output to BlockDevice instance, error: %v", err)
	}

	var (
		ok     bool
		devsV2 []BlockDeviceV2
	)

	if devsV2, ok = rawOut[outputKey]; !ok {
		return nil, fmt.Errorf("unexpected lsblk output format, missing \"%s\" key", outputKey)
	}

	res := make([]BlockDevice, 0)
	for _, d := range devsV2 {
		if d.Type != romDeviceType {
			res = append(res, convertToV1(d))
		}
	}

	return res, nil
}

// SearchDrivePath if not defined returns drive path based on drive S/N, VID and PID.
// Receives an instance of drivecrd.Drive struct
// Returns drive's path based on provided drivecrd.Drive or error if something went wrong
func (l *LSBLKv2) SearchDrivePath(drive *drivecrd.Drive) (string, error) {
	return searchDrivePath(drive, l)
}

// GetVersion returns version of lsblk utility
func (l *LSBLKv2) GetVersion() Version {
	return l.version
}
