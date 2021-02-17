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

package lsblk

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

var (
	testLogger    = logrus.New()
	allDevicesCmd = fmt.Sprintf(CmdTmpl, "")

	sn        = "sn-1111"
	testDrive = api.Drive{
		SerialNumber: sn,
	}

	testDriveCR = drivecrd.Drive{
		Spec: testDrive,
	}
)

func TestGetVersion(t *testing.T)  {
	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk from util-linux 2.34", "", nil)

	l := NewLSBLK(e)
	version := l.GetVersion()
	assert.Equal(t, version.major, 2)
	assert.Equal(t, version.minor, 34)
	assert.Equal(t, version.patch, 0)

	e = &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk from util-linux 2.31.1", "", nil)
	l = NewLSBLK(e)
	version = l.GetVersion()
	assert.Equal(t, version.major, 2)
	assert.Equal(t, version.minor, 31)
	assert.Equal(t, version.patch, 1)

	e = &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk: command not found", "", nil)
	l = NewLSBLK(e)
	assert.Nil(t, l)
}

func TestLSBLK_GetBlockDevices_Success(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk from util-linux 2.31.1", "", nil)
	l := NewLSBLK(e)
	e.On("RunCmd", allDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)

	out, err := l.GetBlockDevices("")
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, 2, len(out))

}

func TestLSBLK_GetBlockDevices_Fail(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	l := NewLSBLK(e)
	e.On(mocks.RunCmd, allDevicesCmd).Return("not a json", "", nil).Times(1)
	out, err := l.GetBlockDevices("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to unmarshal output to BlockDevice instance")

	expectedError := errors.New("lsblk failed")
	e.On(mocks.RunCmd, allDevicesCmd).Return("", "", expectedError).Times(1)
	out, err = l.GetBlockDevices("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Equal(t, expectedError, err)

	e.On(mocks.RunCmd, allDevicesCmd).Return(mocks.NoLsblkKeyStr, "", nil).Times(1)
	out, err = l.GetBlockDevices("")
	assert.Nil(t, out)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unexpected lsblk output format")
}

func TestLSBLK_SearchDrivePath_Success(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk from util-linux 2.31.1", "", nil)
	l := NewLSBLK(e)
	// path is in drive spec
	dCR := testDriveCR
	path := "/dev/sda"
	dCR.Spec.Path = path

	res, err := l.SearchDrivePath(&dCR)
	assert.Nil(t, err)
	assert.Equal(t, path, res)

	// got from lsblk output
	e.On("RunCmd", allDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	sn = "hdd1"                  // from mocks.LsblkTwoDevicesStr
	expectedDevice := "/dev/sda" // from mocks.LsblkTwoDevicesStr
	d2CR := testDriveCR
	d2CR.Spec.SerialNumber = sn

	res, err = l.SearchDrivePath(&d2CR)
	assert.Nil(t, err)
	assert.Equal(t, expectedDevice, res)
}

func TestLSBLK_SearchDrivePath(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk from util-linux 2.31.1", "", nil)
	l := NewLSBLK(e)
	// lsblk fail
	expectedErr := errors.New("lsblk error")
	e.On("RunCmd", allDevicesCmd).Return("", "", expectedErr)
	res, err := l.SearchDrivePath(&testDriveCR)
	assert.Equal(t, "", res)
	assert.Equal(t, expectedErr, err)

	// sn isn't presented in lsblk output
	e.On("RunCmd", allDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	sn := "sn-that-isnt-existed"
	dCR := testDriveCR
	dCR.Spec.SerialNumber = sn

	res, err = l.SearchDrivePath(&dCR)
	assert.Equal(t, "", res)
	assert.NotNil(t, err)

	//different VID and PID
	e.On("RunCmd", allDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	sn = "hdd1" // from mocks.LsblkTwoDevicesStr
	dCR.Spec.SerialNumber = sn
	dCR.Spec.VID = "vendor"
	dCR.Spec.PID = "pid"

	res, err = l.SearchDrivePath(&dCR)
	assert.NotNil(t, err)
}

func TestLSBLK_GetBlockDevicesV2_Success(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk from util-linux 2.34", "", nil)
	e.On("RunCmd", allDevicesCmd).Return(mocks.LsblkDevV2, "", nil)
	l := NewLSBLK(e)

	out, err := l.GetBlockDevices("")
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, 2, len(out))
}

func TestLSBLK_GetAllV2_Success(t *testing.T) {
	e := &mocks.GoMockExecutor{}
	e.On("RunCmd", VersionCmdTmpl).Return("lsblk from util-linux 2.34", "", nil)
	e.On("RunCmd", allDevicesCmd).Return(mocks.LsblkAllV2, "", nil)
	l := NewLSBLK(e)

	out, err := l.GetBlockDevices("")
	assert.Nil(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, 2, len(out))
}
