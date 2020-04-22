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

// LinuxUtils is the struct that unites linux commands for CSI purposes
type LinuxUtils struct {
	Partitioner
	e   CmdExecutor
	log *logrus.Entry
}

// LsblkOutput is the struct that represents output of lsblk command for a device
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
	PartUUID   string        `json:"partuuid,omitempty"`
	Children   []LsblkOutput `json:"children,omitempty"`
}

const (
	// LsblkCmdTmpl adds device name, if add empty string - command will print info about all devices
	LsblkCmdTmpl = "lsblk %s --paths --json --bytes --fs " +
		"--output NAME,TYPE,SIZE,ROTA,SERIAL,WWN,VENDOR,MODEL,REV,MOUNTPOINT,FSTYPE,PARTUUID"
	// LsblkOutputKey is the key to find block devices in lsblk json output
	LsblkOutputKey = "blockdevices"
	// IpmitoolCmd print bmc ip cmd with ipmitool
	IpmitoolCmd = " ipmitool lan print"
	// LVsInVGCmdTmpl print LVs in VG cmd
	LVsInVGCmdTmpl = "/sbin/lvm lvs --select vg_name=%s -o lv_name --noheadings" // add VG name
	// PVsInVGCmdTmpl print PVs in VG cmd
	PVsInVGCmdTmpl = "/sbin/lvm pvs --select vg_name=%s -o pv_name --noheadings" // add VG name
	// EmptyName for PVs in VG
	EmptyName = " "
	// PVCreateCmdTmpl create PV cmd
	PVCreateCmdTmpl = "/sbin/lvm pvcreate --yes %s" // add PV name
	// PVRemoveCmdTmpl remove PV cmd
	PVRemoveCmdTmpl = "/sbin/lvm pvremove --yes %s" // add PV name
	// VGCreateCmdTmpl create VG on provided PVs cmd
	VGCreateCmdTmpl = "/sbin/lvm vgcreate --yes %s %s" // add VG name and PV names
	// VGRemoveCmdTmpl remove VG cmd
	VGRemoveCmdTmpl = "/sbin/lvm vgremove --yes %s" // add VG name
	// LVCreateCmdTmpl create LV on provided VG cmd
	LVCreateCmdTmpl = "/sbin/lvm lvcreate --yes --name %s --size %s %s" // add LV name, size and VG name
	// LVRemoveCmdTmpl remove LV cmd
	LVRemoveCmdTmpl = "/sbin/lvm lvremove --yes %s" // add full LV name
	// FindMntCmdTmpl find source device for target mount path cmd
	FindMntCmdTmpl = "findmnt --target %s --output SOURCE --noheadings" // add target path
	// VGByLVCmdTmpl find VG by LV cmd
	VGByLVCmdTmpl = "lvs %s --options vg_name --noheadings" // add LV name
	// VGFreeSpaceCmdTmpl check VG free space cmd
	VGFreeSpaceCmdTmpl = "vgs %s --options vg_free --units b --noheadings" // add VG name
)

// GetE returns CmdExecutor field from LinuxUtils
func (l *LinuxUtils) GetE() CmdExecutor {
	return l.e
}

// NewLinuxUtils returns new instance of LinuxUtils based on provided executor
// Receives cmdExecutor instance and logrus logger
// Returns an instance of LinuxUtils
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

// SetExecutor sets provided CmdExecutor to LinuxUtils
// Receives CmdExecutor
func (l *LinuxUtils) SetExecutor(executor CmdExecutor) {
	l.e = executor
}

// Lsblk run os lsblk command for device and construct LsblkOutput struct based on output
// Receives device path. If device is empty string, info about all devices will be collected
// Returns slice of LsblkOutput structs or error if something went wrong
func (l *LinuxUtils) Lsblk(device string) ([]LsblkOutput, error) {
	cmd := fmt.Sprintf(LsblkCmdTmpl, device)
	strOut, strErr, err := l.e.RunCmd(cmd)
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
		if d.Type != RomDeviceType {
			res = append(res, d)
		}
	}
	return res, nil
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
	// device path might be already set by hwmgr
	device := drive.Spec.Path
	if device != "" {
		return device, nil
	}

	// try to find it with lsblk
	lsblkOut, err := l.Lsblk("")
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

