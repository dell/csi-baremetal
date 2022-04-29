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

package utilwrappers

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
)

func TestFSOperationsImpl_PrepareAndPerformMount_Success(t *testing.T) {
	var (
		fsOps      = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS     = &mocklu.MockWrapFS{}
		dst        = "~/some/unusual/name"
		src        = "/tmp"
		bindOption = []string{"", ""} // for bind == false
		err        error
	)
	fsOps.WrapFS = wrapFS

	// dst folder isn't exist
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, bindOption).Return(nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, false, true)
	assert.Nil(t, err)
	wrapFS.AssertCalled(t, "MkDir", dst) // ensure that folder was created

	// dst folder is exist and has already mounted
	dst = "/tmp"
	wrapFS.On("IsMounted", dst).Return(true, nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, false, true)

	// dst folder is exist and isn't a mount point, also use bind = true
	wrapFS.On("IsMounted", dst).Return(false, nil).Once()
	wrapFS.On("Mount", src, dst, []string{fs.BindOption, ""}).Return(nil).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, true, true)
	wrapFS.AssertCalled(t, "IsMounted", dst)
}

func TestFSOperationsImpl_PrepareAndPerformMount_Fail(t *testing.T) {
	var (
		fsOps       = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS      = &mocklu.MockWrapFS{}
		dst         = "~/some/unusual/name"
		src         = "/tmp"
		bindOption  = []string{"", ""} // for bind == false and empty mount opts
		expectedErr = errors.New("error")
		err         error
	)
	fsOps.WrapFS = wrapFS

	// dst ins't exist and MkDir failed
	wrapFS.On("MkDir", dst).Return(expectedErr).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false, true)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)

	// dst is exist, IsMounted failed
	dst = "/tmp"
	wrapFS.On("IsMounted", dst).Return(false, expectedErr).Once()
	wrapFS.On("RmDir", dst).Return(nil).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false, true)

	assert.Error(t, err)
	wrapFS.AssertCalled(t, "RmDir", dst)

	// mount operations failed and dst was created during current call (expect RmDir)
	dst = "/some/not-existed/path"
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, bindOption).Return(expectedErr).Once()
	wrapFS.On("RmDir", dst).Return(nil).Once()
	wrapFS.On("IsMounted", src).Return(false, nil).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false, true)
	assert.Error(t, err)
	wrapFS.AssertCalled(t, "MkDir", dst)
	wrapFS.AssertCalled(t, "RmDir", dst)

	// mount operations failed and dst wasn't created during current call (do not expect RmDir)
	dst = "/var" // existed path, different from such that used before - /tmp, (for check AssertNotCalled)
	wrapFS.On("IsMounted", dst).Return(false, nil).Once()
	wrapFS.On("IsMounted", src).Return(false, nil).Once()
	wrapFS.On("Mount", src, dst, bindOption).Return(expectedErr).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false, true)
	assert.Error(t, err)
	wrapFS.AssertCalled(t, "IsMounted", dst)
	wrapFS.AssertNotCalled(t, "RmDir", dst)
}

func TestFSOperationsImpl_PrepareAndPerformMount_Success_WithMountOptions(t *testing.T) {
	var (
		fsOps         = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS        = &mocklu.MockWrapFS{}
		dst           = "~/some/unusual/name"
		src           = "/tmp"
		mountOptions1 = []string{"noatime"}
		mountOptions2 = []string{"noatime", "nodev"}
		cmdOptions1   = []string{"", "-o noatime"}             // bind == false && 1 mountOpt
		cmdOptions2   = []string{"", "-o noatime,nodev"}       // bind == false && 2 mountOpt
		cmdOptions3   = []string{"--bind", "-o noatime,nodev"} // bind == true && 2 mountOpt
		cmdOptions4   = []string{"--bind", ""}                 // bind == true && 0 mountOpt
		cmdOptions5   = []string{"", ""}                       // bind == false && 0 mountOpt
		err           error
	)

	fsOps.WrapFS = wrapFS
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, cmdOptions1).Return(nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, false, true, mountOptions1...)
	assert.Nil(t, err)

	fsOps.WrapFS = wrapFS
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, cmdOptions2).Return(nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, false, true, mountOptions2...)
	assert.Nil(t, err)

	fsOps.WrapFS = wrapFS
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, cmdOptions3).Return(nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, true, true, mountOptions2...)
	assert.Nil(t, err)

	fsOps.WrapFS = wrapFS
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, cmdOptions4).Return(nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, true, true)
	assert.Nil(t, err)

	fsOps.WrapFS = wrapFS
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, cmdOptions5).Return(nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, false, true)
	assert.Nil(t, err)
}

