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

package wbt

import (
	"fmt"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"os/exec"
)

const (
	// isWBTEnabledCmdImpl is a CMD to check current WBT value
	isWBTEnabledCmdImpl = "cat /sys/block/%s/queue/wbt_lat_usec"
	// enableWBTCmdImpl is a CMD to check current WBT value
	enableWBTCmdImpl = "echo -1 > /sys/block/%s/queue/wbt_lat_usec"
	// disableWBTCmdImpl is a CMD to check current WBT value
	disableWBTCmdImpl = "echo 0 > /sys/block/%s/queue/wbt_lat_usec"
	//
	defaultShellCmd = "sh"
	shellCmdOption = "-c"
)

// WrapWbt is an interface that encapsulates operation with WBT
type WrapWbt interface {
	IsEnabled (device string) (bool, error)
	Disable (device string) error
	Enable (device string) error
}

type Wbt struct {
	e command.CmdExecutor
}

func NewWBT(e command.CmdExecutor) *Wbt {
	return &Wbt{e: e}
}

// IsEnabled gets SMART information about device by its Path using smartctl util
func (w *Wbt) IsEnabled (device string) (bool, error) {
	strOut, _, err := w.e.RunCmd(fmt.Sprintf(isWBTEnabledCmdImpl, device))
	if err != nil {
		return false, err
	}

	if strOut == "0" {
		return false, nil
	}

	return true, nil
}

// Disable gets SMART information about device by its Path using smartctl util
func (w *Wbt) Disable (device string) error {
	cmd := exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(disableWBTCmdImpl, device))
	_, _, err := w.e.RunCmd(cmd)
	if err != nil {
		return err
	}

	return nil
}

// Enable gets SMART information about device by its Path using smartctl util
func (w *Wbt) Enable (device string) error {
	cmd := exec.Command(defaultShellCmd, shellCmdOption, fmt.Sprintf(enableWBTCmdImpl, device))
	_, _, err := w.e.RunCmd(cmd)
	if err != nil {
		return err
	}

	return nil
}
