/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package capacitycontroller

import (
	"context"
	"strconv"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

var (
	tCtx       = context.Background()
	testLogger = logrus.New()
	lvg1Name   = "lvg-cr-1"
	drive1UUID = "uuid-drive1"

	ns      = "default"
	node1ID = "node1"

	apiDrive1 = api.Drive{
		UUID:         drive1UUID,
		VID:          "vid-drive1",
		PID:          "pid-drive1",
		SerialNumber: "hdd1", // depend on commands.LsblkTwoDevicesStr - /dev/sda
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         int64(1000 * util.GBYTE),
		Status:       apiV1.DriveStatusOnline,
		Usage:        apiV1.DriveUsageInUse,
		NodeId:       node1ID,
		IsClean:      true,
	}

	drive1CR = drivecrd.Drive{
		TypeMeta: v1.TypeMeta{
			Kind:       "Drive",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: drive1UUID,
		},
		Spec: apiDrive1,
	}

	lvgCR1 = lvgcrd.LogicalVolumeGroup{
		TypeMeta: v1.TypeMeta{
			Kind:       "LogicalVolumeGroup",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: lvg1Name,
		},
		Spec: api.LogicalVolumeGroup{
			Name:      lvg1Name,
			Node:      node1ID,
			Locations: []string{apiDrive1.UUID},
			Size:      int64(1024 * 500 * util.GBYTE),
			Status:    apiV1.Creating,
			Health:    apiV1.HealthGood,
		},
	}

	acSpec = api.AvailableCapacity{
		Location:     drive1UUID,
		NodeId:       apiDrive1.NodeId,
		StorageClass: apiDrive1.Type,
		Size:         apiDrive1.Size,
	}
	acCRName = "ac"
	acCR     = accrd.AvailableCapacity{
		TypeMeta: v1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      acCRName,
			Namespace: ns,
		},
		Spec: acSpec,
	}

	acSpec2 = api.AvailableCapacity{
		Location:     lvg1Name,
		NodeId:       lvgCR1.Spec.Node,
		StorageClass: apiV1.StorageClassSystemLVG,
		Size:         lvgCR1.Spec.Size,
	}
	acCR1Name = "ac1"
	acCR1     = accrd.AvailableCapacity{
		TypeMeta: v1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      acCR1Name,
			Namespace: ns,
		},
		Spec: acSpec2,
	}
)

func Test_NewLVGController(t *testing.T) {
	c := NewCapacityController(nil, nil, testLogger)
	assert.NotNil(t, c)
}

func TestController_ReconcileDrive(t *testing.T) {
	t.Run("Drive is good, AC is not present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR.DeepCopy()
		err = kubeClient.Create(tCtx, testDrive)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testDrive.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, acSpec, acList.Items[0].Spec)
	})

	t.Run("Drive is good, AC is present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		testAC := acCR
		err = kubeClient.Create(tCtx, &testAC)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testDrive.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, acSpec, acList.Items[0].Spec)
	})

	t.Run("Drive is bad, AC is not present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive.Spec.Health = apiV1.HealthBad
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testDrive.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(acList.Items))
	})

	t.Run("Drive is bad, AC is present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive.Spec.Health = apiV1.HealthBad
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		testAC := acCR
		err = kubeClient.Create(tCtx, &testAC)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testDrive.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, int64(0), acList.Items[0].Spec.Size)
	})
	t.Run("Drive is good and not clean, AC is present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive.Spec.IsClean = false
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		testAC := acCR
		err = kubeClient.Create(tCtx, &testAC)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testDrive.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, int64(0), acList.Items[0].Spec.Size)
	})
	t.Run("Drive is good and not clean, AC is not present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive.Spec.IsClean = false
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testDrive.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, int64(0), acList.Items[0].Spec.Size)
	})
}

