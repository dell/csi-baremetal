package drive

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	dcrd "github.com/dell/csi-baremetal/api/v1/drivecrd"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/logger/objects"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/dell/csi-baremetal/pkg/mocks"
)

var (
	testNs = "default"
	testID = "someID"
	nodeID = "node-uuid"

	testLogger = logrus.New()
	testCtx    = context.Background()
	driveUUID  = uuid.New().String()
	driveUUID2 = uuid.New().String()

	drive1 = api.Drive{
		UUID:     driveUUID,
		Size:     1024 * 1024 * 1024 * 500,
		NodeId:   nodeID,
		Type:     apiV1.DriveTypeHDD,
		Status:   apiV1.DriveStatusOnline,
		Health:   apiV1.HealthBad,
		IsSystem: true,
	}

	testBadCRDrive = dcrd.Drive{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: driveUUID},
		Spec:       drive1,
	}

	failedVolCR = vcrd.Volume{
		TypeMeta: v1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{
			Name:              testID,
			Namespace:         testNs,
			CreationTimestamp: v1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:           testID,
			StorageClass: apiV1.StorageClassHDD,
			Location:     driveUUID,
			CSIStatus:    apiV1.Creating,
			NodeId:       nodeID,
			Usage:        apiV1.VolumeUsageFailed,
		},
	}

	acCR = accrd.AvailableCapacity{
		TypeMeta:   v1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: driveUUID, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:         drive1.Size,
			StorageClass: apiV1.StorageClassHDD,
			Location:     "drive-uuid",
			NodeId:       nodeID},
	}
)

func setup() *k8s.KubeClient {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	return kubeClient
}

func TestDriveController_NewDriveController(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.client)
	assert.NotNil(t, dc.crHelper)
	assert.Nil(t, dc.driveMgrClient)
	assert.NotNil(t, dc.eventRecorder)
	assert.NotNil(t, dc.log)
	assert.Equal(t, dc.nodeID, nodeID)
	assert.NotEqual(t, dc.log, testLogger)
}

func TestDriveController_ChangeVolumeUsageAfterActionAnnotation(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	failedVolCR.Annotations = map[string]string{"release": "failed"}

	expectedV := failedVolCR.DeepCopy()
	expectedD := testBadCRDrive.DeepCopy()
	assert.NotNil(t, expectedD)
	assert.NotNil(t, expectedV)

	err := dc.client.CreateCR(testCtx, expectedV.Name, expectedV)
	assert.Nil(t, err)

	t.Run("Fail update", func(t *testing.T) {
		err = dc.changeVolumeUsageAfterActionAnnotation(k8s.UpdateFailCtx, dc.log, expectedD)
		assert.NotNil(t, err)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.NotNil(t, resultVolume[0].Spec)
		assert.NotEmpty(t, resultVolume[0].Annotations)
	})
	t.Run("Fail in GetVolumesByLocation", func(t *testing.T) {
		err = dc.changeVolumeUsageAfterActionAnnotation(k8s.ListFailCtx, dc.log, expectedD)
		assert.NotNil(t, err)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.NotNil(t, resultVolume[0].Spec)
		assert.NotEmpty(t, resultVolume[0].Annotations)
	})
	t.Run("Success Volume Usage change", func(t *testing.T) {
		err = dc.changeVolumeUsageAfterActionAnnotation(testCtx, dc.log, expectedD)
		assert.Nil(t, err)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.NotNil(t, resultVolume[0].Spec)
		assert.Empty(t, resultVolume[0].Annotations)
		assert.NotEqual(t, failedVolCR.Spec, resultVolume[0].Spec)
		assert.Equal(t, resultVolume[0].Spec.Usage, apiV1.DriveUsageInUse)
	})
}

