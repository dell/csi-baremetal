package utilwrappers

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/partitionhelper"
	mocklu "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks/linuxutils"
)

var (
	// constants from provisioner package
	DefaultPartitionLabel = "CSI"
	DefaultPartitionNumber = "1"

	testDevice1   = "/dev/sda"
	testPartUUID1 = "uuid-1111-2222-3333"

	testPart1 = Partition{
		Device:    testDevice1,
		Name:      "",
		Num:       DefaultPartitionNumber,
		TableType: partitionhelper.PartitionGPT,
		Label:     DefaultPartitionLabel,
		PartUUID:  testPartUUID1,
		Ephemeral: false,
	}
)

func setupTestPartitioner() (partOps *PartitionOperationsImpl, mockPH *mocklu.MockWrapPartition) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	partOps = NewPartitionOperationsImpl(&command.Executor{}, logger)
	mockPH = &mocklu.MockWrapPartition{}

	partOps.WrapPartition = mockPH

	return
}

func TestDriveProvisioner_PreparePartition_Success(t *testing.T) {
	var (
		partOps, mockPH = setupTestPartitioner()
		currentPPtr     *Partition
		err             error
	)

	// partition exist and have right UUID
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(true, nil).Once()
	mockPH.On("GetPartitionUUID", testPart1.Device, testPart1.Num).
		Return(testPart1.PartUUID, nil).Once()

	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, err)
	assert.Equal(t, testPart1, *currentPPtr)
	mockPH.Calls = []mock.Call{} // flush mock call records

	// setup mocks for scenarios when partition is created and volume whether ephemeral and no
	var partName = "p1"
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(false, nil).Twice()
	mockPH.On("CreatePartitionTable", testPart1.Device, testPart1.TableType).
		Return(nil).Twice()
	mockPH.On("CreatePartition", testPart1.Device, testPart1.Label).
		Return(nil).Twice()
	// if volume Ephemeral
	var partUUIDForEphemeral = "uuid-eeee"
	mockPH.On("GetPartitionUUID", testPart1.Device, testPart1.Num).
		Return(partUUIDForEphemeral, nil).Once()

	// if volume not an Ephemeral
	mockPH.On("SetPartitionUUID", testPart1.Device, testPart1.Num, testPart1.PartUUID).
		Return(nil).Once()

	// for each test scenario
	mockPH.On("SyncPartitionTable", mock.Anything).Return(nil)

	// not ephemeral
	mockPH.On("GetPartitionNameByUUID", testPart1.Device, testPart1.PartUUID).
		Return(partName, nil).Once()
	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, err)
	p := testPart1
	p.Name = partName
	assert.Equal(t, p, *currentPPtr)
	mockPH.AssertCalled(t, "SetPartitionUUID", testPart1.Device, testPart1.Num, testPart1.PartUUID)
	mockPH.AssertNotCalled(t, "GetPartitionUUID", testPart1.Device, testPart1.Num)
	mockPH.Calls = []mock.Call{} // flush mock call records

	// ephemeral
	mockPH.On("GetPartitionNameByUUID", testPart1.Device, partUUIDForEphemeral).
		Return(partName, nil).Once()
	pE := testPart1
	pE.Ephemeral = true
	currentPPtr, err = partOps.PreparePartition(pE)
	assert.Nil(t, err)
	pE.Name = partName
	pE.PartUUID = partUUIDForEphemeral
	assert.Equal(t, pE, *currentPPtr)
	mockPH.AssertCalled(t, "GetPartitionUUID", testPart1.Device, testPart1.Num)
	mockPH.AssertNotCalled(t, "SetPartitionUUID", testPart1.Device, testPart1.Num, testPart1.PartUUID)
}

func TestDriveProvisioner_PreparePartition_Failed(t *testing.T) {
	var (
		partOps, mockPH = setupTestPartitioner()
		expectedErr     = errors.New("prepare failed")
		currentPPtr     *Partition
		err             error
	)

	// IsPartitionExists failed
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(false, expectedErr).Once()

	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, currentPPtr)
	assert.NotNil(t, err)

	// next two scenarios rely on partition existence
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(true, nil).Twice()
	// partition exists, failed to get it UUID
	mockPH.On("GetPartitionUUID", testPart1.Device, testPart1.Num).
		Return("", expectedErr).Once()

	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, currentPPtr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "fail to get it UUID")

	// partition exists and it UUID doesn't match expected partUUID
	mockPH.On("GetPartitionUUID", testPart1.Device, testPart1.Num).
		Return("another-uuid", nil).Once()

	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, currentPPtr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "has already exist but have another UUID")

	// all next scenarios rely that partition isn't exist
	mockPH.On("IsPartitionExists", mock.Anything, mock.Anything).
		Return(false, nil)

	// CreatePartitionTable failed
	mockPH.On("CreatePartitionTable", testPart1.Device, testPart1.TableType).
		Return(expectedErr).Once()

	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, currentPPtr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to create partition table")

	// all next scenarios rely that CreatePartitionTable passes
	mockPH.On("CreatePartitionTable", mock.Anything, mock.Anything).
		Return(nil)

	// CreatePartition failed
	mockPH.On("CreatePartition", testPart1.Device, testPart1.Label).
		Return(expectedErr).Once()

	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, currentPPtr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to create partition")

	// all next scenarios rely that CreatePartition passes
	mockPH.On("CreatePartition", mock.Anything, mock.Anything).
		Return(nil)
	mockPH.On("SyncPartitionTable", mock.Anything).
		Return(nil)

	// SetPartitionUUID failed for non-ephemeral
	mockPH.On("SetPartitionUUID", testPart1.Device, testPart1.Num, testPart1.PartUUID).
		Return(expectedErr).Once()

	currentPPtr, err = partOps.PreparePartition(testPart1)
	assert.Nil(t, currentPPtr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to set partition UUID")

	// GetPartitionUUID failed for ephemeral
	mockPH.On("GetPartitionUUID", testPart1.Device, testPart1.Num).
		Return("", expectedErr).Once()

	pE := testPart1
	pE.Ephemeral = true
	currentPPtr, err = partOps.PreparePartition(pE)
	assert.Nil(t, currentPPtr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to get partition UUID for ephemeral volume")
}

func TestDriveProvisioner_ReleasePartition_Success(t *testing.T) {
	var (
		err             error
		partOps, mockPH = setupTestPartitioner()
	)

	// partition isn't exist
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(false, nil).Once()

	err = partOps.ReleasePartition(testPart1)
	assert.Nil(t, err)

	// partition exists and deleted successfully
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(true, nil).Once()
	mockPH.On("DeletePartition", testPart1.Device, testPart1.Num).
		Return(nil).Once()

	err = partOps.ReleasePartition(testPart1)
	assert.Nil(t, err)
	mockPH.AssertCalled(t, "DeletePartition", testPart1.Device, testPart1.Num)
}

func TestDriveProvisioner_ReleasePartition_Fail(t *testing.T) {
	var (
		partOps, mockPH = setupTestPartitioner()
		expectedErr     = errors.New("release error")
		err             error
	)

	// IsPartitionExists failed
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(false, expectedErr).Once()

	err = partOps.ReleasePartition(testPart1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to determine partition existence")

	// DeletePartition failed
	mockPH.On("IsPartitionExists", testPart1.Device, testPart1.Num).
		Return(true, nil).Once()
	mockPH.On("DeletePartition", testPart1.Device, testPart1.Num).
		Return(expectedErr).Once()

	err = partOps.ReleasePartition(testPart1)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}
