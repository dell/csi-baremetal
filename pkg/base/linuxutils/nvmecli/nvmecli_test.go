package nvmecli

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

var (
	testLogger = logrus.New()
	testPath   = "/dev/nvme9n1"
)

func TestNVMECLI_GetNVMDevicesSuccess(t *testing.T) {
	output := `
	{
		"Devices" : [
			{
	  			"DevicePath" : "/dev/nvme9n1",
      			"Firmware" : "VDV1DP21",
      			"Index" : 9,
      			"ModelNumber" : "Dell Express Flash NVMe P4510 4TB SFF",
      			"ProductName" : "Unknown Device",
      			"SerialNumber" : "PHLJ9135027L4P0DGN",
      			"UsedBytes" : 4000000000000,
      			"MaximiumLBA" : 7812500000,
       	  		"PhysicalSize" : 4000000000000,
    		  	"SectorSize" : 512
    		}
		]
	}`
	health := `{
  		"critical_warning" : 0,
 		"temperature" : 302,
  		"avail_spare" : 100,
  		"spare_thresh" : 10,
  		"percent_used" : 0,
  		"data_units_read" : 97704077
	}
`
	vendor := `{
  		"vid" : 32902,
  		"ssvid" : 4136,
  		"sn" : "PHLJ914500JE4P0DGN  ",
  		"mn" : "Dell Express Flash NVMe P4510 4TB SFF   ",
  		"fr" : "VDV1DP21",
  		"rab" : 0,
  		"ieee" : 6083300,
  		"cmic" : 0,
  		"mdts" : 5
	}
	`
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)

	e.On("RunCmd", NVMeDeviceCmdImpl).Return(output, "", nil)
	e.On("RunCmd", fmt.Sprintf(NVMeHealthCmdImpl, "/dev/nvme9n1")).Return(health, "", nil)
	e.On("RunCmd", fmt.Sprintf(NVMeVendorCmdImpl, "/dev/nvme9n1")).Return(vendor, "", nil)
	devices, err := l.GetNVMDevices()
	assert.Nil(t, err)

	assert.Equal(t, 1, len(devices))
	assert.Equal(t, "PHLJ9135027L4P0DGN", devices[0].SerialNumber)
	assert.Equal(t, int64(4000000000000), devices[0].PhysicalSize)
	assert.Equal(t, "/dev/nvme9n1", devices[0].DevicePath)
	assert.Equal(t, "VDV1DP21", devices[0].Firmware)
	assert.Equal(t, "Dell Express Flash NVMe P4510 4TB SFF", devices[0].ModelNumber)
	assert.Equal(t, apiV1.HealthGood, devices[0].Health)
	assert.Equal(t, 32902, devices[0].Vendor)
}

func TestNVMECLI_GetNVMDevicesFails(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)

	e.On("RunCmd", NVMeDeviceCmdImpl).Return("", "", fmt.Errorf("error"))

	_, err := l.GetNVMDevices()
	assert.NotNil(t, err)
}

func TestNVMECLI_GetNVMDevicesUnmarshallError(t *testing.T) {
	output := `
	{
		"Devices" : [
			{
	  			"DevicePath" : "/dev/nvme9n1",
      			"Firmware" : "VDV1DP21",
      			"Index" : 9,
      			"ModelNumber" : "Dell Express Flash NVMe P4510 4TB SFF",
      			"ProductName" : "Unknown Device",
      			"SerialNumber" : "PHLJ9135027L4P0DGN",
      			"UsedBytes" : 4000000000000,
      			"MaximiumLBA" : 7812500000,
       	  		"PhysicalSize" : "4000000000000",
    		  	"SectorSize" : 512
    		}
		]
	}`
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)

	e.On("RunCmd", NVMeDeviceCmdImpl).Return(output, "", nil)
	_, err := l.GetNVMDevices()
	assert.NotNil(t, err)
}