func TestDriveController_Reconcile(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("Fail when try to read driveCR", func(t *testing.T) {
		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: failedVolCR.Name}}
		assert.NotNil(t, req)

		res, err := dc.Reconcile(k8s.GetFailCtx, req)
		assert.NotNil(t, res)
		assert.NotNil(t, err)

	})
	t.Run("Get err from handleDriveUpdate", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleasing
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: expectedD.Name}}
		assert.NotNil(t, req)

		res, err := dc.Reconcile(k8s.ListFailCtx, req)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, res, ctrl.Result{RequeueAfter: base.DefaultRequeueForVolume})

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("Get Update request from handleDriveUpdate", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleased
		expectedD.Annotations = map[string]string{"removal": "ready"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Annotations = map[string]string{"removal": "ready"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: expectedD.Name}}
		assert.NotNil(t, req)

		res, err := dc.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("Get Update request from handleDriveUpdate - ctx fail ", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleased
		expectedD.Annotations = map[string]string{"removal": "ready"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Annotations = map[string]string{"removal": "ready"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: expectedD.Name}}
		assert.NotNil(t, req)

		res, err := dc.Reconcile(k8s.UpdateFailCtx, req)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("Get Wait request from handleDriveUpdate", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageRemoving
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Spec.CSIStatus = apiV1.Created
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNs, Name: expectedD.Name}}
		assert.NotNil(t, req)

		res, err := dc.Reconcile(testCtx, req)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, ctrl.Result{RequeueAfter: base.DefaultTimeoutForVolumeUpdate}, res)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
}

