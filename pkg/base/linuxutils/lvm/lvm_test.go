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

package lvm

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/base/baseerr"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

var testLogger = logrus.New()

func TestLinuxUtils_PVCreate(t *testing.T) {
	var (
		e   = &mocks.GoMockExecutor{}
		l   = NewLVM(e, testLogger)
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
		l           = NewLVM(e, testLogger)
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
		l           = NewLVM(e, testLogger)
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

func TestLinuxUtils_VGScan(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
		vg          = "test-vg"
		cmd         = VGScanCmdTmpl
		ok          bool
		err         error
		expectedErr = errors.New("error")
	)

	// error not found
	e.OnCommand(cmd).Return("", "", nil).Times(1)
	ok, err = l.VGScan(vg)
	assert.Equal(t, err, baseerr.ErrorNotFound)
	assert.False(t, ok)

	// error - expected
	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	ok, err = l.VGScan(vg)
	assert.False(t, ok)
	assert.Equal(t, err, expectedErr)

	// IO error detected
	e.OnCommand(cmd).Return("Found volume group \""+vg+"\" using metadata type lvm2",
		"/dev/"+vg+"/test-lv: Input/output error", nil).Times(1)
	ok, err = l.VGScan(vg)
	assert.True(t, ok)
	assert.Nil(t, err)

	// IO error not detected - multiple lines
	e.OnCommand(cmd).Return("Found volume group \""+vg+"\" using metadata type lvm2",
		"/dev/%s/test-lv: no errors\n/dev/other-vg/test-lv: Input/output error", nil).Times(1)
	ok, err = l.VGScan(vg)
	assert.False(t, ok)
	assert.Nil(t, err)

	// IO error detected - multiple lines
	e.OnCommand(cmd).Return("Found volume group \""+vg+"\" using metadata type lvm2",
		"/dev/"+vg+"/test-lv: no errors\n/dev/"+vg+"/test-lv-2: Input/output error", nil).Times(1)
	ok, err = l.VGScan(vg)
	assert.True(t, ok)
	assert.Nil(t, err)

	// error - wrong volume group name
	incorrectName := "*"
	e.OnCommand(cmd).Return(incorrectName, "", nil).Times(1)
	ok, err = l.VGScan(incorrectName)
	assert.False(t, ok)
	assert.NotNil(t, err)
}

func TestLinuxUtils_VGRemove(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
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
		l           = NewLVM(e, testLogger)
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
		l           = NewLVM(e, testLogger)
		lv          = "test-lv"
		vg          = "test-lvg"
		fullLVName  = fmt.Sprintf("/dev/%s/%s", vg, lv)
		cmd         = fmt.Sprintf(LVRemoveCmdTmpl, fullLVName)
		err         error
		expectedErr = errors.New("error")
	)

	e.OnCommandWithAttempts(cmd, 5, timeoutBetweenAttempts).Return("", "", nil).Times(1)
	err = l.LVRemove(fullLVName)
	assert.Nil(t, err)
	e.OnCommandWithAttempts(cmd, 5, timeoutBetweenAttempts).Return("", "Failed to find logical volume", expectedErr).Times(1)
	err = l.LVRemove(fullLVName)
	assert.Nil(t, err)

	e.OnCommandWithAttempts(cmd, 5, timeoutBetweenAttempts).Return("", "", expectedErr).Times(1)
	err = l.LVRemove(fullLVName)
	assert.Equal(t, expectedErr, err)
}

func TestLinuxUtilsIs_VGContainsLVs(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
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

func TestLinuxUtils_GetLVsInVG(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
		vg          = "test-lvg"
		cmd         = fmt.Sprintf(LVsInVGCmdTmpl, vg)
		expectedErr = errors.New("error")
	)

	e.OnCommand(cmd).Return("  asdf\n  adf", "", nil).Times(1)
	res, err := l.GetLVsInVG(vg)
	assert.Nil(t, err)
	assert.Equal(t, len(res), 2)
	assert.Equal(t, res[0], "asdf")
	assert.Equal(t, res[1], "adf")

	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	res, err = l.GetLVsInVG(vg)
	assert.NotNil(t, err)
	assert.Empty(t, res)
}

func TestLinuxUtils_RemoveOrphanPVs(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
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

func TestLinuxUtils_GetVgFreeSpace(t *testing.T) {
	var (
		e            = &mocks.GoMockExecutor{}
		l            = NewLVM(e, testLogger)
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

func TestLinuxUtils_GetAllPVs(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
		expectedErr = errors.New("error")
		res         []string
		err         error
	)

	t.Run("Happy pass", func(t *testing.T) {
		e.OnCommand(AllPVsCmd).Return("  /dev/sda\n  ", "", nil).Once()
		res, err = l.GetAllPVs()
		assert.Nil(t, err)
		assert.Equal(t, len(res), 1)
		assert.Equal(t, res[0], "/dev/sda")
	})

	t.Run("Cmd finished with error", func(t *testing.T) {
		e.OnCommand(AllPVsCmd).Return("", "", expectedErr).Once()
		res, err = l.GetAllPVs()
		assert.NotNil(t, err)
		assert.Empty(t, res)
	})
}

func TestLinuxUtils_GetVGNameByPVName(t *testing.T) {
	var (
		e              = &mocks.GoMockExecutor{}
		l              = NewLVM(e, testLogger)
		pvName         = "/dev/sda2"
		cmd            = fmt.Sprintf(PVInfoCmdTmpl, pvName)
		expectedVGName = "root-vg"
		expectedErr    = errors.New("error")
		res            string
		err            error
	)

	t.Run("Happy pass", func(t *testing.T) {
		e.OnCommand(cmd).Return(fmt.Sprintf("%s:%s:another:info", pvName, expectedVGName), "", nil).Once()
		res, err = l.GetVGNameByPVName(pvName)
		assert.Nil(t, err)
		assert.Equal(t, expectedVGName, res)
	})

	t.Run("Cmd finished with error", func(t *testing.T) {
		e.OnCommand(cmd).Return("", "", expectedErr).Once()
		res, err = l.GetVGNameByPVName(pvName)
		assert.Equal(t, "", res)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("PV isn't related to any VG", func(t *testing.T) {
		e.OnCommand(cmd).Return("/dev/sda is a new physical volume\nsome::another:info", "", nil).Once()
		res, err = l.GetVGNameByPVName(pvName)
		assert.Equal(t, "", res)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "isn't related to any VG")
	})

	t.Run("Unable to parse output", func(t *testing.T) {
		e.OnCommand(cmd).Return("/dev/sda", "", nil).Once()
		res, err = l.GetVGNameByPVName(pvName)
		assert.Equal(t, "", res)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "unable to find VG name for PV")
	})
}
