package base

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoopBackManager_CheckRootFsSpaceFailWrongOutput(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	rf := NewRootFsUtils(mockexec)
	mockexec.On("RunCmd", fmt.Sprintf(CheckSpaceCmdImpl, "M")).
		Return("dadasda", "", nil)
	freeBytes, err := rf.CheckRootFsSpace()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "wrong df output")
	assert.Equal(t, freeBytes, int64(0))
}

func TestLoopBackManager_checkRootFsSpaceFailParse(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	rf := NewRootFsUtils(mockexec)
	mockexec.On("RunCmd", fmt.Sprintf(CheckSpaceCmdImpl, "M")).
		Return("Mounted on                       Avail\n/   10MM", "", nil)
	freeBytes, err := rf.CheckRootFsSpace()
	assert.NotNil(t, err)
	assert.Equal(t, freeBytes, int64(0))
}
func TestLoopBackManager_checkRootFsSpaceFailCommandError(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	rf := NewRootFsUtils(mockexec)
	mockexec.On("RunCmd", fmt.Sprintf(CheckSpaceCmdImpl, "M")).
		Return("/   10MM", "", fmt.Errorf("error"))
	freeBytes, err := rf.CheckRootFsSpace()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error")
	assert.Equal(t, freeBytes, int64(0))
}

func TestLoopBackManager_checkRootFsSpaceSuccessFloatSize(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	rf := NewRootFsUtils(mockexec)
	mockexec.On("RunCmd", fmt.Sprintf(CheckSpaceCmdImpl, "M")).
		Return("Mounted on                       Avail\n/   1000.11M", "", nil)
	freeBytes, err := rf.CheckRootFsSpace()
	assert.Nil(t, err)
	assert.Equal(t, freeBytes, int64(1048691343))
}
func TestLoopBackManager_checkRootFsSpaceSuccess(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	rf := NewRootFsUtils(mockexec)
	mockexec.On("RunCmd", fmt.Sprintf(CheckSpaceCmdImpl, "M")).
		Return("Mounted on                       Avail\n/   1000M", "", nil)
	freeBytes, err := rf.CheckRootFsSpace()
	assert.Nil(t, err)
	assert.Equal(t, freeBytes, int64(1048576000))
}
func TestLoopBackManager_checkRootFsSpaceSuccessTwoString(t *testing.T) {
	var mockexec = &mocks.GoMockExecutor{}
	rf := NewRootFsUtils(mockexec)
	mockexec.On("RunCmd", fmt.Sprintf(CheckSpaceCmdImpl, "M")).
		Return("Mounted on                       Avail\n/run   437M\n/   1000M", "", nil)
	freeBytes, err := rf.CheckRootFsSpace()
	assert.Nil(t, err)
	assert.Equal(t, freeBytes, int64(1048576000))
}
