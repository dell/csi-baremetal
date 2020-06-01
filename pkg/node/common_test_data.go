package node

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
)

// here locates variables that used in UTs for CSINodeService and VolumeMgr

var (
	testNs      = "default"
	testID      = "volume-1-id"
	volLVGName  = "volume-lvg"
	lvgName     = "lvg-cr-1"
	driveUUID   = "drive-uuid"
	nodeID      = "fake-node"
	targetPath  = "/tmp/targetPath"
	stagePath   = "/tmp/stagePath"
	testPodName = "pod-1"

	testLogger  = getTestLogger()
	testCtx     = context.Background()
	disk1       = api.Drive{UUID: uuid.New().String(), SerialNumber: "hdd1", Size: 1024 * 1024 * 1024 * 500, NodeId: nodeID}
	disk2       = api.Drive{UUID: uuid.New().String(), SerialNumber: "hdd2", Size: 1024 * 1024 * 1024 * 200, NodeId: nodeID}
	testAC1Name = fmt.Sprintf("%s-%s", nodeID, strings.ToLower(disk1.UUID))
	testAC1     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC1Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:         1024 * 1024 * 1024 * 1024,
			StorageClass: apiV1.StorageClassHDD,
			Location:     disk1.UUID,
			NodeId:       nodeID,
		},
	}
	testAC2Name = fmt.Sprintf("%s-%s", nodeID, strings.ToLower(disk2.UUID))
	testAC2     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC2Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:         1024 * 1024 * 1024,
			StorageClass: apiV1.StorageClassHDD,
			Location:     disk2.UUID,
			NodeId:       nodeID,
		},
	}
	// volumes
	testV1ID = "volume-1-id"
	testV2ID = "volume-2-id"
	testV3ID = "volume-3-id"

	testVolume1 = api.Volume{Id: testV1ID, NodeId: nodeID, Location: disk1.UUID, StorageClass: apiV1.StorageClassHDD}
	testVolume2 = api.Volume{Id: testV2ID, NodeId: nodeID, Location: disk2.UUID}
	testVolume3 = api.Volume{Id: testV3ID, NodeId: nodeID, Location: ""}

	testVolumeCR1 = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testVolume1.Id,
			Namespace:         testNs,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: testVolume1,
	}
	testVolumeCR2 = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testVolume2.Id,
			Namespace:         testNs,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: testVolume2,
	}
	testVolumeCR3 = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testVolume3.Id,
			Namespace:         testNs,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: testVolume3,
	}

	testVolumeCap = &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{
				FsType: "xfs",
			},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}
)

// assertLenVListItemsEqualsTo read volumes CR List and assert it len is equals to expected, used t for asserting
func assertLenVListItemsEqualsTo(t *testing.T, k8sClient *k8s.KubeClient, expected int) {
	assert.Equal(t, expected, len(getVolumeCRsListItems(t, k8sClient)))
}

func getVolumeCRsListItems(t *testing.T, k8sClient *k8s.KubeClient) []vcrd.Volume {
	vList := &vcrd.VolumeList{}
	err := k8sClient.ReadList(testCtx, vList)
	assert.Nil(t, err)
	return vList.Items
}

func getTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}
