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

package provisioners

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	api "github.com/dell/csi-baremetal/api/generated/v1"
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
	fsOps.On("CreateFSIfNotExist", fs.FileSystem(testVolume1.Type), devFile, testVolume1.Id).
		Return(nil).Times(1)

	err := lp.PrepareVolume(&testVolume1)
	assert.Nil(t, err)
}

func TestLVMProvisioner_PrepareVolume_Block_Success(t *testing.T) {
	setupTestLVMProvisioner()

	lvmOps.On("LVCreate", testVolume1.Id, mock.Anything, testVolume1.Location).
		Return(nil).Times(1)

	err := lp.PrepareVolume(&testVolume1Raw)
	assert.Nil(t, err)
}

func TestLVMProvisioner_PrepareVolume_Block_RawPart_Success(t *testing.T) {
	setupTestLVMProvisioner()

	lvmOps.On("LVCreate", testVolume1.Id, mock.Anything, testVolume1.Location).
		Return(nil).Times(1)

	err := lp.PrepareVolume(&testVolume1RawPart)
	assert.Nil(t, err)
}

func TestLVMProvisioner_PrepareVolume_Fail(t *testing.T) {
	setupTestLVMProvisioner()
	var err error

	// getVGName failed
	vol := testVolume1
	// in that case vgName will be searching in CRs and here we get error
	vol.StorageClass = apiV1.StorageClassSystemLVG

	err = lp.PrepareVolume(&vol)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to determine VG name")

	// LVCreate failed
	lvmOps.On("LVCreate", testVolume1.Id, mock.Anything, testVolume1.Location).
		Return(errTest).Times(1)

	err = lp.PrepareVolume(&testVolume1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to create LV")

	// CreateFS failed
	lvmOps.On("LVCreate", testVolume1.Id, mock.Anything, testVolume1.Location).
		Return(nil).Times(1)

	devFile := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	fsOps.On("CreateFSIfNotExist", fs.FileSystem(testVolume1.Type), devFile, testVolume1.Id).
		Return(errTest).Times(1)

	err = lp.PrepareVolume(&testVolume1)
	assert.NotNil(t, err)
	assert.Equal(t, errTest, err)
}

func TestLVMProvisioner_ReleaseVolume_Success(t *testing.T) {
	setupTestLVMProvisioner()

	var (
		devFile = fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
		err     error
	)

	fsOps.On("WipeFS", devFile).Return(nil).Times(1)
	lvmOps.On("LVRemove", devFile).Return(nil).Times(1)

	err = lp.ReleaseVolume(&testVolume1, &api.Drive{})
	assert.Nil(t, err)

	// WipeFS failed, LV isn't exist - ReleaseVolume success
	fsOps.On("WipeFS", devFile).Return(errTest).Times(1)
	lvmOps.On("GetLVsInVG", testVolume1.Location).Return(nil, nil).Times(1)

	err = lp.ReleaseVolume(&testVolume1, &api.Drive{})
	assert.Nil(t, err)
}

func TestLVMProvisioner_ReleaseVolume_Fail(t *testing.T) {
	setupTestLVMProvisioner()

	var err error

	// getVGName failed
	vol := testVolume1
	// in that case vgName will be searching in CRs and here we get error
	vol.StorageClass = apiV1.StorageClassSystemLVG

	err = lp.PrepareVolume(&vol)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to determine VG name")

	// WipeFS failed and LV still exist
	devFile := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	fsOps.On("WipeFS", devFile).Return(errTest).Times(1)
	lvmOps.On("GetLVsInVG", testVolume1.Location).Return([]string{testVolume1.Id}, nil).Times(1)

	err = lp.ReleaseVolume(&testVolume1, &api.Drive{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to wipe FS")

	// WipeFS failed and GetLVsInVG failed as well
	fsOps.On("WipeFS", devFile).Return(errTest).Times(1)
	lvmOps.On("GetLVsInVG", testVolume1.Location).Return(nil, errTest).Times(1)

	err = lp.ReleaseVolume(&testVolume1, &api.Drive{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to remove LV")
	assert.Contains(t, err.Error(), "and unable to list LVs in VG")

	// LVRemove failed and LV still exist
	fsOps.On("WipeFS", devFile).Return(nil).Times(1)
	lvmOps.On("LVRemove", devFile).Return(errTest).Times(1)
	lvmOps.On("GetLVsInVG", testVolume1.Location).
		Return([]string{testVolume1.Id}, nil).Times(1)

	err = lp.ReleaseVolume(&testVolume1, &api.Drive{})
	assert.NotNil(t, err)
	assert.Equal(t, errTest, err)
}

func TestLVMProvisioner_GetVolumePath_Success(t *testing.T) {
	setupTestLVMProvisioner()

	expectedPath := fmt.Sprintf("/dev/%s/%s", testVolume1.Location, testVolume1.Id)
	currentPath, err := lp.GetVolumePath(&testVolume1)
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
