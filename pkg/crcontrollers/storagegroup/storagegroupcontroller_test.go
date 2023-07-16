package storagegroup

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	sgcrd "github.com/dell/csi-baremetal/api/v1/storagegroupcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

var (
	testNs = "default"
	testID = "someID"
	nodeID = "node-uuid"

	testLogger         = logrus.New()
	testCtx            = context.Background()
	driveUUID          = uuid.New().String()
	driveUUID2         = uuid.New().String()
	driveSerialNumber  = "VDH19UBD"
	driveSerialNumber2 = "MDH16UAC"
	testSG1Name        = "test-hdd-1"

	drive1 = api.Drive{
		UUID:         driveUUID,
		Size:         1024 * 1024 * 1024 * 500,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthBad,
		IsSystem:     true,
		SerialNumber: driveSerialNumber,
	}

	drive2 = api.Drive{
		UUID:         driveUUID2,
		Size:         1024 * 1024 * 1024 * 500,
		NodeId:       nodeID,
		Type:         apiV1.DriveTypeHDD,
		Status:       apiV1.DriveStatusOnline,
		Health:       apiV1.HealthGood,
		SerialNumber: driveSerialNumber2,
	}

	testSG1 = sgcrd.StorageGroup{
		TypeMeta:   v1.TypeMeta{Kind: "StorageGroup", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{Name: testSG1Name},
		Spec: api.StorageGroupSpec{
			DriveSelector: &api.DriveSelector{
				NumberDrivesPerNode: 1,
				MatchFields:         map[string]string{"Type": "HDD"},
			},
		},
	}
)

func setup() *k8s.KubeClient {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	return kubeClient
}

func TestStorageGroupController_NewController(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)

	controller := NewController(kubeClient, kubeClient, testLogger)
	assert.NotNil(t, controller)
	assert.NotNil(t, controller.client)
	assert.NotNil(t, controller.crHelper)

	assert.NotNil(t, controller.log)
	assert.NotEqual(t, controller.log, testLogger)
}
