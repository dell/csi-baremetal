package storagegroup

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	sg2Name            = "hdd-group-r"
	sg3Name            = "hdd-group-invalid"

	drive1 = &dcrd.Drive{
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
			IsClean:      true,
		},
	}

	drive2 = &dcrd.Drive{
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
			IsClean:      true,
		},
	}

	ac1 = &accrd.AvailableCapacity{
		TypeMeta:   v1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: acUUID1},
		Spec: api.AvailableCapacity{
			Size:         drive1.Spec.Size,
			StorageClass: apiV1.StorageClassHDD,
			Location:     driveUUID1,
			NodeId:       nodeID},
	}

	ac2 = &accrd.AvailableCapacity{
		TypeMeta:   v1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: acUUID2},
		Spec: api.AvailableCapacity{
			Size:         drive2.Spec.Size,
			StorageClass: apiV1.StorageClassHDD,
			Location:     driveUUID2,
			NodeId:       nodeID},
	}

	sg1 = &sgcrd.StorageGroup{
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

	sg2 = &sgcrd.StorageGroup{
		TypeMeta:   v1.TypeMeta{Kind: "StorageGroup", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: sg2Name},
		Spec: api.StorageGroupSpec{
			DriveSelector: &api.DriveSelector{
				NumberDrivesPerNode: 1,
				MatchFields:         map[string]string{"Type": "HDD"},
			},
		},
	}

	sg3 = &sgcrd.StorageGroup{
		TypeMeta:   v1.TypeMeta{Kind: "StorageGroup", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: sg3Name},
		Spec: api.StorageGroupSpec{
			DriveSelector: &api.DriveSelector{
				MatchFields: map[string]string{
					"Type":  "HDD",
					"IsSSD": "no",
				},
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

	t.Run("reconcile drive with sg label manually added", func(t *testing.T) {
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
		assert.Empty(t, testACResult.Labels[apiV1.StorageGroupLabelKey])

		testDriveResult := &dcrd.Drive{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testDrive.Name, "", testDriveResult))
		assert.Empty(t, testDriveResult.Labels[apiV1.StorageGroupLabelKey])

		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testDrive))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testAC))
	})

	t.Run("reconcile storage groups and drives", func(t *testing.T) {
		// setup resources
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
		testSG2 := sg2.DeepCopy()
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testSG2.Name, testSG2))

		// reconcile creation of testSG1 and testSG2
		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: testSG1.Name}}
		assert.NotNil(t, req)

		res, err := storageGroupController.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: testSG2.Name}}
		assert.NotNil(t, req)

		res, err = storageGroupController.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		testSG1Result := &sgcrd.StorageGroup{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testSG1.Name, "", testSG1Result))
		assert.Equal(t, 1, len(testSG1Result.Finalizers))
		assert.Equal(t, apiV1.StorageGroupPhaseSynced, testSG1Result.Status.Phase)
		testSG2Result := &sgcrd.StorageGroup{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testSG2.Name, "", testSG2Result))
		assert.Equal(t, 1, len(testSG2Result.Finalizers))
		assert.Equal(t, apiV1.StorageGroupPhaseSynced, testSG2Result.Status.Phase)

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
		assert.Equal(t, testSG2.Name, testAC2Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Equal(t, testSG2.Name, testDrive2Result.Labels[apiV1.StorageGroupLabelKey])

		// reconcile deletion of testSG1
		testSG1Result.DeletionTimestamp = &v1.Time{Time: time.Now()}
		assert.Nil(t, storageGroupController.client.UpdateCR(testCtx, testSG1Result))

		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: testSG1.Name}}
		assert.NotNil(t, req)

		res, err = storageGroupController.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		err = storageGroupController.client.ReadCR(testCtx, testSG1.Name, "", testSG1Result)
		assert.True(t, k8serrors.IsNotFound(err))

		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC1.Name, "", testAC1Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC2.Name, "", testAC2Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testDrive1.Name, "", testDrive1Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testDrive2.Name, "", testDrive2Result))
		assert.Empty(t, testAC1Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Empty(t, testDrive1Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Equal(t, testSG2.Name, testAC2Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Equal(t, testSG2.Name, testDrive2Result.Labels[apiV1.StorageGroupLabelKey])

		// reconcile testDrive1 without testSG1
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: testDrive1.Name}}
		assert.NotNil(t, req)

		res, err = storageGroupController.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testSG2.Name, "", testSG2Result))
		assert.Equal(t, apiV1.StorageGroupPhaseSyncing, testSG2Result.Status.Phase)

		// reconcile testDrive1 with testSG1
		testSG1 = sg1.DeepCopy()
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testSG1.Name, testSG1))

		res, err = storageGroupController.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testSG1.Name, "", testSG1Result))
		assert.Equal(t, apiV1.StorageGroupPhaseSyncing, testSG1Result.Status.Phase)

		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC1.Name, "", testAC1Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testAC2.Name, "", testAC2Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testDrive1.Name, "", testDrive1Result))
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testDrive2.Name, "", testDrive2Result))
		assert.Equal(t, testSG1.Name, testAC1Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Equal(t, testSG1.Name, testDrive1Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Equal(t, testSG2.Name, testAC2Result.Labels[apiV1.StorageGroupLabelKey])
		assert.Equal(t, testSG2.Name, testDrive2Result.Labels[apiV1.StorageGroupLabelKey])

		// delete resources
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testSG1))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testSG2))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testDrive1))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testAC1))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testDrive2))
		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testAC2))
	})
}

