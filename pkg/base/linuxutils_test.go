package base

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var (
	luLogger           = logrus.New()
	lsblkAllDevicesCmd = fmt.Sprintf(LsblkCmdTmpl, "")
)

func TestLinuxUtils_SetLinuxeUtilsExecutor(t *testing.T) {
	e1 := new(Executor)
	e1.SetLogger(luLogger)
	e2 := new(Executor)
	e2.SetLogger(logrus.New())

	l := NewLinuxUtils(e1, luLogger)
	assert.Equal(t, l.e, e1)
	l.SetExecutor(e2)
	assert.Equal(t, l.e, e2)
}

func TestLinuxUtils_LsblkSuccess(t *testing.T) {

	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", lsblkAllDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	l := NewLinuxUtils(e, luLogger)

	out, err := l.Lsblk("")
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, 2, len(out))

}

func TestLinuxUtils_LsblkFail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLinuxUtils(e, luLogger)

	e.On(mocks.RunCmd, lsblkAllDevicesCmd).Return("not a json", "", nil).Times(1)
	out, err := l.Lsblk("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to unmarshal output to LsblkOutput instance")

	expectedError := errors.New("lsblk failed")
	e.On(mocks.RunCmd, lsblkAllDevicesCmd).Return("", "", expectedError).Times(1)
	out, err = l.Lsblk("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Equal(t, expectedError, err)

	e.On(mocks.RunCmd, lsblkAllDevicesCmd).Return(mocks.NoLsblkKeyStr, "", nil).Times(1)
	out, err = l.Lsblk("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unexpected lsblk output format")
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

func TestLinuxUtils_PVCreate(t *testing.T) {
	var (
		e   = &mocks.GoMockExecutor{}
		l   = NewLinuxUtils(e, luLogger)
		dev = "/dev/sda"
		cmd = fmt.Sprintf(PVCreateCmdTmpl, dev)
		err error
	)
	e.OnCommand(cmd).Return("", "", nil)
	err = l.PVCreate(dev)
	assert.Nil(t, err)
}

func TestLinuxUtils_PVRemove(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		dev         = "/dev/sda"
		cmd         = fmt.Sprintf(PVRemoveCmdTmpl, dev)
		err         error
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = l.PVRemove(dev)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "No PV label found on /dev/sda", expectedErr).Times(1)
	err = l.PVRemove(dev)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "some another error", expectedErr).Times(1)
	err = l.PVRemove(dev)
	assert.NotNil(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtils_VGCreate(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		vg          = "test-lvg"
		dev1        = "/dev/sda"
		dev2        = "/dev/sdb"
		cmd         = fmt.Sprintf(VGCreateCmdTmpl, vg, strings.Join([]string{dev1, dev2}, " "))
		err         error
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = l.VGCreate(vg, dev1, dev2)
	assert.Nil(t, err)

	e.OnCommand(cmd).
		Return("", "already exists", expectedErr).
		Times(1)
	err = l.VGCreate(vg, dev1, dev2)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	err = l.VGCreate(vg, dev1, dev2)
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtils_VGRemove(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		vg          = "test-lvg"
		cmd         = fmt.Sprintf(VGRemoveCmdTmpl, vg)
		err         error
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = l.VGRemove(vg)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "not found", expectedErr).Times(1)
	err = l.VGRemove(vg)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	err = l.VGRemove(vg)
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtils_LVCreate(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		lv          = "test-lv"
		size        = "9g"
		vg          = "test-lvg"
		cmd         = fmt.Sprintf(LVCreateCmdTmpl, lv, size, vg)
		err         error
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = l.LVCreate(lv, size, vg)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "already exists", expectedErr).Times(1)
	err = l.LVCreate(lv, size, vg)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	err = l.LVCreate(lv, size, vg)
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtils_LVRemove(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		lv          = "test-lv"
		vg          = "test-lvg"
		fullLVName  = fmt.Sprintf("/dev/%s/%s", vg, lv)
		cmd         = fmt.Sprintf(LVRemoveCmdTmpl, fullLVName)
		err         error
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = l.LVRemove(fullLVName)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "Failed to find logical volume", expectedErr).Times(1)
	err = l.LVRemove(fullLVName)
	assert.Nil(t, err)

	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	err = l.LVRemove(fullLVName)
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtilsIs_VGContainsLVs(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		vg          = "test-lvg"
		cmd         = fmt.Sprintf(LVsInVGCmdTmpl, vg)
		res         bool
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("\n", "", nil).Times(1)
	res = l.IsVGContainsLVs(vg)
	assert.False(t, res)

	e.OnCommand(cmd).Return("asdf\nadf", "", nil).Times(1)
	res = l.IsVGContainsLVs(vg)
	assert.True(t, res)

	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	res = l.IsVGContainsLVs(vg)
	assert.True(t, res)
}

func TestLinuxUtils_RemoveOrphanPVs(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		dev1        = "/dev/sda"
		cmd         = fmt.Sprintf(PVsInVGCmdTmpl, EmptyName)
		err         error
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("\n", "", nil).Times(1)
	err = l.RemoveOrphanPVs()
	assert.Nil(t, err)

	e.OnCommand(cmd).Return(dev1, "", nil).Times(1)
	e.OnCommand(fmt.Sprintf(PVRemoveCmdTmpl, dev1)).
		Return("", "", nil).Times(1)
	err = l.RemoveOrphanPVs()
	assert.Nil(t, err)

	e.OnCommand(cmd).Return(dev1, "", nil).Times(1)
	e.OnCommand(fmt.Sprintf(PVRemoveCmdTmpl, dev1)).
		Return("", "", expectedErr).Times(1)
	err = l.RemoveOrphanPVs()
	assert.Equal(t, errors.New("not all PVs were removed"), err)

	e.OnCommand(cmd).Return(dev1, "", expectedErr).Times(1)
	err = l.RemoveOrphanPVs()
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtils_FindVgNameByLvName(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLinuxUtils(e, luLogger)
		lvName      = "/dev/mapper/lv-1"
		cmd         = fmt.Sprintf(VGByLVCmdTmpl, lvName)
		expectedVG  = "vg-1"
		expectedErr = errors.New("error here")
		currentVG   string
		err         error
	)

	// expect success (tabs and new line were trim)
	e.OnCommand(cmd).Return(fmt.Sprintf("\t%s   \t\n", expectedVG), "", nil).Times(1)
	currentVG, err = l.FindVgNameByLvName(lvName)
	assert.Nil(t, err)
	assert.Equal(t, expectedVG, currentVG)

	// expect error
	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	currentVG, err = l.FindVgNameByLvName(lvName)
	assert.Equal(t, "", currentVG)
	assert.Equal(t, expectedErr, err)

}

func TestLinuxUtils_GetVgFreeSpace(t *testing.T) {
	var (
		e            = &mocks.GoMockExecutor{}
		l            = NewLinuxUtils(e, luLogger)
		vgName       = "vg-1"
		cmd          = fmt.Sprintf(VGFreeSpaceCmdTmpl, vgName)
		expectedSize = int64(1000)
		expectedErr  = errors.New("error here")
		currentSize  int64
		err          error
	)

	// expected success (tabs and new line were trim)
	e.OnCommand(cmd).Return(fmt.Sprintf("\t\t %dB \n", expectedSize), "", nil).Times(1)
	currentSize, err = l.GetVgFreeSpace(vgName)
	assert.Nil(t, err)
	assert.Equal(t, expectedSize, currentSize)

	// expected error in cmd
	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	currentSize, err = l.GetVgFreeSpace(vgName)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, int64(-1), currentSize)

	// empty string, expected err
	currentSize, err = l.GetVgFreeSpace("")
	assert.Equal(t, int64(-1), currentSize)
	assert.Equal(t, errors.New("VG name shouldn't be an empty string"), err)

	// empty string, unable to convert to int
	e.OnCommand(cmd).Return(fmt.Sprintf("\t\t %d \n", expectedSize), "", nil).Times(1)
	currentSize, err = l.GetVgFreeSpace(vgName)
	assert.Equal(t, int64(-1), currentSize)
	assert.Contains(t, err.Error(), "unknown size unit")
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
