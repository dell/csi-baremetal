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

package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
)

func setup() *CRHelper {
	k, err := GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	return NewCRHelper(k, testLogger)
}

func TestCRHelper_GetACByLocation(t *testing.T) {
	ch := setup()
	expectedAC := testACCR
	err := ch.k8sClient.CreateCR(testCtx, expectedAC.Name, &expectedAC)
	assert.Nil(t, err)

	currentAC, err := ch.GetACByLocation(testACCR.Spec.Location)
	assert.Nil(t, err)
	assert.Equal(t, expectedAC.Spec, currentAC.Spec)

	// expected nil because of empty string as a location
	currentAC, err = ch.GetACByLocation("")
	assert.Equal(t, err, errTypes.ErrorNotFound)
}

func TestCRHelper_GetVolumeByLocation(t *testing.T) {
	ch := setup()
	expectedV := testVolumeCR.DeepCopy()
	err := ch.k8sClient.CreateCR(testCtx, expectedV.Name, expectedV)
	assert.Nil(t, err)
	ctx := context.Background()
	currentVols, _ := ch.GetVolumesByLocation(ctx, expectedV.Spec.Location)
	assert.NotEmpty(t, currentVols)
	assert.Equal(t, expectedV.Spec, currentVols[0].Spec)

	// expected nil because of empty string as a location
	currentVols, _ = ch.GetVolumesByLocation(ctx, "")
	assert.Nil(t, currentVols)

	// lvm
	ch = setup()
	testVolume := testVolumeCR.DeepCopy()
	testVolume.Spec.Location = testLVGCR.Name
	testVolume.Spec.LocationType = v1.LocationTypeLVM
	err = ch.k8sClient.CreateCR(testCtx, testVolume.Name, testVolume)
	assert.Nil(t, err)
	testLVGCR1 := testLVGCR.DeepCopy()
	err = ch.k8sClient.CreateCR(testCtx, testLVGCR.Name, testLVGCR1)
	assert.Nil(t, err)
	currentVols, _ = ch.GetVolumesByLocation(ctx, testDriveLocation1)
	assert.NotEmpty(t, currentVols)
}

func TestCRHelper_GetVolumeByID(t *testing.T) {
	ch := setup()
	expectedV := testVolumeCR
	err := ch.k8sClient.CreateCR(testCtx, expectedV.Name, &expectedV)
	assert.Nil(t, err)

	currentV, err := ch.GetVolumeByID(expectedV.Spec.Id)
	assert.Nil(t, err)
	assert.NotNil(t, currentV)
	assert.Equal(t, expectedV.Spec, currentV.Spec)

	// expected nil because of empty string as a ID
	volume, err := ch.GetVolumeByID("")
	assert.NotNil(t, err)
	assert.Nil(t, volume)
}

func TestCRHelper_GetDriveCRAndLVGCRByVolume(t *testing.T) {
	var (
		volume = testVolumeCR.DeepCopy()
		lvg    = testLVGCR.DeepCopy()
		drive  = testDriveCR.DeepCopy()
	)

	// Positive case: volume point to lvg
	volume.Spec.Location = lvg.Name
	volume.Spec.LocationType = v1.LocationTypeLVM
	lvg.Spec.Locations = []string{drive.Name}

	ch := setup()
	err := ch.k8sClient.CreateCR(testCtx, volume.Name, volume)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, lvg.Name, lvg)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, drive.Name, drive)
	assert.Nil(t, err)
	driveNew, lvgNew, err := ch.GetDriveCRAndLVGCRByVolume(volume)
	assert.NotNil(t, driveNew)
	assert.NotNil(t, lvgNew)
	assert.Nil(t, err)

	// Negative case, lvg.Spec.Locations is empty
	lvg.Spec.Locations = []string{}
	err = ch.k8sClient.UpdateCR(testCtx, lvg)
	assert.Nil(t, err)

	driveNew, lvgNew, err = ch.GetDriveCRAndLVGCRByVolume(volume)
	assert.Nil(t, driveNew)
	assert.Nil(t, lvgNew)
	assert.NotNil(t, err)

	// Positive case: volume point to drive
	volume.Spec.Location = drive.Name
	volume.Spec.LocationType = v1.LocationTypeDrive
	err = ch.k8sClient.UpdateCR(testCtx, volume)
	assert.Nil(t, err)

	driveNew, lvgNew, err = ch.GetDriveCRAndLVGCRByVolume(volume)
	assert.NotNil(t, driveNew)
	assert.Nil(t, lvgNew)
	assert.Nil(t, err)

}
func TestCRHelper_GetDriveCRByVolume(t *testing.T) {
	var (
		volume = testVolumeCR.DeepCopy()
		lvg    = testLVGCR.DeepCopy()
		drive  = testDriveCR.DeepCopy()
	)

	// Positive case: volume point to lvg
	volume.Spec.Location = lvg.Name
	volume.Spec.LocationType = v1.LocationTypeLVM
	lvg.Spec.Locations = []string{drive.Name}

	ch := setup()
	err := ch.k8sClient.CreateCR(testCtx, volume.Name, volume)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, lvg.Name, lvg)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, drive.Name, drive)
	assert.Nil(t, err)
	driveNew, err := ch.GetDriveCRByVolume(volume)
	assert.NotNil(t, driveNew)
	assert.Nil(t, err)

	// Positive case: volume point to drive
	volume.Spec.Location = drive.Name
	volume.Spec.LocationType = v1.LocationTypeDrive
	err = ch.k8sClient.UpdateCR(testCtx, volume)
	assert.Nil(t, err)

	driveNew, err = ch.GetDriveCRByVolume(volume)
	assert.NotNil(t, driveNew)
	assert.Nil(t, err)

}

