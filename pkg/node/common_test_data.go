/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package node

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dell/csi-baremetal/api/v1/drivecrd"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

// here locates variables that used in UTs for CSINodeService and VolumeMgr

var (
	testNs      = "default"
	testID      = "volume-1-id"
	volLVGName  = "volume-lvg"
	testLVGName = "lvg-cr-1"
	driveUUID   = "drive-uuid"
	nodeID      = "fake-node"
	nodeName    = "fake-node-name"
	targetPath  = "/tmp/targetPath"
	stagePath   = "/tmp/stagePath"
	testPodName = "pod-1"

	testLogger  = getTestLogger()
	testCtx     = context.Background()
	disk1       = api.Drive{UUID: uuid.New().String(), SerialNumber: "hdd1", Size: 1024 * 1024 * 1024 * 500, NodeId: nodeID, Path: "/dev/sda"}
	disk2       = api.Drive{UUID: uuid.New().String(), SerialNumber: "hdd2", Size: 1024 * 1024 * 1024 * 200, NodeId: nodeID, Path: "/dev/sda"}
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

	testVolume1 = api.Volume{
		Id:           testV1ID,
		NodeId:       nodeID,
		Location:     disk1.UUID,
		StorageClass: apiV1.StorageClassHDD,
		CSIStatus:    apiV1.VolumeReady,
	}
	testVolume2 = api.Volume{
		Id:           testV2ID,
		NodeId:       nodeID,
		Location:     disk2.UUID,
		StorageClass: apiV1.StorageClassHDD,
		CSIStatus:    apiV1.Created,
	}
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

func getDriveCRsListItems(t *testing.T, k8sClient *k8s.KubeClient) []drivecrd.Drive {
	dList := &drivecrd.DriveList{}
	assert.Nil(t, k8sClient.ReadList(testCtx, dList))
	return dList.Items
}

func getTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}
