package base

import (
	"encoding/json"
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
