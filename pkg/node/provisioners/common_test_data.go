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

package provisioners

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
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

	testVolume1Raw = api.Volume{
		Id:           testV1ID,
		NodeId:       testNodeID,
		Location:     testAPILVG.Name,
		StorageClass: apiV1.StorageClassHDD,
		Mode:         apiV1.ModeRAW,
	}

	testVolume1RawPart = api.Volume{
		Id:           testV1ID,
		NodeId:       testNodeID,
		Location:     testAPILVG.Name,
		StorageClass: apiV1.StorageClassHDD,
		Mode:         apiV1.ModeRAWPART,
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
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAPIDrive.UUID},
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

	testVolume2Raw = api.Volume{ // points on testDriveCR
		Id:           testV2ID,
		NodeId:       testNodeID,
		Location:     testDriveCR.Name,
		StorageClass: apiV1.StorageClassHDD,
		Mode:         apiV1.ModeRAW,
	}

	testVolume2RawPart = api.Volume{ // points on testDriveCR
		Id:           testV2ID,
		NodeId:       testNodeID,
		Location:     testDriveCR.Name,
		StorageClass: apiV1.StorageClassHDD,
		Mode:         apiV1.ModeRAWPART,
	}
)
