package linuxutils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lvm"
)

// LinuxUtils is the struct that unites linux commands for CSI purposes
type LinuxUtils struct {
	lvm.WrapLVM
	lsblk.WrapLsblk
	base.Partitioner
	e   command.CmdExecutor
	log *logrus.Entry
}

const (
	// IpmitoolCmd print bmc ip cmd with ipmitool
	IpmitoolCmd = " ipmitool lan print"
	// FindMntCmdTmpl find source device for target mount path cmd
	FindMntCmdTmpl = "findmnt --target %s --output SOURCE --noheadings" // add target path
)

// NewLinuxUtils returns new instance of LinuxUtils based on provided executor
// Receives cmdExecutor instance and logrus logger
// Returns an instance of LinuxUtils
func NewLinuxUtils(e command.CmdExecutor, logger *logrus.Logger) *LinuxUtils {
	l := &LinuxUtils{
		Partitioner: base.NewPartition(e),
		WrapLVM:     lvm.NewLVM(e, logger),
		WrapLsblk:   lsblk.NewLSBLK(e, logger),
		e:           e,
	}
	if l.e != nil {
		l.e.SetLogger(logger)
	}
	l.log = logger.WithField("component", "LinuxUtils")
	return l
}

// SetExecutor sets provided CmdExecutor to LinuxUtils
// Receives CmdExecutor
func (l *LinuxUtils) SetExecutor(executor command.CmdExecutor) {
	l.e = executor
}

// GetBmcIP returns BMC IP using IPMI
func (l *LinuxUtils) GetBmcIP() string {
	/* Sample output
	IP Address Source       : DHCP Address
	IP Address              : 10.245.137.136
	*/

	strOut, _, err := l.e.RunCmd(IpmitoolCmd)
	if err != nil {
		return ""
	}
	ipAddrStr := "ip address"
	var ip string
	//Regular expr to find ip address
	regex := regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)
	for _, str := range strings.Split(strOut, "\n") {
		str = strings.ToLower(str)
		if strings.Contains(str, ipAddrStr) {
			newStr := strings.Split(str, ":")
			if len(newStr) == 2 {
				s := strings.TrimSpace(newStr[1])
				matched := regex.MatchString(s)
				if matched {
					ip = s
				}
			}
		}
	}
	return ip
}

// SearchDrivePath if not defined returns drive path based on drive S/N.
// Receives an instance of drivecrd.Drive struct
// Returns drive's path based on provided drivecrd.Drive or error if something went wrong
// TODO AK8S-594 to check VID/PID as well
func (l *LinuxUtils) SearchDrivePath(drive *drivecrd.Drive) (string, error) {
	// device path might be already set by drivemgr
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

// FindMnt returns source of mount point for target
// Receives path of a mount point as target
// Returns mount point or empty string and error
// TODO: move to struct that calls Mounter
func (l *LinuxUtils) FindMnt(target string) (string, error) {
	/*
		Example of output:
		root@provo-goop:~# findmnt --target / --output SOURCE --noheadings
		/dev/mapper/unassigned--hostname--vg-root
	*/
	cmd := fmt.Sprintf(FindMntCmdTmpl, target)
	strOut, _, err := l.e.RunCmd(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strOut), nil
}

// GetPartitionNameByUUID gets partition name (for example, /dev/sda1, /dev/nvme1p2, /dev/loopback0p3) by UUID
// Receives a device path and uuid of partition to find
// Returns a partition path or error if something went wrong
func (l *LinuxUtils) GetPartitionNameByUUID(device, uuid string) (string, error) {
	if device == "" {
		return "", fmt.Errorf("unable to find partition name by UUID %s - device name is empty", uuid)
	}

	if uuid == "" {
		return "", fmt.Errorf("unable to find partition name for device %s partition UUID is empty", device)
	}

	// list partitions
	blockdevices, err := l.GetBlockDevices(device)
	if err != nil {
		return "", err
	}

	// try to find partition name
	for _, id := range blockdevices[0].Children {
		// ignore cases
		if strings.EqualFold(uuid, id.PartUUID) {
			// partition name not detected
			if id.Name == "" {
				return "", fmt.Errorf("partition %s for device %s found but name is not present", uuid, device)
			}
			return id.Name, nil
		}
	}

	return "", fmt.Errorf("unable to find partition name by UUID %s for device %s", uuid, device)
}
