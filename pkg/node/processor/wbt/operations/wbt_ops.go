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

package operations

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dell/csi-baremetal/pkg/base/command"
)

// TODO - remove cat and echo usage (https://github.com/dell/csi-baremetal/issues/653)
const (
	wbtValuePath = "/sys/block/%s/queue/wbt_lat_usec"
	// isWBTEnabledCmdImpl is a CMD to check current WBT value
	checkWbtValueCmd = "cat " + wbtValuePath
	// enableWBTCmdImpl is a CMD to check current WBT value
	restoreWbtValueCmd = "echo -1 > " + wbtValuePath
	// disableWBTCmdImpl is a CMD to check current WBT value
	setWbtValueCmd = "echo %d > " + wbtValuePath
	// exec cmd via sh
	defaultShellCmd = "sh"
	shellCmdOption  = "-c"

	invalidArgError = "Invalid argument"
)

// WrapWbt is an interface that encapsulates operation with WBT
type WrapWbt interface {
	SetValue(device string, value uint32) error
	RestoreDefault(device string) error
}

// Wbt is WrapWbt implementation with CMD executor
type Wbt struct {
	e command.CmdExecutor
}

// NewWbt returns Wbt instance
func NewWbt(e command.CmdExecutor) *Wbt {
	return &Wbt{e: e}
}

// SetValue checks Wbt value for given device and change it if not equal
// Example output: sh -c echo <value> /sys/block/<device>/queue/wbt_lat_usec
func (w *Wbt) SetValue(device string, value uint32) error {
	strOut, stdErr, err := w.e.RunCmd(fmt.Sprintf(checkWbtValueCmd, device))
	if err != nil {
		// Invalid argument is acceptable error, means that value for WBT is not set yet
		if !strings.Contains(stdErr, invalidArgError) {
			return err
		}
	}

	if strOut == strconv.Itoa(int(value)) {
		return nil
	}

	cmd := exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(setWbtValueCmd, value, device))
	_, _, err = w.e.RunCmd(cmd)
	if err != nil {
		return err
	}

	return nil
}

// RestoreDefault restores default Wbt value for given device
// Example output: sh -c echo -1 /sys/block/<device>/queue/wbt_lat_usec
func (w *Wbt) RestoreDefault(device string) error {
	cmd := exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(restoreWbtValueCmd, device))
	_, _, err := w.e.RunCmd(cmd)
	if err != nil {
		return err
	}

	return nil
}
