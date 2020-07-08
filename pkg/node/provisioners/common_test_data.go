package provisioners

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal.git/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal.git/api/v1"
	"github.com/dell/csi-baremetal.git/api/v1/drivecrd"
)

var (
	testNs     = "default"
	testLogger = logrus.New()
	testNodeID = "node1"
	errTest    = errors.New("error")
	testCtx    = context.Background()

	disk1 = api.Drive{
		UUID:         uuid.New().String(),
		SerialNumber: "hdd1",
		Size:         1024 * 1024 * 1024 * 500,
		NodeId:       testNodeID,
	}

	// prepare test data: LVG points on drive and Volume points on that LVG
	testAPILVG = api.LogicalVolumeGroup{
		Name:      uuid.New().String(),
		Node:      testNodeID,
		Locations: []string{disk1.UUID},
		Size:      1024,
	}

	testV1ID    = "volume-1-id"
	testVolume1 = api.Volume{
		Id:           testV1ID,
		NodeId:       testNodeID,
		Location:     testAPILVG.Name,
		StorageClass: apiV1.StorageClassHDD,
		Type:         "xfs",
	}

	// Volume CR that points on Drive CR
	testAPIDrive = api.Drive{
		UUID:         "drive1-uuid",
		SerialNumber: "drive1-sn",
		NodeId:       testNodeID,
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024,
		Status:       apiV1.DriveStatusOnline,
	}
	testDriveCR = drivecrd.Drive{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAPIDrive.UUID, Namespace: testNs},
		Spec:       testAPIDrive,
	}

	testV2ID    = "volume-2-id"
	testVolume2 = api.Volume{ // points on testDriveCR
		Id:           testV2ID,
		NodeId:       testNodeID,
		Location:     testDriveCR.Name,
		StorageClass: apiV1.StorageClassHDD,
		Type:         "xfs",
	}
)
