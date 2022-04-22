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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
	mockProv "github.com/dell/csi-baremetal/pkg/mocks/provisioners"
	uw "github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
)

// setupTestDriveProvisioner creates DriveProvisioner and all mock fields and return them
func setupTestDriveProvisioner() (dp *DriveProvisioner,
	mockLsblk *mocklu.MockWrapLsblk,
	mockPH *mockProv.MockPartitionOps,
	mockFS *mockProv.MockFsOpts) {
	fakeK8s, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	dp = NewDriveProvisioner(&command.Executor{}, fakeK8s, logger)
	mockLsblk = &mocklu.MockWrapLsblk{}
	mockPH = &mockProv.MockPartitionOps{}
	mockFS = &mockProv.MockFsOpts{}

	dp.listBlk = mockLsblk
	dp.partOps = mockPH
	dp.fsOps = mockFS

	return
}

func TestDriveProvisioner_PrepareVolume_Success(t *testing.T) {
	var (
		dp, mockLsblk, mockPH, mockFS = setupTestDriveProvisioner()
		err                           error
	)

	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	var (
		device = "/some/device"
		part   = uw.Partition{
			Device:    device,
			TableType: partitionhelper.PartitionGPT,
			Label:     DefaultPartitionLabel,
			Num:       DefaultPartitionNumber,
			PartUUID:  testVolume2.Id,
		}
		expectedPart = uw.Partition{
			Device:    device,
			TableType: partitionhelper.PartitionGPT,
			Label:     DefaultPartitionLabel,
			Num:       DefaultPartitionNumber,
			PartUUID:  testVolume2.Id,
			Name:      "p1n1",
		}
	)

	mockLsblk.On("SearchDrivePath", &testDriveCR.Spec).Return(device, nil)
	mockPH.On("PreparePartition", part).Return(&expectedPart, nil)
	mockFS.On("CreateFSIfNotExist", fs.FileSystem(testVolume2.Type), expectedPart.GetFullPath(), testVolume2.Id).
		Return(nil)

	err = dp.PrepareVolume(&testVolume2)
	assert.Nil(t, err)
}

func TestDriveProvisioner_PrepareVolume_Block_Success(t *testing.T) {
	var (
		dp, mockLsblk, _, _ = setupTestDriveProvisioner()
		err                 error
	)

	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	var (
		device = "/some/device"
	)

	mockLsblk.On("SearchDrivePath", &testDriveCR.Spec).Return(device, nil)

	err = dp.PrepareVolume(&testVolume2Raw)
	assert.Nil(t, err)
}

func TestDriveProvisioner_PrepareVolume_Blockrawpart_Success(t *testing.T) {
	var (
		dp, mockLsblk, mockPH, _ = setupTestDriveProvisioner()
		err                      error
	)

	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	var (
		device = "/some/device"
		part   = uw.Partition{
			Device:    device,
			TableType: partitionhelper.PartitionGPT,
			Label:     DefaultPartitionLabel,
			Num:       DefaultPartitionNumber,
			PartUUID:  testVolume2RawPart.Id,
		}
		expectedPart = uw.Partition{
			Device:    device,
			TableType: partitionhelper.PartitionGPT,
			Label:     DefaultPartitionLabel,
			Num:       DefaultPartitionNumber,
			PartUUID:  testVolume2RawPart.Id,
			Name:      "p1n1",
		}
	)

	mockLsblk.On("SearchDrivePath", &testDriveCR.Spec).Return(device, nil)
	mockPH.On("PreparePartition", part).Return(&expectedPart, nil)

	err = dp.PrepareVolume(&testVolume2RawPart)
	assert.Nil(t, err)
}

