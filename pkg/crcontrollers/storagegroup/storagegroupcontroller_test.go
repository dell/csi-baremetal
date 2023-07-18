package storagegroup

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	dcrd "github.com/dell/csi-baremetal/api/v1/drivecrd"
	sgcrd "github.com/dell/csi-baremetal/api/v1/storagegroupcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

var (
	testNs = "default"
	testID = "someID"
	nodeID = "node-uuid"

	testLogger         = logrus.New()
	testCtx            = context.Background()
	driveUUID1         = uuid.New().String()
	driveUUID2         = uuid.New().String()
	acUUID1            = uuid.New().String()
	acUUID2            = uuid.New().String()
	driveSerialNumber  = "VDH19UBD"
	driveSerialNumber2 = "MDH16UAC"
	sg1Name            = "hdd-group-1"
	sgrName            = "hdd-group-r"

	drive1 = dcrd.Drive{
		TypeMeta:   v1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: driveUUID1},
		Spec: api.Drive{
			UUID:         driveUUID1,
			Size:         1024 * 1024 * 1024 * 500,
			NodeId:       nodeID,
			Type:         apiV1.DriveTypeHDD,
			Status:       apiV1.DriveStatusOnline,
			Health:       apiV1.HealthGood,
			Slot:         "1",
			SerialNumber: driveSerialNumber,
		},
	}

	drive2 = dcrd.Drive{
		TypeMeta:   v1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: driveUUID2},
		Spec: api.Drive{
			UUID:         driveUUID2,
			Size:         1024 * 1024 * 1024 * 500,
			NodeId:       nodeID,
			Type:         apiV1.DriveTypeHDD,
			Status:       apiV1.DriveStatusOnline,
			Health:       apiV1.HealthGood,
			Slot:         "2",
			SerialNumber: driveSerialNumber2,
		},
	}

	ac1 = accrd.AvailableCapacity{
		TypeMeta:   v1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: acUUID1},
		Spec: api.AvailableCapacity{
			Size:         drive1.Spec.Size,
			StorageClass: apiV1.StorageClassHDD,
			Location:     driveUUID1,
			NodeId:       nodeID},
	}

	ac2 = accrd.AvailableCapacity{
		TypeMeta:   v1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: acUUID2},
		Spec: api.AvailableCapacity{
			Size:         drive2.Spec.Size,
			StorageClass: apiV1.StorageClassHDD,
			Location:     driveUUID2,
			NodeId:       nodeID},
	}

	sg1 = sgcrd.StorageGroup{
		TypeMeta:   v1.TypeMeta{Kind: "StorageGroup", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: sg1Name},
		Spec: api.StorageGroupSpec{
			DriveSelector: &api.DriveSelector{
				MatchFields: map[string]string{
					"Slot": "1",
					"Type": "HDD",
				},
			},
		},
	}

	sgr = sgcrd.StorageGroup{
		TypeMeta:   v1.TypeMeta{Kind: "StorageGroup", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: sg1Name},
		Spec: api.StorageGroupSpec{
			DriveSelector: &api.DriveSelector{
				NumberDrivesPerNode: 1,
				MatchFields:         map[string]string{"Type": "HDD"},
			},
		},
	}
)

func TestStorageGroupController_NewController(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)

	storageGroupController := NewController(kubeClient, kubeClient, testLogger)
	assert.NotNil(t, storageGroupController)
	assert.NotNil(t, storageGroupController.client)
	assert.NotNil(t, storageGroupController.crHelper)

	assert.NotNil(t, storageGroupController.log)
	assert.NotEqual(t, storageGroupController.log, testLogger)
}

func TestStorageGroupController_addRemoveACStorageGroupLabel(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)

	storageGroupController := NewController(kubeClient, kubeClient, testLogger)
	assert.NotNil(t, storageGroupController)
	assert.NotNil(t, storageGroupController.client)
	assert.NotNil(t, storageGroupController.crHelper)
	assert.NotNil(t, storageGroupController.log)
	assert.NotEqual(t, storageGroupController.log, testLogger)

	t.Run("test on AC's sg label", func(t *testing.T) {
		newSGName := "hdd-group-new"

		testAC := ac1.DeepCopy()
		testDrive := drive1.DeepCopy()
		testDrive.Labels = map[string]string{apiV1.StorageGroupLabelKey: newSGName}

		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testAC.Name, testAC))
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testDrive.Name, testDrive))

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: testDrive.Name}}
		assert.NotNil(t, req)

		res, err := storageGroupController.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		resultAC := &accrd.AvailableCapacity{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC.Name, "", resultAC))
		assert.Equal(t, newSGName, resultAC.Labels[apiV1.StorageGroupLabelKey])

		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testDrive))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testAC))
	})
}
