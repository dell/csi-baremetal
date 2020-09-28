package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
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

	currentAC := ch.GetACByLocation(testACCR.Spec.Location)
	assert.NotNil(t, currentAC)
	assert.Equal(t, expectedAC.Spec, currentAC.Spec)

	// expected nil because of empty string as a location
	assert.Nil(t, ch.GetACByLocation(""))
}

func TestCRHelper_GetVolumeByLocation(t *testing.T) {
	ch := setup()
	expectedV := testVolumeCR
	err := ch.k8sClient.CreateCR(testCtx, expectedV.Name, &expectedV)
	assert.Nil(t, err)

	currentV := ch.GetVolumeByLocation(expectedV.Spec.Location)
	assert.NotNil(t, currentV)
	assert.Equal(t, expectedV.Spec, currentV.Spec)

	// expected nil because of empty string as a location
	assert.Nil(t, ch.GetVolumeByLocation(""))
}

func TestCRHelper_GetVolumeByID(t *testing.T) {
	ch := setup()
	expectedV := testVolumeCR
	err := ch.k8sClient.CreateCR(testCtx, expectedV.Name, &expectedV)
	assert.Nil(t, err)

	currentV := ch.GetVolumeByID(expectedV.Spec.Id)
	assert.NotNil(t, currentV)
	assert.Equal(t, expectedV.Spec, currentV.Spec)

	// expected nil because of empty string as a ID
	assert.Nil(t, ch.GetVolumeByID(""))
}

func TestCRHelper_GetDriveCRByUUID(t *testing.T) {
	ch := setup()
	expectedD := testDriveCR
	err := ch.k8sClient.CreateCR(testCtx, expectedD.Name, &expectedD)
	assert.Nil(t, err)

	currentD := ch.GetDriveCRByUUID(expectedD.Spec.UUID)
	assert.NotNil(t, currentD)
	assert.Equal(t, expectedD.Spec, currentD.Spec)

	// expected nil because of empty string as a ID
	assert.Nil(t, ch.GetDriveCRByUUID(""))
}

func TestCRHelper_GetVolumeCRs(t *testing.T) {
	ch := setup()
	v1 := testVolumeCR
	v2 := testVolumeCR
	v2.Name = "anotherName"
	v2.Spec.NodeId = "anotherNode"

	err := ch.k8sClient.CreateCR(testCtx, v1.Name, &v1)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, v2.Name, &v2)
	assert.Nil(t, err)

	// node as empty string - expected all volumes
	currentVs, _ := ch.GetVolumeCRs()
	assert.NotNil(t, currentVs)
	assert.Equal(t, 2, len(currentVs))

	// expected one volume
	currentVs, _ = ch.GetVolumeCRs(v1.Spec.NodeId)
	assert.NotNil(t, currentVs)
	assert.Equal(t, 1, len(currentVs))
	assert.Equal(t, v1.Spec, currentVs[0].Spec)
}

func TestCRHelper_GetDriveCRs(t *testing.T) {
	ch := setup()
	d1 := testDriveCR
	d2 := testDriveCR
	d2.Name = "anotherName"
	d2.Spec.NodeId = "anotherNode"

	err := ch.k8sClient.CreateCR(testCtx, d1.Name, &d1)
	assert.Nil(t, err)
	err = ch.k8sClient.CreateCR(testCtx, d2.Name, &d2)
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
	err := mock.k8sClient.CreateCR(testCtx, testACCR.Name, &testACCR)
	assert.Nil(t, err)

	err = mock.DeleteACsByNodeID(testACCR.Spec.NodeId)
	assert.Nil(t, err)
}

// test Drive status update
func TestCRHelper_UpdateDrivesStatusOnNode(t *testing.T) {
	mock := setup()
	err := mock.k8sClient.CreateCR(testCtx, testDriveCR.Name, &testDriveCR)
	assert.Nil(t, err)

	err = mock.UpdateDrivesStatusOnNode(testDriveCR.Spec.NodeId, v1.DriveStatusOffline)
	assert.Nil(t, err)

	drive := mock.GetDriveCRByUUID(testDriveCR.Name)
	assert.Equal(t, drive.Spec.Status, v1.DriveStatusOffline)
}

// test Volume operational status update
func TestCRHelper_UpdateVolumesOpStatusOnNode(t *testing.T) {
	mock := setup()
	err := mock.k8sClient.CreateCR(testCtx, testVolume.Name, &testVolume)
	assert.Nil(t, err)

	err = mock.UpdateVolumesOpStatusOnNode(testVolume.Spec.NodeId, v1.OperationalStatusMissing)
	assert.Nil(t, err)

	volume := mock.GetVolumeByID(testVolume.Name)
	assert.Equal(t, volume.Spec.OperationalStatus, v1.OperationalStatusMissing)
}

func TestCRHelper_DeleteObjectByName(t *testing.T) {
	mock := setup()
	// object does not exist
	err := mock.DeleteObjectByName(testCtx, "aaaa", &accrd.AvailableCapacity{})
	assert.Nil(t, err)

	assert.Nil(t, mock.k8sClient.CreateCR(testCtx, testVolumeCR.Name, &testVolumeCR))
	assert.Nil(t, mock.DeleteObjectByName(testCtx, testVolumeCR.Name, &volumecrd.Volume{}))

	vList := &volumecrd.VolumeList{}
	assert.Nil(t, mock.k8sClient.ReadList(testCtx, vList))
	assert.Equal(t, 0, len(vList.Items))
}