func TestDriveController_handleDriveUpdate(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("Fail in DriveStatus", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		res, err := dc.handleDriveUpdate(k8s.ListFailCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, res, ignore)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("Success with drive in InUse Usage", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageInUse
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, update)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageReleasing)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("ReleasingUsage volumes without annotations - ignore branch", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleasing
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, ignore)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageReleasing)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("ReleasingUsage get volumes by location fail", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleasing
		assert.Nil(t, dc.client.CreateCR(k8s.ListFailCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		assert.Nil(t, dc.client.CreateCR(k8s.ListFailCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(k8s.ListFailCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, err, errors.New("raise list error"))
		assert.Equal(t, res, ignore)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageReleasing)

		assert.Nil(t, dc.client.DeleteCR(k8s.ListFailCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(k8s.ListFailCtx, expectedV))
	})
	t.Run("ReleasedUsage - fail in GetVolumesByLocation", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleased
		assert.Nil(t, dc.client.CreateCR(k8s.ListFailCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		assert.Nil(t, dc.client.CreateCR(k8s.ListFailCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(k8s.ListFailCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, err, errors.New("raise list error"))
		assert.Equal(t, res, ignore)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageReleased)

		assert.Nil(t, dc.client.DeleteCR(k8s.ListFailCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(k8s.ListFailCtx, expectedV))
	})
	t.Run("ReleasedUsage - has drive removalReady annotation", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleased
		expectedD.Annotations = map[string]string{"removal": "ready"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Annotations = map[string]string{"removal": "ready"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, update)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageRemoving)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("ReleasedUsage - has drive actionAdd annotation", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageReleased
		expectedD.Annotations = map[string]string{"action": "add"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, update)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageInUse)
		assert.Empty(t, expectedD.Annotations)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("ReleasedUsage - update drive annotations", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Annotations = map[string]string{"removal": "ready"}
		expectedD.Spec.Usage = apiV1.DriveUsageReleased
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Annotations = map[string]string{"removal": "replacement"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, update)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageRemoving)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.NotEmpty(t, resultVolume[0].Annotations)
		assert.Equal(t, resultVolume[0].Annotations[apiV1.DriveAnnotationRemoval], apiV1.DriveAnnotationRemovalReady)
		assert.Equal(t, resultVolume[0].Annotations[apiV1.DriveAnnotationReplacement], apiV1.DriveAnnotationRemovalReady)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("ReleasedUsage - fail on update drive annotations", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Annotations = map[string]string{"removal": "ready"}
		expectedD.Spec.Usage = apiV1.DriveUsageReleased
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Annotations = map[string]string{"removal": "replacement"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(k8s.UpdateFailCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, res, ignore)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageRemoving)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.Equal(t, resultVolume[0].Annotations[apiV1.DriveAnnotationRemoval], "replacement")
		assert.Empty(t, resultVolume[0].Annotations[apiV1.DriveAnnotationReplacement])

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("RemovedUsage - fail - DriveStatusOnline", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageRemoved
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, ignore)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("FailedUsage - update ctx fail", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Annotations = map[string]string{"action": "add"}
		expectedD.Spec.Usage = apiV1.DriveUsageFailed
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Spec.Usage = apiV1.VolumeUsageFailed
		expectedV.Annotations = map[string]string{"release": "failed"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(k8s.UpdateFailCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, res, ignore)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageInUse)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.Equal(t, expectedV.Spec.Usage, apiV1.VolumeUsageFailed)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("FailedUsage - success update", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Annotations = map[string]string{"action": "add"}
		expectedD.Spec.Usage = apiV1.DriveUsageFailed
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Spec.Usage = apiV1.VolumeUsageFailed
		expectedV.Annotations = map[string]string{"release": "failed"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, update)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageInUse)
		assert.Empty(t, expectedD.Annotations)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.Equal(t, resultVolume[0].Spec.Usage, apiV1.VolumeUsageInUse)
		assert.Empty(t, resultVolume[0].Annotations)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("FailedUsage - fail - invalid annotation value", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Annotations = map[string]string{"action": "test-remove"}
		expectedD.Spec.Usage = apiV1.DriveUsageFailed
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		res, err := dc.handleDriveUpdate(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, ignore)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
}

func TestDriveController_handleDriveStatus(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("Fail when try to read volumelist", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)

		err := dc.handleDriveStatus(k8s.ListFailCtx, expectedD)
		assert.NotNil(t, err)
		assert.Equal(t, expectedD.Spec.UUID, driveUUID)
		assert.Equal(t, expectedD.Spec.Status, apiV1.DriveStatusOnline)
		assert.Equal(t, expectedD.Spec.Health, apiV1.HealthBad)
		assert.Empty(t, expectedD.Spec.Usage)
	})
	t.Run("Fail with onlineStatus in updateVolume step", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Spec.OperationalStatus = apiV1.OperationalStatusMissing
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		err := dc.handleDriveStatus(k8s.UpdateFailCtx, expectedD)
		assert.NotNil(t, err)
		assert.Equal(t, expectedD.Spec.UUID, driveUUID)
		assert.Equal(t, expectedD.Spec.Status, apiV1.DriveStatusOnline)
		assert.Equal(t, expectedD.Spec.Health, apiV1.HealthBad)
		assert.Empty(t, expectedD.Spec.Usage)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.NotNil(t, resultVolume[0].Spec)
		assert.Equal(t, resultVolume[0].Spec.OperationalStatus, apiV1.OperationalStatusMissing)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("Fail with offlineStatus", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Status = apiV1.DriveStatusOffline
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		err := dc.handleDriveStatus(k8s.UpdateFailCtx, expectedD)
		assert.NotNil(t, err)
		assert.Equal(t, expectedD.Spec.UUID, driveUUID)
		assert.Equal(t, expectedD.Spec.Status, apiV1.DriveStatusOffline)
		assert.Empty(t, expectedD.Spec.Usage)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.Empty(t, resultVolume[0].Spec.OperationalStatus)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
	t.Run("Success with Online status", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Spec.OperationalStatus = apiV1.OperationalStatusMissing
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		err := dc.handleDriveStatus(testCtx, expectedD)
		assert.Nil(t, err)
		assert.Equal(t, expectedD.Spec.UUID, driveUUID)
		assert.Equal(t, expectedD.Spec.Status, apiV1.DriveStatusOnline)
		assert.Equal(t, expectedD.Spec.Health, apiV1.HealthBad)
		assert.Empty(t, expectedD.Spec.Usage)

		resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
		assert.Nil(t, err)
		assert.NotNil(t, resultVolume)
		assert.NotNil(t, resultVolume[0].Spec)
		assert.Equal(t, resultVolume[0].Spec.OperationalStatus, apiV1.OperationalStatusOperative)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedV))
	})
}

func TestDriveController_checkAndPlaceStatusInUse(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("Success - has driveActionAnnotationKey", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageFailed
		expectedD.Annotations = map[string]string{"action": "add"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		status := dc.checkAndPlaceStatusInUse(expectedD)
		assert.True(t, status)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageInUse)
		assert.Empty(t, expectedD.Annotations)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("Success - has deprecated annotation", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageFailed
		expectedD.Annotations = map[string]string{"drive": "add"}
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		status := dc.checkAndPlaceStatusInUse(expectedD)
		assert.True(t, status)
		assert.Equal(t, expectedD.Spec.Usage, apiV1.DriveUsageInUse)
		assert.Empty(t, expectedD.Annotations)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("Fail - without annotation", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		status := dc.checkAndPlaceStatusInUse(expectedD)
		assert.False(t, status)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
}

func TestDriveController_checkAndPlaceStatusRemoved(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("Fail - without annotation key", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		status := dc.checkAndPlaceStatusRemoved(expectedD)
		assert.False(t, status)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
}

func TestDriveController_handleDriveUsageRemoving(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	dc.driveMgrClient = &mocks.MockDriveMgrClient{}
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("Fail when try to read volumes", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		res, err := dc.handleDriveUsageRemoving(k8s.ListFailCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.NotNil(t, err)
		assert.Equal(t, res, ignore)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("Get wait when try to check volumes", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		expectedD.Spec.Usage = apiV1.DriveUsageRemoving
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedD.Name, expectedD))

		expectedV := failedVolCR.DeepCopy()
		assert.NotNil(t, expectedV)
		expectedV.Spec.CSIStatus = apiV1.Created
		assert.Nil(t, dc.client.CreateCR(testCtx, expectedV.Name, expectedV))

		res, err := dc.handleDriveUsageRemoving(testCtx, dc.log, expectedD)
		assert.NotNil(t, res)
		assert.Nil(t, err)
		assert.Equal(t, res, wait)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
}

func TestDriveController_removeRelatedAC(t *testing.T) {
	kubeClient := setup()
	dc := NewController(kubeClient, nodeID, nil, new(events.Recorder), testLogger)
	dc.driveMgrClient = &mocks.MockDriveMgrClient{}
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("AC not exist in this location", func(t *testing.T) {
		accr := acCR.DeepCopy()
		assert.Nil(t, dc.client.CreateCR(testCtx, accr.Name, accr))

		err := dc.removeRelatedAC(testCtx, dc.log, &testBadCRDrive)
		assert.Nil(t, err)

		ac, err := dc.crHelper.GetACByLocation(testBadCRDrive.GetName())
		assert.Nil(t, ac)
		assert.Equal(t, err, errTypes.ErrorNotFound)
		assert.Nil(t, dc.client.DeleteCR(testCtx, accr))
	})
	t.Run("AC exist in this location", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		err := dc.client.CreateCR(testCtx, expectedD.Name, expectedD)
		assert.Nil(t, err)

		accr := acCR.DeepCopy()
		accr.Spec.Location = expectedD.Name
		assert.Nil(t, dc.client.CreateCR(testCtx, accr.Name, accr))

		err = dc.removeRelatedAC(testCtx, dc.log, expectedD)
		assert.Nil(t, err)

		ac, err := dc.crHelper.GetACByLocation(expectedD.GetName())
		assert.Nil(t, ac)
		assert.NotNil(t, err)
		assert.Equal(t, err, errTypes.ErrorNotFound)

		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
	t.Run("AC failed when try to deleteCR", func(t *testing.T) {
		expectedD := testBadCRDrive.DeepCopy()
		assert.NotNil(t, expectedD)
		err := dc.client.CreateCR(testCtx, expectedD.Name, expectedD)
		assert.Nil(t, err)

		accr := acCR.DeepCopy()
		accr.Spec.Location = expectedD.Name
		assert.Nil(t, dc.client.CreateCR(testCtx, accr.Name, accr))

		err = dc.removeRelatedAC(k8s.DeleteFailCtx, dc.log, expectedD)
		assert.NotNil(t, err)

		ac, err := dc.crHelper.GetACByLocation(expectedD.GetName())
		assert.NotNil(t, ac)
		assert.Nil(t, err)
		assert.NotEqual(t, err, errTypes.ErrorNotFound)

		assert.Nil(t, dc.client.DeleteCR(testCtx, accr))
		assert.Nil(t, dc.client.DeleteCR(testCtx, expectedD))
	})
}

func TestDriveController_stopLocateNodeLED(t *testing.T) {
	mockK8sClient := &mocks.K8Client{}
	kubeClient := k8s.NewKubeClient(mockK8sClient, testLogger, objects.NewObjectLogger(), testNs)
	dc := NewController(kubeClient, nodeID, &mocks.MockDriveMgrClient{}, new(events.Recorder), testLogger)
	assert.NotNil(t, dc)
	assert.NotNil(t, dc.crHelper)

	t.Run("Fail - locate node not implemented", func(t *testing.T) {
		mockK8sClient.On("List", mock.Anything, &dcrd.DriveList{}, mock.Anything).Return(nil)
		err := dc.stopLocateNodeLED(testCtx, dc.log, &testBadCRDrive)
		assert.NotNil(t, err)
	})
}
