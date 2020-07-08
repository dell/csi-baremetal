// Package ipmi contains code for running and interpreting output of system ipmitool util
package ipmi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal.git/pkg/mocks"
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
