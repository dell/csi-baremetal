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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/mocks"
)

func TestIPMI_GetBmcIP(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewIPMI(e)

	strOut := "IP Address Source       : DHCP Address \n IP Address              : 10.245.137.136"
	e.On(mocks.RunCmd, LanPrintCmd).Return(strOut, "", nil).Times(1)
	ip := l.GetBmcIP()
	assert.Equal(t, "10.245.137.136", ip)

	strOut = "IP Address Source       : DHCP Address \n"
	e.On(mocks.RunCmd, LanPrintCmd).Return(strOut, "", nil).Times(1)
	ip = l.GetBmcIP()
	assert.Equal(t, "", ip)

	expectedError := errors.New("ipmitool failed")
	e.On(mocks.RunCmd, LanPrintCmd).Return("", "", expectedError).Times(1)
	ip = l.GetBmcIP()
	assert.Equal(t, "", ip)
}
