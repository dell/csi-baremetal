package lsscsi

import (
	"github.com/dell/csi-baremetal.git/pkg/mocks"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

var testLogger = logrus.New()

func TestLSSCSI_getSCSIDevicesBasicInfoSuccess(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	output := `	[0:0:0:0]    disk    VMware   Virtual disk     2.0   /dev/sda
		[0:0:1:0]    disk    VMware   Virtual disk     2.0   /dev/sdb
		[0:0:2:0]    cd/dvd   VMware   Virtual disk     2.0   /dev/sdc`
	e.On("RunCmd", LsscsiCmdImpl).Return(output, "", nil)

	devs, err := l.getSCSIDevicesBasicInfo()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(devs))

	assert.Equal(t, "[0:0:0:0]", devs[0].ID)
	assert.Equal(t, "/dev/sda", devs[0].Path)

	assert.Equal(t, "[0:0:1:0]", devs[1].ID)
	assert.Equal(t, "/dev/sdb", devs[1].Path)
}

func TestLSSCSI_getSCSIDevicesBasicInfoFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	e.On("RunCmd", LsscsiCmdImpl).Return("", "", fmt.Errorf("error"))

	_, err := l.getSCSIDevicesBasicInfo()
	assert.NotNil(t, err)
}

func TestLSSCSI_getSCSIDeviceSize(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	output := "[2:0:0:0]    /dev/sda   32.3GB"
	cmd := fmt.Sprintf(SCSIDeviceSizeCmdImpl, "[2:0:0:0]")
	e.On("RunCmd", cmd).Return(output, "", nil)

	devs := &SCSIDevice{ID: "[2:0:0:0]"}

	err := l.fillDeviceSize(devs)
	assert.Nil(t, err)
	assert.Equal(t, int64(34681860915), devs.Size)
}

func TestLSSCSI_getSCSIDeviceSizeWrongSizeFormat(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	output := "[2:0:0:0]    /dev/sda   asda"
	cmd := fmt.Sprintf(SCSIDeviceSizeCmdImpl, "[2:0:0:0]")
	e.On("RunCmd", cmd).Return(output, "", nil)

	devs := &SCSIDevice{ID: "[2:0:0:0]"}

	err := l.fillDeviceSize(devs)
	assert.NotNil(t, err)
}

func TestLSSCSI_getSCSIDeviceSizeFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	cmd := fmt.Sprintf(SCSIDeviceSizeCmdImpl, "[2:0:0:0]")
	e.On("RunCmd", cmd).Return("", "", fmt.Errorf("error"))

	devs := &SCSIDevice{ID: "[2:0:0:0]"}

	err := l.fillDeviceSize(devs)
	assert.NotNil(t, err)
}

func TestLSSCSI_getSCSIDeviceInfo(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	output := `Attached devices:
		Host: scsi0 Channel: 00 Target: 00 Lun: 00
		Vendor: VMware vendor   Model: Virtual disk model    Rev: 2.0
		Type:   Direct-Access                    ANSI SCSI revision: 06`

	devs := &SCSIDevice{ID: "[2:0:0:0]"}
	cmd := fmt.Sprintf(SCSIDeviceCmdImpl, devs.ID)

	e.On("RunCmd", cmd).Return(output, "", nil)

	err := l.fillDeviceInfo(devs)

	assert.Nil(t, err)
	assert.Equal(t, "VMware vendor", devs.Vendor)
	assert.Equal(t, "Virtual disk model", devs.Model)
	assert.Equal(t, "2.0", devs.Firmware)
}

func TestLSSCSI_getSCSIDeviceInfoFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	devs := &SCSIDevice{ID: "[2:0:0:0]"}
	cmd := fmt.Sprintf(SCSIDeviceCmdImpl, devs.ID)

	e.On("RunCmd", cmd).Return("", "", fmt.Errorf("error"))

	err := l.fillDeviceInfo(devs)

	assert.NotNil(t, err)
}

func TestLSSCSI_GetSCSIDevicesFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	e.On("RunCmd", LsscsiCmdImpl).Return("", "", fmt.Errorf("error"))

	_, err := l.GetSCSIDevices()
	assert.NotNil(t, err)
}

func TestLSSCSI_GetSCSIDevicesSizeCMDFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSSCSI(e, testLogger)

	output := `	[0:0:0:0]    disk    VMware   Virtual disk     2.0   /dev/sda
		[0:0:1:0]    disk    VMware   Virtual disk     2.0   /dev/sdb`
	e.On("RunCmd", LsscsiCmdImpl).Return(output, "", nil)

	output = "[0:0:0:0]    /dev/sda   32.3GB"
	cmd := fmt.Sprintf(SCSIDeviceSizeCmdImpl, "[0:0:0:0]")
	e.On("RunCmd", cmd).Return(output, "", nil)

	output = "Vendor: VMware vendor   Model: Virtual disk model    Rev: 2.0"
	cmd = fmt.Sprintf(SCSIDeviceCmdImpl, "[0:0:0:0]")
	e.On("RunCmd", cmd).Return(output, "", nil)

	cmd = fmt.Sprintf(SCSIDeviceSizeCmdImpl, "[0:0:1:0]")
	e.On("RunCmd", cmd).Return("", "", fmt.Errorf("error"))

	cmd = fmt.Sprintf(SCSIDeviceCmdImpl, "[0:0:1:0]")
	e.On("RunCmd", cmd).Return("", "", fmt.Errorf("error"))

	devs, err := l.GetSCSIDevices()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(devs))
}
