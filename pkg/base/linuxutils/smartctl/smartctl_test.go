package smartctl

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
					"rotation_rate": 7200, 
					"smart_status": { 
						"passed": true
					} 
				}`
	cmd := fmt.Sprintf(SmartctlDeviceInfoCmdImpl, "/dev/sdd")
	e := &mocks.GoMockExecutor{}
	l := NewSMARTCTL(e)

	e.On("RunCmd", cmd).Return(output, "", nil)

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
