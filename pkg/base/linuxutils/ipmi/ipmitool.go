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

// Package ipmi contains code for running and interpreting output of system ipmitool util
package ipmi

import (
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/metrics/common"
)

const (
	// LanPrintCmd print bmc ip cmd with ipmitool
	LanPrintCmd = " ipmitool lan print"
)

// WrapIpmi is an interface that encapsulates operation with system ipmi util
type WrapIpmi interface {
	GetBmcIP() string
}

// IPMI is implementation for WrapImpi interface
type IPMI struct {
	e command.CmdExecutor
}

// NewIPMI is a constructor for LSBLK struct
func NewIPMI(e command.CmdExecutor) *IPMI {
	return &IPMI{e: e}
}

// GetBmcIP returns BMC IP using ipmitool
func (i *IPMI) GetBmcIP() string {
	/* Sample output
	IP Address Source       : DHCP Address
	IP Address              : 10.245.137.136
	*/

	evalDuration := common.SystemCMDDuration.EvaluateDuration(prometheus.Labels{"name": LanPrintCmd, "method": "GetBmcIP"})
	strOut, _, err := i.e.RunCmd(LanPrintCmd)
	if err != nil {
		return ""
	}
	evalDuration()
	ipAddrStr := "ip address"
	var ip string
	// Regular expr to find ip address
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