func TestController_ReconcileLVG(t *testing.T) {
	t.Run("LVG is good, lvg doesn't have annotation", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		err = kubeClient.Create(tCtx, &testLVG)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testLVG.Name}})
		assert.Nil(t, err)
	})

	t.Run("LVG is good, Annotation is present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive.Spec.IsSystem = true
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		testAC := acCR
		err = kubeClient.Create(tCtx, &testAC)
		assert.Nil(t, err)
		testLVG := lvgCR1
		testLVG.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: strconv.FormatInt(int64(util.GBYTE), 10)}
		err = kubeClient.Create(tCtx, &testLVG)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testLVG.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, int64(util.GBYTE), acList.Items[0].Spec.Size)
	})
	t.Run("LVG is good, Annotation is present, wrong annotation value", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR.DeepCopy()
		testDrive.Spec.IsSystem = true
		err = kubeClient.Create(tCtx, testDrive)
		assert.Nil(t, err)
		testLVG := lvgCR1.DeepCopy()
		testLVG.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: "error"}
		err = kubeClient.Create(tCtx, testLVG)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testLVG.Name}})
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid syntax")
	})
	t.Run("LVG is bad, AC is not present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		testLVG.Spec.Health = apiV1.HealthBad
		err = kubeClient.Create(tCtx, &testLVG)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testLVG.Name}})
		assert.Nil(t, err)
	})
	t.Run("LVG is bad, AC is present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testAC := acCR1
		err = kubeClient.Create(tCtx, &testAC)
		assert.Nil(t, err)
		testLVG := lvgCR1
		testLVG.Spec.Health = apiV1.HealthBad
		err = kubeClient.Create(tCtx, &testLVG)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testLVG.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, int64(0), acList.Items[0].Spec.Size)
	})
	t.Run("LVG is good, AC is not present", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive.Spec.IsSystem = true
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		testLVG := lvgCR1
		testLVG.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: strconv.FormatInt(int64(util.GBYTE), 10)}
		err = kubeClient.Create(tCtx, &testLVG)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testLVG.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, int64(util.GBYTE), acList.Items[0].Spec.Size)
		assert.Equal(t, apiV1.StorageClassSystemLVG, acList.Items[0].Spec.StorageClass)
	})
	t.Run("LVG is good, AC is present for drive", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive.Spec.IsSystem = true
		err = kubeClient.Create(tCtx, &testDrive)
		assert.Nil(t, err)
		testAC := acCR.DeepCopy()
		testAC.Spec.Location = testDrive.Name
		err = kubeClient.Create(tCtx, testAC)
		assert.Nil(t, err)
		testLVG := lvgCR1
		testLVG.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: strconv.FormatInt(int64(util.GBYTE), 10)}
		err = kubeClient.Create(tCtx, &testLVG)
		assert.Nil(t, err)
		_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: testLVG.Name}})
		assert.Nil(t, err)
		acList := &accrd.AvailableCapacityList{}
		err = kubeClient.ReadList(tCtx, acList)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(acList.Items))
		assert.Equal(t, int64(util.GBYTE), acList.Items[0].Spec.Size)
		assert.Equal(t, apiV1.StorageClassSystemLVG, acList.Items[0].Spec.StorageClass)
	})
}
func TestController_ReconcileResourcesNotFound(t *testing.T) {
	kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
	assert.Nil(t, err)
	controller := NewCapacityController(kubeClient, kubeClient, testLogger)
	assert.NotNil(t, controller)
	_, err = controller.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: drive1UUID}})
	assert.Nil(t, err)
}

func TestController_filterUpdateEvent_Drive(t *testing.T) {
	t.Run("Drives have different statuses", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive2 := drive1CR
		testDrive2.Spec.Health = apiV1.HealthBad
		assert.True(t, controller.filterUpdateEvent(&testDrive, &testDrive2))
	})
	t.Run("Drives have different health statuses", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive2 := drive1CR
		testDrive2.Spec.Status = apiV1.Failed
		assert.True(t, controller.filterUpdateEvent(&testDrive, &testDrive2))
	})
	t.Run("Drives have different clean", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive2 := drive1CR
		testDrive2.Spec.IsClean = !testDrive.Spec.IsClean
		assert.True(t, controller.filterUpdateEvent(&testDrive, &testDrive2))
	})
	t.Run("Drives are filtered", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testDrive := drive1CR
		testDrive2 := drive1CR
		assert.False(t, controller.filterUpdateEvent(&testDrive, &testDrive2))
	})
}

func TestController_filterUpdateEvent_LVG(t *testing.T) {
	t.Run("LVG have different health statuses", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		testLVG2 := lvgCR1
		testLVG2.Spec.Health = apiV1.HealthBad
		assert.True(t, controller.filterUpdateEvent(&testLVG, &testLVG2))
	})
	t.Run("LVG have different statuses", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		testLVG2 := lvgCR1
		testLVG2.Spec.Status = apiV1.Failed
		assert.True(t, controller.filterUpdateEvent(&testLVG, &testLVG2))
	})
	t.Run("Old lvg has annotation, new one doesn't", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		testLVG2 := lvgCR1
		testLVG.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: "10000"}
		assert.False(t, controller.filterUpdateEvent(&testLVG, &testLVG2))
	})
	t.Run("Old lvg doesn't annotation, new one does", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		testLVG2 := lvgCR1
		testLVG2.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: "10000"}
		assert.True(t, controller.filterUpdateEvent(&testLVG, &testLVG2))
	})
	t.Run("Old lvg doesn't annotation, new one doesn't have annotation", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		testLVG2 := lvgCR1
		assert.False(t, controller.filterUpdateEvent(&testLVG, &testLVG2))
	})
	t.Run("Both LVG have annotation", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(ns, testLogger)
		assert.Nil(t, err)
		controller := NewCapacityController(kubeClient, kubeClient, testLogger)
		assert.NotNil(t, controller)
		testLVG := lvgCR1
		testLVG2 := lvgCR1
		testLVG2.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: "10000"}
		testLVG.Annotations = map[string]string{apiV1.LVGFreeSpaceAnnotation: "5000"}
		assert.True(t, controller.filterUpdateEvent(&testLVG, &testLVG2))
	})
}
