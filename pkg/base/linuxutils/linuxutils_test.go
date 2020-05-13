package linuxutils

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var (
	luLogger           = logrus.New()
	lsblkAllDevicesCmd = fmt.Sprintf(lsblk.CmdTmpl, "")
)

func TestLinuxUtils_SetLinuxeUtilsExecutor(t *testing.T) {
	e1 := new(command.Executor)
	e1.SetLogger(luLogger)
	e2 := new(command.Executor)
	e2.SetLogger(logrus.New())

	l := NewLinuxUtils(e1, luLogger)
	assert.Equal(t, l.e, e1)
	l.SetExecutor(e2)
	assert.Equal(t, l.e, e2)
}

func Test_GetBmcIP(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLinuxUtils(e, luLogger)

	strOut := "IP Address Source       : DHCP Address \n IP Address              : 10.245.137.136"
	e.On(mocks.RunCmd, IpmitoolCmd).Return(strOut, "", nil).Times(1)
	ip := l.GetBmcIP()
	assert.Equal(t, "10.245.137.136", ip)

	strOut = "IP Address Source       : DHCP Address \n"
	e.On(mocks.RunCmd, IpmitoolCmd).Return(strOut, "", nil).Times(1)
	ip = l.GetBmcIP()
	assert.Equal(t, "", ip)

	expectedError := errors.New("ipmitool failed")
	e.On(mocks.RunCmd, IpmitoolCmd).Return("", "", expectedError).Times(1)
	ip = l.GetBmcIP()
	assert.Equal(t, "", ip)
}

func TestLinuxUtils_SearchDrivePathBySN(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	e.OnCommand(lsblkAllDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil).Times(2)
	l := NewLinuxUtils(e, luLogger)

	// success when path is set by hwgmr
	var drive = new(drivecrd.Drive)
	expectedDev := "/dev/sda"
	drive.Spec.Path = expectedDev
	drive.Spec.SerialNumber = "hdd1"
	dev, err := l.SearchDrivePath(drive)
	assert.Nil(t, err)
	assert.Equal(t, expectedDev, dev)

	// success when path is not set by hwmgr
	drive.Spec.Path = ""
	dev, err = l.SearchDrivePath(drive)
	assert.Nil(t, err)
	assert.Equal(t, expectedDev, dev)

	// fail: dev was not found
	drive.Spec.SerialNumber = "hdd12341"
	dev, err = l.SearchDrivePath(drive)
	assert.Empty(t, dev)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to find drive path")

	// fail: lsblk was failed
	e.OnCommand(lsblkAllDevicesCmd).Return("", "", errors.New("error"))
	dev, err = l.SearchDrivePath(drive)
	assert.Empty(t, dev)
	assert.NotNil(t, err)
}

func TestLinuxUtils_FindMnt(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		target      = "/some/path"
		cmd         = fmt.Sprintf(FindMntCmdTmpl, target)
		expectedRes = "/dev/mapper/lv-1"
		expectedErr = errors.New("error here")
		currentRes  string
		err         error
	)

	// success
	e.OnCommand(cmd).Return(expectedRes, "", nil).Times(1)
	currentRes, err = l.FindMnt(target)
	assert.Nil(t, err)
	assert.Equal(t, expectedRes, currentRes)

	// expect error
	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	currentRes, err = l.FindMnt(target)
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtils_GetPartitionNameByUUID(t *testing.T) {
	var (
		e        = &mocks.GoMockExecutor{}
		l        = NewLinuxUtils(e, luLogger)
		device   = "/dev/loop25"
		partName = "/dev/loop25p1"
		partUUID = "19ad3d36-38fa-4688-a57c-aa74961aa126"
		cmd      = fmt.Sprintf(lsblk.CmdTmpl, device)
		output   = "{\"blockdevices\": [{\"name\":\"/dev/loop25\",\"partuuid\":null,\"children\":[{\"name\":\"" +
			partName + "\",\"partuuid\":\"" + partUUID + "\"}]}]}"
	)

	e.OnCommand(cmd).Return(output, "", nil)
	name, err := l.GetPartitionNameByUUID(device, partUUID)
	assert.Equal(t, name, partName)
	assert.Nil(t, err)
}

func TestLinuxUtils_GetPartitionNameByUUIDNameNotPresent(t *testing.T) {
	var (
		e        = &mocks.GoMockExecutor{}
		l        = NewLinuxUtils(e, luLogger)
		device   = "/dev/loop25"
		partUUID = "19ad3d36-38fa-4688-a57c-aa74961aa126"
		cmd      = fmt.Sprintf(lsblk.CmdTmpl, device)
		output   = "{\"blockdevices\": [{\"name\":\"/dev/loop25\",\"partuuid\":null,\"children\":[{\"partuuid\":\"" +
			partUUID + "\"}]}]}"
	)

	e.OnCommand(cmd).Return(output, "", nil)
	name, err := l.GetPartitionNameByUUID(device, partUUID)
	assert.Empty(t, name)
	assert.NotNil(t, err)
}

func TestLinuxUtils_GetPartitionNameByUUIDNotFound(t *testing.T) {
	var (
		e         = &mocks.GoMockExecutor{}
		l         = NewLinuxUtils(e, luLogger)
		device    = "/dev/loop25"
		partName  = "/dev/loop25p1"
		partUUID  = "19ad3d36-38fa-4688-a57c-aa74961aa126"
		partUUID2 = "19ad3d36-38fa-4688-a57c-aa74961aa127"
		cmd       = fmt.Sprintf(lsblk.CmdTmpl, device)
		output    = "{\"blockdevices\": [{\"name\":\"/dev/loop25\",\"partuuid\":null,\"children\":[{\"name\":\"" +
			partName + "\",\"partuuid\":\"" + partUUID2 + "\"}]}]}"
	)

	e.OnCommand(cmd).Return(output, "", nil)
	name, err := l.GetPartitionNameByUUID(device, partUUID)
	assert.Empty(t, name)
	assert.NotNil(t, err)
}

func TestLinuxUtils_GetPartitionNameByUUIDFail(t *testing.T) {
	var (
		e        = &mocks.GoMockExecutor{}
		l        = NewLinuxUtils(e, luLogger)
		device   = "/dev/loop25"
		partUUID = "19ad3d36-38fa-4688-a57c-aa74961aa126"
		cmd      = fmt.Sprintf(lsblk.CmdTmpl, device)
	)

	name, err := l.GetPartitionNameByUUID("", partUUID)
	assert.Empty(t, name)
	assert.NotNil(t, err)

	name, err = l.GetPartitionNameByUUID(device, "")
	assert.Empty(t, name)
	assert.NotNil(t, err)

	name, err = l.GetPartitionNameByUUID("", "")
	assert.Empty(t, name)
	assert.NotNil(t, err)

	e.OnCommand(cmd).Return("", "", errors.New("error"))
	name, err = l.GetPartitionNameByUUID(device, partUUID)
	assert.Empty(t, name)
	assert.NotNil(t, err)
}