func TestDriveProvisioner_PrepareVolume_Fail(t *testing.T) {
	var (
		dp, mockLsblk, mockPH, mockFS = setupTestDriveProvisioner()
		err                           error
	)

	// drive CR isn't exist
	err = dp.PrepareVolume(&testVolume2)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to read drive CR with name")

	// add drive CR
	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	// SearchDrivePath failed
	mockLsblk.On("SearchDrivePath", mock.Anything).
		Return("", errTest).Once()

	err = dp.PrepareVolume(&testVolume2)
	assert.Error(t, err)
	assert.Equal(t, errTest, err)

	// all next scenarios rely that SearchDrivePath passes
	mockLsblk.On("SearchDrivePath", mock.Anything).
		Return("some-path", nil)

	// PreparePartition failed
	mockPH.On("PreparePartition", mock.Anything).
		Return(&uw.Partition{}, errTest).Once()

	err = dp.PrepareVolume(&testVolume2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to prepare partition for volume")

	// CreateFS failed
	mockPH.On("PreparePartition", mock.Anything).
		Return(&uw.Partition{}, nil).Once()
	mockFS.On("CreateFSIfNotExist", fs.FileSystem(testVolume2.Type), mock.Anything, testVolume2.Id).Return(errTest)

	err = dp.PrepareVolume(&testVolume2)
	assert.Error(t, err)
	assert.Equal(t, errTest, err)
}

func TestDriveProvisioner_ReleaseVolume_Success(t *testing.T) {
	var (
		dp, mockLsblk, mockPH, mockFS = setupTestDriveProvisioner()
		err                           error
	)

	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	var (
		deviceFile = "/dev/sda"
		partName   = "p1"
		part       = uw.Partition{
			Device:    deviceFile,
			Name:      partName,
			Num:       DefaultPartitionNumber,
			TableType: "",
			Label:     "",
			PartUUID:  testVolume2.Id,
		}
	)

	mockLsblk.On("SearchDrivePath", &testDriveCR.Spec).Return(deviceFile, nil)
	mockPH.On("SearchPartName", deviceFile, testVolume2.Id).Return(partName, nil).Once()
	mockFS.On("WipeFS", deviceFile+partName).Return(nil).Once()
	mockPH.On("ReleasePartition", part).Return(nil)
	mockFS.On("WipeFS", deviceFile).Return(nil).Once()

	err = dp.ReleaseVolume(&testVolume2, &testDriveCR.Spec)
	assert.Nil(t, err)

	// SearchPartName failed but partition isn't exist (was removed before)
	mockLsblk.On("SearchDrivePath",
		mock.MatchedBy(func(d *drivecrd.Drive) bool { return d.Name == testDriveCR.Name })).
		Return(deviceFile, nil).Once()
	mockPH.On("SearchPartName", deviceFile, testVolume2.Id).Return("", errTest).Once()
	mockLsblk.On("GetBlockDevices", deviceFile).Return(nil, nil).Once()
	mockFS.On("WipeFS", deviceFile).Return(nil).Once()

	err = dp.ReleaseVolume(&testVolume2, &testDriveCR.Spec)
	assert.Nil(t, err)
}

func TestDriveProvisioner_ReleaseVolume_Fail(t *testing.T) {
	var (
		dp, mockLsblk, mockPH, mockFS = setupTestDriveProvisioner()
		err                           error
	)

	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	// SearchDrivePath failed
	mockLsblk.On("SearchDrivePath", &testDriveCR.Spec).Return("", errTest).Once()

	err = dp.ReleaseVolume(&testVolume2, &testDriveCR.Spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find device for drive with S/N")

	// next scenarios rely on SearchDrivePath passes
	var deviceFile = "/dev/sdw"
	mockLsblk.On("SearchDrivePath", mock.Anything).
		Return(deviceFile, nil)

	// SearchPartName returns empty string and GetBlockDevices return error
	mockPH.On("SearchPartName", deviceFile, testVolume2.Id).
		Return("").Once()
	mockLsblk.On("GetBlockDevices", deviceFile).
		Return(nil, errTest)

	err = dp.ReleaseVolume(&testVolume2, &testDriveCR.Spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find partition name")

	// next scenarios rely on SearchPartName passes
	var partName = "p1n1"
	mockPH.On("SearchPartName", deviceFile, testVolume2.Id).
		Return(partName)

	// WipeFS failed
	mockFS.On("WipeFS", deviceFile+partName).Return(errTest).Once()

	err = dp.ReleaseVolume(&testVolume2, &testDriveCR.Spec)
	assert.Error(t, err)
	assert.Equal(t, errTest, err)

	// ReleasePartition failed
	mockFS.On("WipeFS", mock.Anything).Return(nil).Once()
	mockPH.On("ReleasePartition", mock.Anything).Return(errTest).Once()

	err = dp.ReleaseVolume(&testVolume2, &testDriveCR.Spec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to release partition")

	// WipeFS on device file failed
	mockFS.On("WipeFS", mock.Anything).Return(nil).Once()
	mockPH.On("ReleasePartition", mock.Anything).Return(nil)
	mockFS.On("WipeFS", deviceFile).Return(errTest)

	err = dp.ReleaseVolume(&testVolume2, &testDriveCR.Spec)
	assert.Error(t, err)
	assert.Equal(t, errTest, err)
}

func TestDriveProvisioner_GetVolumePath_Success(t *testing.T) {
	var (
		dp, mockLsblk, mockPH, _ = setupTestDriveProvisioner()
		fullPath                 string
		err                      error
	)

	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	var (
		deviceFile = "/dev/sda"
		partName   = "p1"
	)

	mockLsblk.On("SearchDrivePath", &testDriveCR.Spec).Return(deviceFile, nil).Once()
	mockPH.On("SearchPartName", deviceFile, testVolume2.Id).
		Return(partName, nil).Once()

	fullPath, err = dp.GetVolumePath(&testVolume2)
	assert.Nil(t, err)
	assert.Equal(t, deviceFile+partName, fullPath)
}

func TestDriveProvisioner_GetVolumePath_Fail(t *testing.T) {
	var (
		dp, mockLsblk, mockPH, _ = setupTestDriveProvisioner()
		fullPath                 string
		err                      error
	)

	// failed to find DriveCR
	fullPath, err = dp.GetVolumePath(&api.Volume{})
	assert.Error(t, err)
	assert.Equal(t, "", fullPath)

	err = dp.k8sClient.CreateCR(testCtx, testDriveCR.Name, testDriveCR.DeepCopy())
	assert.Nil(t, err)

	// SearchDrivePath
	mockLsblk.On("SearchDrivePath", &testDriveCR.Spec).Return("", errTest).Once()

	fullPath, err = dp.GetVolumePath(&testVolume2)
	assert.Error(t, err)
	assert.Equal(t, "", fullPath)
	assert.Contains(t, err.Error(), "unable to find device for drive with S/N")

	// SearchPartName failed
	var deviceFile = "/dev/sdw"
	mockLsblk.On("SearchDrivePath", mock.Anything).
		Return(deviceFile, nil).Once()
	mockPH.On("SearchPartName", deviceFile, testVolume2.Id).
		Return("").Once()

	fullPath, err = dp.GetVolumePath(&testVolume2)
	assert.Error(t, err)
	assert.Equal(t, "", fullPath)
	assert.Contains(t, err.Error(), "unable to find part name for device")
}
