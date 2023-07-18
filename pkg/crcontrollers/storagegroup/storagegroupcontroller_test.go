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

func TestStorageGroupController_Reconcile(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)

	storageGroupController := NewController(kubeClient, kubeClient, testLogger)
	assert.NotNil(t, storageGroupController)
	assert.NotNil(t, storageGroupController.client)
	assert.NotNil(t, storageGroupController.crHelper)
	assert.NotNil(t, storageGroupController.log)
	assert.NotEqual(t, storageGroupController.log, testLogger)

	t.Run("manually add sg label to drive", func(t *testing.T) {
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

		testACResult := &accrd.AvailableCapacity{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC.Name, "", testACResult))
		assert.Equal(t, newSGName, testACResult.Labels[apiV1.StorageGroupLabelKey])

		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testDrive))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testAC))
	})

	t.Run("handle storage group creation", func(t *testing.T) {
		testAC1 := ac1.DeepCopy()
		testDrive1 := drive1.DeepCopy()
		testAC2 := ac2.DeepCopy()
		testDrive2 := drive2.DeepCopy()

		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testAC1.Name, testAC1))
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testDrive1.Name, testDrive1))
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testAC2.Name, testAC2))
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testDrive2.Name, testDrive2))

		testSG1 := sg1.DeepCopy()
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testSG1.Name, testSG1))

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: testSG1.Name}}
		assert.NotNil(t, req)

		res, err := storageGroupController.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		testSG1Result := &sgcrd.StorageGroup{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testSG1.Name, "", testSG1Result))
		assert.Equal(t, 1, len(testSG1Result.Finalizers))
		assert.Equal(t, apiV1.StorageGroupPhaseSynced, testSG1Result.Status.Phase)

		testAC1Result := &accrd.AvailableCapacity{}
		testAC2Result := &accrd.AvailableCapacity{}
		testDrive1Result := &dcrd.Drive{}
		testDrive2Result := &dcrd.Drive{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC1.Name, "", testAC1Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC2.Name, "", testAC2Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testDrive1.Name, "", testDrive1Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testDrive2.Name, "", testDrive2Result))
		assert.Equal(t, testSG1.Name, testAC1Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Equal(t, testSG1.Name, testDrive1Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Empty(t, testAC2Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Empty(t, testDrive2Result.Labels[apiV1.StorageGroupLabelKey])

		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testSG1))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testDrive1))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testAC1))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testDrive2))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testAC2))
	})
}