func TestNVMECLI_GetNVMDevicesWrongKey(t *testing.T) {
	output := `
	{
		"Wrong Key" : [
			{
	  			"DevicePath" : "/dev/nvme9n1",
      			"Firmware" : "VDV1DP21",
      			"Index" : 9,
      			"ModelNumber" : "Dell Express Flash NVMe P4510 4TB SFF",
      			"ProductName" : "Unknown Device",
      			"SerialNumber" : "PHLJ9135027L4P0DGN",
      			"UsedBytes" : 4000000000000,
      			"MaximiumLBA" : 7812500000,
       	  		"PhysicalSize" : 4000000000000,
    		  	"SectorSize" : 512
    		}
		]
	}`
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)

	e.On("RunCmd", NVMeDeviceCmdImpl).Return(output, "", nil)
	_, err := l.GetNVMDevices()
	assert.NotNil(t, err)
}

func TestNVMECLI_getNVMDeviceHealthBad(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)

	health := `{
  		"critical_warning" : 4
	}
	`
	e.On("RunCmd", fmt.Sprintf(NVMeHealthCmdImpl, testPath)).Return(health, "", nil)
	deviceHealth := l.getNVMDeviceHealth(testPath)
	assert.Equal(t, apiV1.HealthBad, deviceHealth)
}
func TestNVMECLI_getNVMDeviceHealthSuspect(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)
	health := `{
  		"critical_warning" : 5
	}
	`
	e.On("RunCmd", fmt.Sprintf(NVMeHealthCmdImpl, testPath)).Return(health, "", nil)
	deviceHealth := l.getNVMDeviceHealth(testPath)
	assert.Equal(t, apiV1.HealthSuspect, deviceHealth)
}

func TestNVMECLI_getNVMDeviceHealthGood(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)
	health := `{
  		"critical_warning" : 0
	}
	`
	e.On("RunCmd", fmt.Sprintf(NVMeHealthCmdImpl, testPath)).Return(health, "", nil)
	deviceHealth := l.getNVMDeviceHealth(testPath)
	assert.Equal(t, apiV1.HealthGood, deviceHealth)
}

func TestNVMECLI_getNVMDeviceHealthUnmarshallError(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)
	//unmarshall error
	health := `{
  		"critical_warning" : "5"
	}
	`
	e.On("RunCmd", fmt.Sprintf(NVMeHealthCmdImpl, testPath)).Return(health, "", nil)
	deviceHealth := l.getNVMDeviceHealth(testPath)
	assert.Equal(t, apiV1.HealthUnknown, deviceHealth)
}

func TestNVMECLI_getNVMDeviceHealthCMDError(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)
	e.On("RunCmd", fmt.Sprintf(NVMeHealthCmdImpl, testPath)).Return("", "", fmt.Errorf("error"))
	deviceHealth := l.getNVMDeviceHealth(testPath)
	assert.Equal(t, apiV1.HealthUnknown, deviceHealth)
}

func TestNVMECLI_getNVMDeviceVendorFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)
	device := NVMDevice{
		DevicePath: "/dev/nvme9n1",
	}
	e.On("RunCmd", fmt.Sprintf(NVMeVendorCmdImpl, "/dev/nvme9n1")).Return("", "", fmt.Errorf("error"))
	l.fillNVMDeviceVendor(&device)
	assert.Equal(t, 0, device.Vendor)
}

func TestNVMECLI_getNVMDeviceVendorUnmarshalError(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)
	vendor := `{ (
  		"vid" : 32902
	}
	`
	device := NVMDevice{
		DevicePath: "/dev/nvme9n1",
	}
	e.On("RunCmd", fmt.Sprintf(NVMeVendorCmdImpl, "/dev/nvme9n1")).Return(vendor, "", nil)
	l.fillNVMDeviceVendor(&device)
	assert.Equal(t, 0, device.Vendor)
}

func TestNVMECLI_isOneOfBitsSet(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewNVMECLI(e, testLogger)
	set := l.isOneOfBitsSet(1, 0)
	assert.True(t, set)
	set = l.isOneOfBitsSet(4, 3)
	assert.False(t, set)
	set = l.isOneOfBitsSet(5, 64)
	assert.False(t, set)
}
