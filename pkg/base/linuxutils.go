package base

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
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
	lvm            = "/sbin/lvm"
	pvcreate       = "pvcreate"
	pvremove       = "pvremove"
	vgcreate       = "vgcreate"
	vgremove       = "vgremove"
	lvcreate       = "lvcreate"
	lvremove       = "lvremove"
	LVsInVGCmdTmpl = "lvs --select vg_name=%s -o lv_name --noheadings"
	PVsInVGCmdTmpl = "pvs --select vg_name=%s -o pv_name --noheadings"
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

func (l *LinuxUtils) SetLinuxUtilsExecutor(executor CmdExecutor) {
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

// SearchDrivePathBySN returns drive path based on drive S/N
func (l *LinuxUtils) SearchDrivePathBySN(sn string) (string, error) {
	lsblkOut, err := l.Lsblk("disk")
	if err != nil {
		return "", err
	}

	device := ""
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
	cmd := fmt.Sprintf("%s %s --yes %s", lvm, pvcreate, dev)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// PVRemove removes physical volumes
func (l *LinuxUtils) PVRemove(name string) error {
	cmd := fmt.Sprintf("%s %s --yes %s", lvm, pvremove, name)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// VGCreate creates volume group and based on provided physical volumes (pvs)
func (l *LinuxUtils) VGCreate(name string, pvs ...string) error {
	cmd := fmt.Sprintf("%s %s --yes %s %s", lvm, vgcreate, name, strings.Join(pvs, " "))
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// VGRemove removes volume group
func (l *LinuxUtils) VGRemove(name string) error {
	cmd := fmt.Sprintf("%s %s --yes %s", lvm, vgremove, name)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// LVCreate created logical volume in volume group
// size it is a string like 1.2G, 100M
func (l *LinuxUtils) LVCreate(name, size, vgName string) error {
	cmd := fmt.Sprintf("%s %s --yes --name %s --size %s %s", lvm, lvcreate, name, size, vgName)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// LVRemove removes logical volume
func (l *LinuxUtils) LVRemove(name, vgName string) error {
	cmd := fmt.Sprintf("%s %s --yes /dev/%s/%s", lvm, lvremove, vgName, name)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// IsVGContainsLVs checks whether VG vgName contains any LVs or no
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
	pvsCmd := fmt.Sprintf(PVsInVGCmdTmpl, " ")
	stdout, _, err := l.e.RunCmd(pvsCmd)
	if err != nil {
		return err
	}
	var wasError bool
	for _, pv := range strings.Split(strings.TrimSpace(stdout), "\n") {
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
