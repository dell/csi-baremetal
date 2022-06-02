package drive 

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/dell/csi-baremetal/pkg/events"
	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	dcrd "github.com/dell/csi-baremetal/api/v1/drivecrd"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
)

var (
	testNs = "default"
	testID =  "someID"
	nodeID = "node-uuid"
	
	testLogger = logrus.New()
	testCtx = context.Background()
	driveUUID = uuid.New().String()
	
	testBadCRDrive = dcrd.Drive{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: driveUUID},
		Spec: api.Drive{
			UUID:         driveUUID,
			Size:         1024 * 1024 * 1024 * 500,
			NodeId:       nodeID,
			Type:         apiV1.DriveTypeHDD,
			Status:       apiV1.DriveStatusOnline,
			Health:       apiV1.HealthBad,
			IsSystem:     true,
		},
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

)

func setup() *k8s.KubeClient{
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	return kubeClient
}


func TestDriveController_NewDriveController(t *testing.T){
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

func TestDriveController_ChangeVolumeUsageAfterActionAnnotation(t *testing.T){
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

	err = dc.changeVolumeUsageAfterActionAnnotation(testCtx, dc.log, expectedD)
	assert.Nil(t, err) 
	
	resultVolume, err := dc.crHelper.GetVolumesByLocation(testCtx, driveUUID)
	assert.Nil(t, err)
	assert.NotNil(t, resultVolume)
	assert.NotNil(t, resultVolume[0].Spec)
	assert.Empty(t, resultVolume[0].Annotations)
	assert.NotEqual(t, failedVolCR.Spec, resultVolume[0].Spec)
	assert.Equal(t, resultVolume[0].Spec.Usage, apiV1.DriveUsageInUse)
}
