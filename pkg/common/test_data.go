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

package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dell/csi-baremetal/pkg/base/k8s"
	corev1 "k8s.io/api/core/v1"

	"github.com/sirupsen/logrus"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

var (
	testNS     = "default"
	testLogger = logrus.New()

	testCtx       = context.Background()
	testNode1Name = "node1"
	testNode2Name = "node2"

	testApp = "app"

	// Drives variables
	testDrive1UUID = "drive1-uuid"
	testDrive2UUID = "drive2-uuid"
	testDrive4UUID = "drive4-uuid"

	testAPIDrive4 = api.Drive{
		UUID:     testDrive4UUID,
		Type:     apiV1.DriveTypeHDD,
		Size:     1024 * 1024,
		IsSystem: false,
		NodeId:   testNode2Name,
	}

	testDriveTypeMeta = k8smetav1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version}
	testDriveCR4      = drivecrd.Drive{
		TypeMeta:   testDriveTypeMeta,
		ObjectMeta: k8smetav1.ObjectMeta{Name: testDrive4UUID},
		Spec:       testAPIDrive4,
	}

	// Available Capacity variables
	testAC1Name = fmt.Sprintf("%s-%s", testNode1Name, strings.ToLower(testDrive2UUID))
	testAC1     = &accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC1Name},
		Spec: api.AvailableCapacity{
			Location:     testDrive1UUID,
			NodeId:       testNode1Name,
			StorageClass: apiV1.StorageClassHDD,
			Size:         int64(util.GBYTE) * 42,
		},
	}

	testAC4Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDrive4UUID))
	testAC4     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC4Name},
		Spec: api.AvailableCapacity{
			Size:         testLVG.Spec.Size,
			StorageClass: apiV1.StorageClassHDDLVG,
			Location:     testLVGName,
			NodeId:       testNode2Name,
		},
	}

	// LogicalVolumeGroup variables
	testLVGName = "lvg-1"
	testLVG     = lvgcrd.LogicalVolumeGroup{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "LogicalVolumeGroup", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testLVGName, Namespace: ""},
		Spec: api.LogicalVolumeGroup{
			Name:      testLVGName,
			Node:      testNode2Name,
			Locations: []string{testDrive4UUID},
			Size:      int64(util.GBYTE) * 90,
			Status:    apiV1.Creating,
		},
	}

	// Volumes variables
	testVolume1Name = "aaaa-1111"
	testVolume1     = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testVolume1Name,
			Namespace:         testNS,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:                testVolume1Name,
			Location:          testAC1.Spec.Location,
			StorageClass:      testAC1.Spec.StorageClass,
			NodeId:            testAC1.Spec.NodeId,
			Size:              testAC1.Spec.Size,
			CSIStatus:         apiV1.Creating,
			Health:            apiV1.HealthGood,
			LocationType:      apiV1.LocationTypeDrive,
			OperationalStatus: apiV1.OperationalStatusOperative,
			Usage:             apiV1.VolumeUsageInUse,
		},
	}

	testVolumeLVG1Name = "aaaa-lvg-1111"
	testVolumeLVG1     = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testVolumeLVG1Name,
			Namespace:         testNS,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:                testVolumeLVG1Name,
			Location:          testAC4.Spec.Location,
			StorageClass:      testAC4.Spec.StorageClass,
			NodeId:            testAC4.Spec.NodeId,
			Size:              testAC4.Spec.Size / 2,
			CSIStatus:         apiV1.Creating,
			Health:            apiV1.HealthGood,
			LocationType:      apiV1.LocationTypeLVM,
			OperationalStatus: apiV1.OperationalStatusOperative,
			Usage:             apiV1.VolumeUsageInUse,
		},
	}

	testVolumeLVG2Name = "aaaa-lvg-2222"

	// PVC variables
	testPVC1 = &corev1.PersistentVolumeClaim{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      testVolume1.Name,
			Namespace: testNS,
			Labels: map[string]string{
				k8s.AppLabelKey: testApp,
			},
		},
	}
)
