package base

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
)

type LinuxUtils struct {
	Partitioner
	e   CmdExecutor
	log *logrus.Entry
}

type LsblkOutput struct {
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
	Children   []LsblkOutput `json:"children,omitempty"`
}

const (
	LsblkCmd       = "lsblk --paths --json --bytes --fs --output NAME,TYPE,SIZE,ROTA,SERIAL,WWN,VENDOR,MODEL,REV,MOUNTPOINT,FSTYPE"
	LsblkOutputKey = "blockdevices"
	IpmitoolCmd    = " ipmitool lan print"
	// cmd templates related to LVM
	LVsInVGCmdTmpl  = "/sbin/lvm lvs --select vg_name=%s -o lv_name --noheadings" // add VG name
	PVsInVGCmdTmpl  = "/sbin/lvm pvs --select vg_name=%s -o pv_name --noheadings" // add VG name
	EmptyName       = " "
	PVCreateCmdTmpl = "/sbin/lvm pvcreate --yes %s"                     // add PV name
	PVRemoveCmdTmpl = "/sbin/lvm pvremove --yes %s"                     // add PV name
	VGCreateCmdTmpl = "/sbin/lvm vgcreate --yes %s %s"                  // add VG name and PV names
	VGRemoveCmdTmpl = "/sbin/lvm vgremove --yes %s"                     // add VG name
	LVCreateCmdTmpl = "/sbin/lvm lvcreate --yes --name %s --size %s %s" // add LV name, size and VG name
	LVRemoveCmdTmpl = "/sbin/lvm lvremove --yes /dev/%s/%s"             // add VG name and LV name
)

// NewLinuxUtils returns new instance of LinuxUtils based on provided e
func NewLinuxUtils(e CmdExecutor, logger *logrus.Logger) *LinuxUtils {
	l := &LinuxUtils{
		Partitioner: &Partition{e: e},
		e:           e,
	}
	if l.e != nil {
		l.e.SetLogger(logger)
	}
	l.log = logger.WithField("component", "LinuxUtils")
	return l
}

func (l *LinuxUtils) SetExecutor(executor CmdExecutor) {
	l.e = executor
}

func (l *LinuxUtils) Lsblk(devType string) (*[]LsblkOutput, error) {
	strOut, strErr, err := l.e.RunCmd(LsblkCmd)
	if err != nil {
		l.log.Errorf("lsblk failed, stdErr: %s, Error: %v", strErr, err)
		return nil, err
	}
	rawOut := make(map[string][]LsblkOutput, 1)
	err = json.Unmarshal([]byte(strOut), &rawOut)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal output to LsblkOutput instance, error: %v", err)
	}
	res := make([]LsblkOutput, 0)
	var (
		devs []LsblkOutput
		ok   bool
	)
	if devs, ok = rawOut[LsblkOutputKey]; !ok {
		l.log.Errorf("key \"%s\" is not in map %v", LsblkOutputKey, rawOut)
		return nil, fmt.Errorf("unexpected lsblk output format")
	}
	for _, d := range devs {
		if d.Type == devType {
			res = append(res, d)
		}
	}
	return &res, nil
}

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
// TODO AK8S-594 to check VID/PID as well
func (l *LinuxUtils) SearchDrivePath(drive *drivecrd.Drive) (string, error) {
	// device path might be already set by hwmgr
	device := drive.Spec.Path
	if device != "" {
		return device, nil
	}

	// try to find it with lsblk
	lsblkOut, err := l.Lsblk("disk")
	if err != nil {
		return "", err
	}

	// get drive serial number
	sn := drive.Spec.SerialNumber
	for _, l := range *lsblkOut {
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

// PVCreate creates physical volume based on provided device or partition
func (l *LinuxUtils) PVCreate(dev string) error {
	cmd := fmt.Sprintf(PVCreateCmdTmpl, dev)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// PVRemove removes physical volumes, ignore error if PV doesn't exist
func (l *LinuxUtils) PVRemove(name string) error {
	cmd := fmt.Sprintf(PVRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "No PV label found") {
		return nil
	}
	return err
}

// VGCreate creates volume group and based on provided physical volumes (pvs)
// ignore error if VG already exists
func (l *LinuxUtils) VGCreate(name string, pvs ...string) error {
	cmd := fmt.Sprintf(VGCreateCmdTmpl, name, strings.Join(pvs, " "))
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "already exists") {
		return nil
	}
	return err
}

// VGRemove removes volume group, ignore error if VG doesn't exist
func (l *LinuxUtils) VGRemove(name string) error {
	cmd := fmt.Sprintf(VGRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd)
	if strings.Contains(stdErr, "not found") {
		return nil
	}
	return err
}

// LVCreate created logical volume in volume group, ignore error if LV already exists
// size it is a string like 1.2G, 100M
func (l *LinuxUtils) LVCreate(name, size, vgName string) error {
	cmd := fmt.Sprintf(LVCreateCmdTmpl, name, size, vgName)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "already exists") {
		return nil
	}
	return err
}

// LVRemove removes logical volume, ignore error if LV doesn't exist
func (l *LinuxUtils) LVRemove(name, vgName string) error {
	cmd := fmt.Sprintf(LVRemoveCmdTmpl, vgName, name)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "Failed to find logical volume") {
		return nil
	}
	return err
}

// IsVGContainsLVs checks whether VG vgName contains any LVs or no
// return true in case of error to prevent mistaken VG remove
func (l *LinuxUtils) IsVGContainsLVs(vgName string) bool {
	cmd := fmt.Sprintf(LVsInVGCmdTmpl, vgName)
	stdout, _, err := l.e.RunCmd(cmd)
	if err != nil {
		l.log.WithField("method", "IsVGContainsLVs").
			Errorf("Unable to check whether VG %s contains LVs or no. Suppose yes.", vgName)
		return true
	}
	res := len(strings.TrimSpace(stdout)) > 0
	return res
}

// RemoveOrphanPVs removes PVs that do not have VG
func (l *LinuxUtils) RemoveOrphanPVs() error {
	pvsCmd := fmt.Sprintf(PVsInVGCmdTmpl, EmptyName)
	stdout, _, err := l.e.RunCmd(pvsCmd)
	if err != nil {
		return err
	}
	var wasError bool
	for _, pv := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if len(pv) == 0 {
			continue
		}
		if err := l.PVRemove(pv); err != nil {
			l.log.WithField("method", "RemoveOrphanPVs").Errorf("Unable to remove pv %s: %v", pv, err)
			wasError = true
		}
	}
	if wasError {
		return errors.New("not all PVs were removed")
	}
	return nil
}