func TestFSOperationsImpl_MountWithCheck_Success(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		err    error
	)
	fsOps.WrapFS = wrapFS

	// not mounted
	wrapFS.On("IsMounted", path).Return(false, nil).Once()
	err = fsOps.UnmountWithCheck(path)
	assert.Nil(t, err)
	for _, c := range wrapFS.Calls {
		if c.Method == "Unmount" {
			t.Error("Method Unmount shouldn't have been called")
		}
	}

	// Unmount successfully
	wrapFS.On("IsMounted", path).Return(true, nil).Once()
	wrapFS.On("Unmount", path).Return(nil).Once()
	err = fsOps.UnmountWithCheck(path)
	assert.Nil(t, err)
	unmountCalled := false
	for _, c := range wrapFS.Calls {
		if c.Method == "Unmount" {
			unmountCalled = true
			break
		}
	}
	assert.True(t, unmountCalled)
}

func TestFSOperationsImpl_MountWithCheck_Fail(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		err    error
	)
	fsOps.WrapFS = wrapFS

	// IsMounted failed
	isMountedErr := errors.New("isMounted failed")
	wrapFS.On("IsMounted", path).Return(false, isMountedErr).Once()
	err = fsOps.UnmountWithCheck(path)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), isMountedErr.Error())

	// Unmount failed
	unmountErr := errors.New("unmount failed")
	wrapFS.On("IsMounted", path).Return(true, nil).Once()
	wrapFS.On("Unmount", path).Return(unmountErr).Once()
	err = fsOps.UnmountWithCheck(path)
	assert.NotNil(t, err)
	assert.Equal(t, unmountErr, err)
}

func TestFSOperationsImpl_CreateFSIfNotExist_NoFS(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		uuid   = "test-uuid"
		fsType = "xfs"
		err    error
	)
	fsOps.WrapFS = wrapFS

	wrapFS.On("GetFSType", path).Return("", nil).Once()
	wrapFS.On("GetFSUUID", path).Return("", nil).Once()
	wrapFS.On("CreateFS", fs.FileSystem(fsType), path, uuid).Return(nil).Once()

	err = fsOps.CreateFSIfNotExist(fs.FileSystem(fsType), path, uuid)
	assert.Nil(t, err)
}

func TestFSOperationsImpl_CreateFSIfNotExist_FSExists(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		fsType = "xfs"
		uuid   = "test-uuid"
		err    error
	)
	fsOps.WrapFS = wrapFS

	wrapFS.On("GetFSType", path).Return(fsType, nil).Once()
	wrapFS.On("GetFSUUID", path).Return(uuid, nil).Once()

	err = fsOps.CreateFSIfNotExist(fs.FileSystem(fsType), path, uuid)
	assert.Nil(t, err)
}

func TestFSOperationsImpl_CreateFSIfNotExist_OtherFS(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		uuid   = "test-uuid"
		fsType = "xfs"
		err    error
	)
	fsOps.WrapFS = wrapFS

	wrapFS.On("GetFSType", path).Return("other_FS", nil).Once()
	wrapFS.On("GetFSUUID", path).Return(uuid, nil).Once()

	err = fsOps.CreateFSIfNotExist(fs.FileSystem(fsType), path, uuid)
	assert.NotNil(t, err)
}

func TestFSOperationsImpl_CreateFSIfNotExist_OtherUUID(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		uuid   = "test-uuid"
		fsType = "xfs"
		err    error
	)
	fsOps.WrapFS = wrapFS

	wrapFS.On("GetFSType", path).Return(fsType, nil).Once()
	wrapFS.On("GetFSUUID", path).Return("other_UUID", nil).Once()

	err = fsOps.CreateFSIfNotExist(fs.FileSystem(fsType), path, uuid)
	assert.NotNil(t, err)
}

func TestFSOperationsImpl_CreateFSIfNotExist_CheckError(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		uuid   = "test-uuid"
		fsType = "xfs"
		err    error
	)
	fsOps.WrapFS = wrapFS

	wrapFS.On("GetFSType", path).Return("", errors.New("some_error")).Once()

	err = fsOps.CreateFSIfNotExist(fs.FileSystem(fsType), path, uuid)
	assert.NotNil(t, err)
}

func TestFSOperationsImpl_CreateFSIfNotExist_CheckUUIDError(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		uuid   = "test-uuid"
		fsType = "xfs"
		err    error
	)
	fsOps.WrapFS = wrapFS

	wrapFS.On("GetFSType", path).Return("", nil).Once()
	wrapFS.On("GetFSUUID", path).Return("", errors.New("some_error")).Once()

	err = fsOps.CreateFSIfNotExist(fs.FileSystem(fsType), path, uuid)
	assert.NotNil(t, err)
}

func TestFSOperationsImpl_CreateFSIfNotExist_CreateError(t *testing.T) {
	var (
		fsOps  = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS = &mocklu.MockWrapFS{}
		path   = "/some/path"
		uuid   = "test-uuid"
		fsType = "xfs"
		err    error
	)
	fsOps.WrapFS = wrapFS

	wrapFS.On("GetFSType", path).Return("", nil).Once()
	wrapFS.On("GetFSUUID", path).Return("", nil).Once()
	wrapFS.On("CreateFS", fs.FileSystem(fsType), path, uuid).Return(errors.New("some_error")).Once()

	err = fsOps.CreateFSIfNotExist(fs.FileSystem(fsType), path, uuid)
	assert.NotNil(t, err)
}