func TestStorageGroupController_reconcileStorageGroup(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)

	storageGroupController := NewController(kubeClient, kubeClient, testLogger)
	assert.NotNil(t, storageGroupController)
	assert.NotNil(t, storageGroupController.client)
	assert.NotNil(t, storageGroupController.crHelper)
	assert.NotNil(t, storageGroupController.log)
	assert.NotEqual(t, storageGroupController.log, testLogger)

	t.Run("reconcile invalid storage group", func(t *testing.T) {
		testSG3 := sg3.DeepCopy()
		assert.Nil(t, storageGroupController.client.CreateCR(testCtx, testSG3.Name, testSG3))

		res, err := storageGroupController.reconcileStorageGroup(testCtx, testSG3)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		testSG3Result := &sgcrd.StorageGroup{}
		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testSG3.Name, "", testSG3Result))
		assert.Equal(t, apiV1.StorageGroupPhaseInvalid, testSG3Result.Status.Phase)

		res, err = storageGroupController.reconcileStorageGroup(testCtx, testSG3Result)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		assert.Nil(t, storageGroupController.client.ReadCR(testCtx, testSG3.Name, "", testSG3Result))
		assert.Equal(t, apiV1.StorageGroupPhaseInvalid, testSG3Result.Status.Phase)

		assert.Nil(t, storageGroupController.client.DeleteCR(testCtx, testSG3))
	})

	t.Run("reconcile storage group with error", func(t *testing.T) {
		testSG3 := sg3.DeepCopy()

		res, err := storageGroupController.reconcileStorageGroup(testCtx, testSG3)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{Requeue: true}, res)

		testSG3.Finalizers = append(testSG3.Finalizers, sgFinalizer)
		res, err = storageGroupController.reconcileStorageGroup(testCtx, testSG3)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{Requeue: true}, res)

		testSG1 := sg1.DeepCopy()
		testSG1.Finalizers = append(testSG1.Finalizers, sgFinalizer)
		res, err = storageGroupController.reconcileStorageGroup(testCtx, testSG1)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{Requeue: true}, res)

		testSG1.DeletionTimestamp = &v1.Time{Time: time.Now()}
		testSG1.Finalizers = append(testSG1.Finalizers, sgFinalizer)
		res, err = storageGroupController.reconcileStorageGroup(testCtx, testSG1)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{Requeue: true}, res)
	})
}