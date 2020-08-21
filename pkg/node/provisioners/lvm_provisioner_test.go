package provisioners

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
	mockProv "github.com/dell/csi-baremetal/pkg/mocks/provisioners"
)

var (
	lp     *LVMProvisioner
	lvmOps *mocklu.MockWrapLVM
	fsOps  *mockProv.MockFsOpts
)

func setupTestLVMProvisioner() {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}

	lp = NewLVMProvisioner(&command.Executor{}, kubeClient, testLogger)
	lvmOps = &mocklu.MockWrapLVM{}
	fsOps = &mockProv.MockFsOpts{}

	lp.lvmOps = lvmOps
	lp.fsOps = fsOps
}

func TestLVMProvisioner_PrepareVolume_Success(t *testing.T) {
	setupTestLVMProvisioner()

	lvmOps.On("LVCreate", testVolume1.Id, mock.Anything, testVolume1.Location).
		Return(nil).Times(1)

	devFile := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	fsOps.On("CreateFS", fs.FileSystem(testVolume1.Type), devFile).
		Return(nil).Times(1)

	err := lp.PrepareVolume(testVolume1)
	assert.Nil(t, err)
}

func TestLVMProvisioner_PrepareVolume_Fail(t *testing.T) {
	setupTestLVMProvisioner()
	var err error

	// getVGName failed
	vol := testVolume1
	// in that case vgName will be searching in CRs and here we get error
	vol.StorageClass = apiV1.StorageClassSystemSSDLVG

	err = lp.PrepareVolume(vol)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to determine VG name")

	// LVCreate failed
	lvmOps.On("LVCreate", testVolume1.Id, mock.Anything, testVolume1.Location).
		Return(errTest).Times(1)

	err = lp.PrepareVolume(testVolume1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to create LV")

	// CreateFS failed
	lvmOps.On("LVCreate", testVolume1.Id, mock.Anything, testVolume1.Location).
		Return(nil).Times(1)

	devFile := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	fsOps.On("CreateFS", fs.FileSystem(testVolume1.Type), devFile).
		Return(errTest).Times(1)

	err = lp.PrepareVolume(testVolume1)
	assert.NotNil(t, err)
	assert.Equal(t, errTest, err)
}

func TestLVMProvisioner_ReleaseVolume_Success(t *testing.T) {
	setupTestLVMProvisioner()

	devFile := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	fsOps.On("WipeFS", devFile).Return(nil).Times(1)
	lvmOps.On("LVRemove", devFile).Return(nil).Times(1)

	err := lp.ReleaseVolume(testVolume1)
	assert.Nil(t, err)
}

func TestLVMProvisioner_ReleaseVolume_Fail(t *testing.T) {
	setupTestLVMProvisioner()

	var err error

	// getVGName failed
	vol := testVolume1
	// in that case vgName will be searching in CRs and here we get error
	vol.StorageClass = apiV1.StorageClassSystemSSDLVG

	err = lp.PrepareVolume(vol)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to determine VG name")

	// WipeFS failed
	devFile := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	fsOps.On("WipeFS", devFile).
		Return(errTest).Times(1)

	err = lp.ReleaseVolume(testVolume1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to wipe FS")

	// LVRemove failed
	fsOps.On("WipeFS", devFile).
		Return(nil).Times(1)
	lvmOps.On("LVRemove", devFile).
		Return(errTest).Times(1)

	err = lp.ReleaseVolume(testVolume1)
	assert.NotNil(t, err)
	assert.Equal(t, errTest, err)
}

func TestLVMProvisioner_GetVolumePath_Success(t *testing.T) {
	setupTestLVMProvisioner()

	expectedPath := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	currentPath, err := lp.GetVolumePath(testVolume1)
	assert.Nil(t, err)
	assert.Equal(t, expectedPath, currentPath)
}

func TestLVMProvisioner_getVGName_Success(t *testing.T) {
	setupTestLVMProvisioner()

	// not a system drive (SSDLVG)
	vgName, err := lp.getVGName(&testVolume1)
	assert.Nil(t, err)
	assert.Equal(t, testVolume1.Location, vgName)
}
