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

	"github.com/dell/csi-baremetal/pkg/mocks"
)

var (
	testLogger = logrus.New()
)

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

func TestLinuxUtils_FindVgNameByLvName(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
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

func TestLinuxUtils_IsLVGExists(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		l           = NewLVM(e, testLogger)
		lvName      = "/dev/mapper/lv-1"
		cmd         = fmt.Sprintf(VGByLVCmdTmpl, lvName)
		expectedVG  = "vg-1"
		expectedErr = errors.New("error here")
		err         error
	)

	// expect success (tabs and new line were trim)
	e.OnCommand(cmd).Return(fmt.Sprintf("\t%s   \t\n", expectedVG), "", nil).Times(1)
	mp, err := l.IsLVGExists(lvName)
	assert.Nil(t, err)
	assert.Equal(t, true, mp)

	// expect error
	e.OnCommand(cmd).Return("root_vg", "", expectedErr).Times(1)
	mp, err = l.IsLVGExists(lvName)
	assert.Equal(t, false, mp)
	assert.Equal(t, expectedErr, err)

	// expect volume group node found
	e.OnCommand(cmd).Return("", "Volume group \"lv-1\" not found", nil).Times(1)
	mp, err = l.IsLVGExists(lvName)
	assert.Equal(t, false, mp)
	assert.Equal(t, nil, err)

	// expect unable to determine
	e.OnCommand(cmd).Return("", "", nil).Times(1)
	mp, err = l.IsLVGExists(lvName)
	assert.Equal(t, false, mp)
	assert.NotNil(t, err)

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
