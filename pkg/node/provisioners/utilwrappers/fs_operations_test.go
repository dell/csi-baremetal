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
		bindOption = []string{""} // for bind == false
		err        error
	)
	fsOps.WrapFS = wrapFS

	// dst folder isn't exist
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, bindOption).Return(nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, false)
	assert.Nil(t, err)
	wrapFS.AssertCalled(t, "MkDir", dst) // ensure that folder was created

	// dst folder is exist and has already mounted
	dst = "/tmp"
	wrapFS.On("IsMounted", dst).Return(true, nil).Once()
	err = fsOps.PrepareAndPerformMount(src, dst, false)

	// dst folder is exist and isn't a mount point, also use bind = true
	wrapFS.On("IsMounted", dst).Return(false, nil).Once()
	wrapFS.On("Mount", src, dst, []string{fs.BindOption}).Return(nil).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, true)
	wrapFS.AssertCalled(t, "IsMounted", dst)
}

func TestFSOperationsImpl_PrepareAndPerformMount_Fail(t *testing.T) {
	var (
		fsOps       = NewFSOperationsImpl(&command.Executor{}, logrus.New())
		wrapFS      = &mocklu.MockWrapFS{}
		dst         = "~/some/unusual/name"
		src         = "/tmp"
		bindOption  = []string{""} // for bind == false
		expectedErr = errors.New("error")
		err         error
	)
	fsOps.WrapFS = wrapFS

	// dst ins't exist and MkDir failed
	wrapFS.On("MkDir", dst).Return(expectedErr).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)

	// dst is exist, IsMounted failed
	dst = "/tmp"
	wrapFS.On("IsMounted", dst).Return(false, expectedErr).Once()
	wrapFS.On("RmDir", dst).Return(nil).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false)

	assert.Error(t, err)
	wrapFS.AssertCalled(t, "RmDir", dst)

	// mount operations failed and dst was created during current call (expect RmDir)
	dst = "/some/not-existed/path"
	wrapFS.On("MkDir", dst).Return(nil).Once()
	wrapFS.On("Mount", src, dst, bindOption).Return(expectedErr).Once()
	wrapFS.On("RmDir", dst).Return(nil).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false)
	assert.Error(t, err)
	wrapFS.AssertCalled(t, "MkDir", dst)
	wrapFS.AssertCalled(t, "RmDir", dst)

	// mount operations failed and dst wasn't created during current call (do not expect RmDir)
	dst = "/var" // existed path, different from such that used before - /tmp, (for check AssertNotCalled)
	wrapFS.On("IsMounted", dst).Return(false, nil).Once()
	wrapFS.On("Mount", src, dst, bindOption).Return(expectedErr).Once()

	err = fsOps.PrepareAndPerformMount(src, dst, false)
	assert.Error(t, err)
	wrapFS.AssertCalled(t, "IsMounted", dst)
	wrapFS.AssertNotCalled(t, "RmDir", dst)
}
