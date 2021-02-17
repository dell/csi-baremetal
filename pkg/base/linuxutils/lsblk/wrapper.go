package lsblk

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/command"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
)

const (
	lsblkCmd = "lsblk"
	// VersionCmdTmpl returns lsblk Version
	VersionCmdTmpl = lsblkCmd + " --Version"
	// CmdTmpl adds device name, if add empty string - command will print info about all devices
	CmdTmpl = lsblkCmd + " %s --paths --json --bytes --fs " +
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
	GetVersion() Version
}

// Version represents version of lsblk util
type Version struct {
	major uint16
	minor uint16
	patch uint16
}

// NewLSBLK is a constructor for LSBLK struct
func NewLSBLK(e command.CmdExecutor) WrapLsblk {
	e.SetLevel(logrus.TraceLevel)
	// detect Version
	version, err := getVersion(e)
	if err != nil {
		// todo need to return error
		return nil
	}

	if version.major >= 2 && version.minor >= 34 {
		return &LSBLKv2{e: e, version: version}
	}

	return &LSBLK{e: e, version: version}
}

// getVersion receives Version of lsblk utility
// output examples: lsblk from util-linux 2.34, lsblk from util-linux 2.31.1
func getVersion(e command.CmdExecutor) (Version, error) {
	cmdOut, _, err := e.RunCmd(VersionCmdTmpl)
	if err != nil {
		return Version{}, err
	}

	versionString := regexp.MustCompile(`[0-9]+\.[0-9]+\.?[0-9]?`).FindString(cmdOut)
	if versionString == "" {
		return Version{}, errTypes.ErrorFailedParsing
	}

	versionSplit := strings.Split(versionString, ".")
	major, _ := strconv.ParseUint(versionSplit[0], 10, 64)
	minor, _ := strconv.ParseUint(versionSplit[1], 10, 64)
	version := Version{major: uint16(major), minor: uint16(minor)}

	if len(versionSplit) == 3 {
		// add patch Version as well
		patch, _ := strconv.ParseUint(versionSplit[2], 10, 64)
		version.patch = uint16(patch)
	}

	return version, nil
}

func searchDrivePath(drive *drivecrd.Drive, lsblk WrapLsblk) (string, error) {
	// device path might be already set by hwmgr
	device := drive.Spec.Path
	if device != "" {
		return device, nil
	}

	// try to find it with lsblk
	lsblkOut, err := lsblk.GetBlockDevices("")
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