// PVCreate creates physical volume based on provided device or partition
// Receives device path
// Returns error if something went wrong
func (l *LinuxUtils) PVCreate(dev string) error {
	cmd := fmt.Sprintf(PVCreateCmdTmpl, dev)
	_, _, err := l.e.RunCmd(cmd)
	return err
}

// PVRemove removes physical volumes, ignore error if PV doesn't exist
// Receives name of a physical volume to delete
// Returns error if something went wrong
func (l *LinuxUtils) PVRemove(name string) error {
	cmd := fmt.Sprintf(PVRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "No PV label found") {
		return nil
	}
	return err
}

// VGCreate creates volume group and based on provided physical volumes (pvs). Ignore error if VG already exists
// Receives name of VG to create and names of physical volumes which VG should based on
// Returns error if something went wrong
func (l *LinuxUtils) VGCreate(name string, pvs ...string) error {
	cmd := fmt.Sprintf(VGCreateCmdTmpl, name, strings.Join(pvs, " "))
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "already exists") {
		return nil
	}
	return err
}

// VGRemove removes volume group, ignore error if VG doesn't exist
// Receives name of VG to remove
// Returns error if something went wrong
func (l *LinuxUtils) VGRemove(name string) error {
	cmd := fmt.Sprintf(VGRemoveCmdTmpl, name)
	_, stdErr, err := l.e.RunCmd(cmd)
	if strings.Contains(stdErr, "not found") {
		return nil
	}
	return err
}

// LVCreate created logical volume in volume group, ignore error if LV already exists
// Receives name of created LV, size which is a string like 1.2G, 100M and name of VG which LV should be based on
// Returns error if something went wrong
func (l *LinuxUtils) LVCreate(name, size, vgName string) error {
	cmd := fmt.Sprintf(LVCreateCmdTmpl, name, size, vgName)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "already exists") {
		return nil
	}
	return err
}

// LVRemove removes logical volume, ignore error if LV doesn't exist
// Receives fullLVName that is a path to LV
// Returns error if something went wrong
func (l *LinuxUtils) LVRemove(fullLVName string) error {
	cmd := fmt.Sprintf(LVRemoveCmdTmpl, fullLVName)
	_, stdErr, err := l.e.RunCmd(cmd)
	if err != nil && strings.Contains(stdErr, "Failed to find logical volume") {
		return nil
	}
	return err
}

// IsVGContainsLVs checks whether VG vgName contains any LVs or no
// Receives Volume Group name to check
// Returns true in case of error to prevent mistaken VG remove
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
// Returns error if something went wrong
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

// FindVgNameByLvName search VG name by LV name, LV name should be full
// Receives LV name to find its VG
// Returns VG name or empty string and error
func (l *LinuxUtils) FindVgNameByLvName(lvName string) (string, error) {
	/*
		Example of output:
		root@provo-goop:~# lvs /dev/mapper/unassigned--hostname--vg-root --options vg_name --noheadings
			  unassigned-hostname-vg
	*/
	cmd := fmt.Sprintf(VGByLVCmdTmpl, lvName)
	strOut, _, err := l.e.RunCmd(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strOut), nil
}

// GetVgFreeSpace returns VG free space in bytes
// Receives VG name to count ints free space
// Returns -1 in case of error and error
func (l *LinuxUtils) GetVgFreeSpace(vgName string) (int64, error) {
	/*
		Example of output:
		root@provo-goop:~# vgs --options vg_free unassigned-hostname-vg --units b --nosuffix --noheadings
			      0
	*/

	if vgName == "" {
		return -1, errors.New("VG name shouldn't be an empty string")
	}

	cmd := fmt.Sprintf(VGFreeSpaceCmdTmpl, vgName)
	strOut, _, err := l.e.RunCmd(cmd)
	if err != nil {
		return -1, err
	}

	bytes, err := StrToBytes(strings.TrimSpace(strOut))
	if err != nil {
		return -1, err
	}

	return bytes, nil
}

// FindMnt returns source of mount point for target
// Receives path of a mount point as target
// Returns mount point or empty string and error
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
	blockdevices, err := l.Lsblk(device)
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