func TestCRHelper_GetVolumeCRs(t *testing.T) {
	ch := setup()
	vol1 := testVolumeCR.DeepCopy()
	vol2 := testVolumeCR.DeepCopy()
	vol2.Name = "anotherName"
	vol2.Spec.NodeId = "anotherNode"

	err := ch.k8sClient.CreateCR(testCtx, vol1.Name, vol1)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, vol2.Name, vol2)
	assert.Nil(t, err)

	// node as empty string - expected all volumes
	currentVs, _ := ch.GetVolumeCRs()
	assert.NotNil(t, currentVs)
	assert.Equal(t, 2, len(currentVs))

	// expected one volume
	currentVs, _ = ch.GetVolumeCRs(vol1.Spec.NodeId)
	assert.NotNil(t, currentVs)
	assert.Equal(t, 1, len(currentVs))
	assert.Equal(t, vol1.Spec, currentVs[0].Spec)
}

func TestCRHelper_GetDriveCRs(t *testing.T) {
	ch := setup()
	d1 := testDriveCR.DeepCopy()
	d2 := testDriveCR.DeepCopy()
	d2.Name = "anotherName"
	d2.Spec.NodeId = "anotherNode"

	err := ch.k8sClient.CreateCR(testCtx, d1.Name, d1)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, d2.Name, d2)
	assert.Nil(t, err)

	// node as empty string - expected all drives
	currentDs, _ := ch.GetDriveCRs()
	assert.NotNil(t, currentDs)
	assert.Equal(t, 2, len(currentDs))

	// expected one volume
	currentDs, _ = ch.GetDriveCRs(d1.Spec.NodeId)
	assert.NotNil(t, currentDs)
	assert.Equal(t, 1, len(currentDs))
	assert.Equal(t, d1.Spec, currentDs[0].Spec)
}

func TestCRHelper_GetVGNameByLVGCRName(t *testing.T) {
	ch := setup()
	lvgCR := testLVGCR
	err := ch.k8sClient.CreateCR(testCtx, lvgCR.Name, &lvgCR)
	assert.Nil(t, err)

	currentVGName, err := ch.GetVGNameByLVGCRName(lvgCR.Name)
	assert.Nil(t, err)
	assert.Equal(t, lvgCR.Spec.Name, currentVGName)

	// expected that LVG will not be found
	currentVGName, err = ch.GetVGNameByLVGCRName("randomName")
	assert.NotNil(t, err)
	assert.Equal(t, "", currentVGName)
}

// test AC deletion
func TestCRHelper_DeleteACsByNodeID(t *testing.T) {
	mock := setup()
	testACCRCopy := testACCR.DeepCopy()
	err := mock.k8sClient.CreateCR(testCtx, testACCR.Name, testACCRCopy)
	assert.Nil(t, err)

	err = mock.DeleteACsByNodeID(testACCRCopy.Spec.NodeId)
	assert.Nil(t, err)
}

// test Drive status update
func TestCRHelper_UpdateDrivesStatusOnNode(t *testing.T) {
	mock := setup()
	testDriveCRCopy := testDriveCR.DeepCopy()
	err := mock.k8sClient.CreateCR(testCtx, testDriveCRCopy.Name, testDriveCRCopy)
	assert.Nil(t, err)

	err = mock.UpdateDrivesStatusOnNode(testDriveCRCopy.Spec.NodeId, v1.DriveStatusOffline)
	assert.Nil(t, err)

	drives, err := mock.GetDriveCRs(testDriveCRCopy.Spec.NodeId)
	assert.Nil(t, err)
	assert.Len(t, drives, 1)
	assert.Equal(t, drives[0].Spec.Status, v1.DriveStatusOffline)
}

// test Volume operational status update
func TestCRHelper_UpdateVolumesOpStatusOnNode(t *testing.T) {
	mock := setup()
	err := mock.k8sClient.CreateCR(testCtx, testVolume.Name, testVolume.DeepCopy())
	assert.Nil(t, err)

	err = mock.UpdateVolumesOpStatusOnNode(testVolume.Spec.NodeId, v1.OperationalStatusMissing)
	assert.Nil(t, err)

	volume, err := mock.GetVolumeByID(testVolume.Name)
	assert.Nil(t, err)
	assert.Equal(t, volume.Spec.OperationalStatus, v1.OperationalStatusMissing)
}

func TestCRHelper_DeleteObjectByName(t *testing.T) {
	mock := setup()
	// object does not exist
	err := mock.DeleteObjectByName(testCtx, "aaaa", "", &accrd.AvailableCapacity{})
	assert.Nil(t, err)

	assert.Nil(t, mock.k8sClient.CreateCR(testCtx, testVolumeCR.Name, &testVolumeCR))
	assert.Nil(t, mock.DeleteObjectByName(testCtx, testVolumeCR.Name, "", &volumecrd.Volume{}))

	vList := &volumecrd.VolumeList{}
	assert.Nil(t, mock.k8sClient.ReadList(testCtx, vList))
	assert.Equal(t, 0, len(vList.Items))
}
