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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/mocks"
)

func TestSMARCTL_GetDriveInfoByPath(t *testing.T) {
	output := `{ 
				"serial_number": "29P4K65PF9NF", 
				"device": { 
					"name": "/dev/sdd", 
					"info_name": "/dev/sdd [SAT]", 
					"type": "sat", "protocol": "ATA"}, 
					"rotation_rate": 7200
				}`
	outputHealth := `{
    "smart_status": {
        "passed": true
    }}`
	cmd := fmt.Sprintf(SmartctlDeviceInfoCmdImpl, "/dev/sdd")
	cmdHealth := fmt.Sprintf(SmartctlHealthCmdImpl, "/dev/sdd")
	e := &mocks.GoMockExecutor{}
	l := NewSMARTCTL(e)

	e.On("RunCmd", cmd).Return(output, "", nil)
	e.On("RunCmd", cmdHealth).Return(outputHealth, "", nil)
	smartInfo, err := l.GetDriveInfoByPath("/dev/sdd")
	assert.Nil(t, err)

	assert.Equal(t, smartInfo.SerialNumber, "29P4K65PF9NF")
	assert.Equal(t, smartInfo.Rotation, 7200)
	assert.Equal(t, smartInfo.SmartStatus, map[string]bool{"passed": true})
}

func TestSMARCTL_GetDriveInfoByPathFails(t *testing.T) {
	cmd := fmt.Sprintf(SmartctlDeviceInfoCmdImpl, "/dev/sdd")
	e := &mocks.GoMockExecutor{}
	l := NewSMARTCTL(e)

	e.On("RunCmd", cmd).Return("", "", fmt.Errorf("error"))

	_, err := l.GetDriveInfoByPath("/dev/sdd")
	assert.NotNil(t, err)
}

func TestSMARCTL_GetDriveInfoByPathUnmarshallError(t *testing.T) {
	output := `{ 
				"serial_number": "29P4K65PF9NF", 
				"device": { 
					"name": "/dev/sdd", 
					"info_name": "/dev/sdd [SAT]", 
					"type": "sat", "protocol": "ATA"}, 
					"rotation_rate": 7200, 
					"smart_status": { 
						"passed": "true"
					} 
				}`
	cmd := fmt.Sprintf(SmartctlDeviceInfoCmdImpl, "/dev/sdd")
	e := &mocks.GoMockExecutor{}
	l := NewSMARTCTL(e)

	e.On("RunCmd", cmd).Return(output, "", nil)

	_, err := l.GetDriveInfoByPath("/dev/sdd")
	assert.NotNil(t, err)
}

func TestSMARCTL_fillSmartStatus(t *testing.T) {
	cmd := fmt.Sprintf(SmartctlHealthCmdImpl, "/dev/sdd")
	e := &mocks.GoMockExecutor{}
	l := NewSMARTCTL(e)

	e.On("RunCmd", cmd).Return("", "", fmt.Errorf("error"))

	err := l.fillSmartStatus(&DeviceSMARTInfo{}, "/dev/sdd")
	assert.NotNil(t, err)
}

func TestSMARCTL_fillSmartStatusUnmarshallError(t *testing.T) {
	output := `{
					"smart_status": { 
						"passed": "true",
					} 
				}`
	cmd := fmt.Sprintf(SmartctlHealthCmdImpl, "/dev/sdd")
	e := &mocks.GoMockExecutor{}
	l := NewSMARTCTL(e)

	e.On("RunCmd", cmd).Return(output, "", nil)

	err := l.fillSmartStatus(&DeviceSMARTInfo{}, "/dev/sdd")
	assert.NotNil(t, err)
}
