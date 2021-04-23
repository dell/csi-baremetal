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

package fs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

var (
	testError = errors.New("error")
)

func TestFindMountPoint(t *testing.T) {
	var (
		e           = &mocks.GoMockExecutor{}
		fh          = NewFSImpl(e)
		target      = "/some/path"
		cmd         = fmt.Sprintf(FindMntCmdTmpl, target)
		expectedRes = "/dev/mapper/lv-1"
		expectedErr = errors.New("error here")
		currentRes  string
		err         error
	)

	// success
	e.OnCommand(cmd).Return(expectedRes, "", nil).Times(1)
	currentRes, err = fh.FindMountPoint(target)
	assert.Nil(t, err)
	assert.Equal(t, expectedRes, currentRes)

	// expect error
	e.OnCommand(cmd).Return("", "", expectedErr).Times(1)
	currentRes, err = fh.FindMountPoint(target)
	assert.Equal(t, expectedErr, err)
}

func TestGetFSSpace_Fail(t *testing.T) {
	var (
		mockexec = &mocks.GoMockExecutor{}
		fh       = NewFSImpl(mockexec)
		path     = "/"
		cmd      = fmt.Sprintf(CheckSpaceCmdImpl, path)
	)

	// wrong df output
	mockexec.On("RunCmd", cmd).
		Return("dadasda", "", nil).Times(1)
	freeBytes, err := fh.GetFSSpace("/")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "wrong df output")
	assert.Equal(t, freeBytes, int64(0))

	// fail to parse output
	mockexec.On("RunCmd", cmd).
		Return("Mounted on Avail\n/   10MM", "", nil).Times(1)
	freeBytes, err = fh.GetFSSpace(path)
	assert.NotNil(t, err)
	assert.Equal(t, freeBytes, int64(0))

	// command error
	mockexec.On("RunCmd", cmd).
		Return("/   10MM", "", fmt.Errorf("error")).Times(1)
	freeBytes, err = fh.GetFSSpace("/")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error")
	assert.Equal(t, freeBytes, int64(0))
}

func TestGetFSSpace_Success(t *testing.T) {
	var (
		mockexec  = &mocks.GoMockExecutor{}
		fh        = NewFSImpl(mockexec)
		path      = "/"
		sizeStr   = "1000M"
		cmd       = fmt.Sprintf(CheckSpaceCmdImpl, path)
		cmdResult = fmt.Sprintf("Mounted on Avail\n%s   %s", path, sizeStr)
	)

	mockexec.On("RunCmd", cmd).
		Return(cmdResult, "", nil)
	freeBytes, err := fh.GetFSSpace(path)
	assert.Nil(t, err)
	expectedRes, err := util.StrToBytes(sizeStr)
	assert.Nil(t, err)
	assert.Equal(t, expectedRes, freeBytes)
}

func TestMkDir(t *testing.T) {
	var (
		e   = &mocks.GoMockExecutor{}
		fh  = NewFSImpl(e)
		src = "/dev/mnt"
		cmd = fmt.Sprintf(MkDirCmdTmpl, src)
		err error
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = fh.MkDir(src)
	assert.Nil(t, err)

	// cmd failed
	e.OnCommand(cmd).Return("", "", testError).Times(1)
	err = fh.MkDir(src)
	assert.NotNil(t, err)
}

func TestRmDir(t *testing.T) {
	var (
		e   = &mocks.GoMockExecutor{}
		fh  = NewFSImpl(e)
		src = "/dev/mnt"
		cmd = fmt.Sprintf(RmDirCmdTmpl, src)
		err error
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = fh.RmDir(src)
	assert.Nil(t, err)

	// cmd failed
	e.OnCommand(cmd).Return("", "", testError).Times(1)
	err = fh.RmDir(src)
	assert.NotNil(t, err)
}

func TestCreateFS(t *testing.T) {
	var (
		e      = &mocks.GoMockExecutor{}
		fh     = NewFSImpl(e)
		device = "/dev/sda1"
		fsType = XFS
		cmd    = fmt.Sprintf(MkFSCmdTmpl, fsType, device)
		err    error
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = fh.CreateFS(fsType, device)
	assert.Nil(t, err)

	// cmd failed
	e.OnCommand(cmd).Return("", "", testError).Times(1)
	err = fh.CreateFS(fsType, device)
	assert.NotNil(t, err)

	// unsupported FS
	err = fh.CreateFS("anotherFS", device)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unsupported file system")
}

func TestWipeFS(t *testing.T) {
	var (
		e      = &mocks.GoMockExecutor{}
		fh     = NewFSImpl(e)
		device = "/dev/sda"
		cmd    = fmt.Sprintf(WipeFSCmdTmpl, device)
		err    error
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = fh.WipeFS(device)
	assert.Nil(t, err)

	// cmd failed
	e.OnCommand(cmd).Return("", "", testError).Times(1)
	err = fh.WipeFS(device)
	assert.NotNil(t, err)
}

func TestGetFSType(t *testing.T) {
	var (
		e          = &mocks.GoMockExecutor{}
		fh         = NewFSImpl(e)
		device     = "/dev/sda"
		cmd        = fmt.Sprintf(GetFSTypeCmdTmpl, device)
		expectedFS = "xfs"
		currentFS  FileSystem
		err        error
	)

	e.OnCommand(cmd).Return(expectedFS, "", nil).Times(1)
	currentFS, err = fh.GetFSType(device)
	assert.Nil(t, err)
	assert.Equal(t, FileSystem(expectedFS), currentFS)

	// cmd failed
	e.OnCommand(cmd).Return("", "", testError).Times(1)
	_, err = fh.GetFSType(device)
	assert.NotNil(t, err)
}

func TestMount(t *testing.T) {
	var (
		e   = &mocks.GoMockExecutor{}
		fh  = NewFSImpl(e)
		src = "/dev/sda1"
		dst = "/mnt/pod1"
		cmd = fmt.Sprintf(MountCmdTmpl, "", src, dst)
		err error
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = fh.Mount(src, dst)
	assert.Nil(t, err)

	// cmd failed
	e.OnCommand(cmd).Return("", "", testError).Times(1)
	err = fh.Mount(src, dst)
	assert.NotNil(t, err)
}

func TestUnmount(t *testing.T) {
	var (
		e    = &mocks.GoMockExecutor{}
		fh   = NewFSImpl(e)
		path = "/mnt/pod1"
		cmd  = fmt.Sprintf(UnmountCmdTmpl, path)
		err  error
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	err = fh.Unmount(path)
	assert.Nil(t, err)

	// cmd failed
	e.OnCommand(cmd).Return("", "", testError).Times(1)
	err = fh.Unmount(path)
	assert.NotNil(t, err)
}

func Test_DriveHasData(t *testing.T) {
	var (
		e    = &mocks.GoMockExecutor{}
		fh   = NewFSImpl(e)
		path = "/dev/sda"
		cmd  = fmt.Sprintf(DetectFSCmdTmpl, path)
	)

	e.OnCommand(cmd).Return("", "", nil).Times(1)
	hasData, err := fh.DriveHasData(path)
	assert.Nil(t, err)
	assert.False(t, hasData)

	e.OnCommand(cmd).Return("xfs", "", testError).Times(1)
	hasData, err = fh.DriveHasData(path)
	assert.NotNil(t, err)
	assert.False(t, hasData)

	e.OnCommand(cmd).Return("xfs", "", nil).Times(1)
	hasData, err = fh.DriveHasData(path)
	assert.Nil(t, err)
	assert.True(t, hasData)
}
