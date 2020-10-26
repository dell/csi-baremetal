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

package lvg

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	vccrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/mocks"
	mocklu "github.com/dell/csi-baremetal/pkg/mocks/linuxutils"
)

var (
	lsblkAllDevicesCmd = fmt.Sprintf(lsblk.CmdTmpl, "")
	tCtx               = context.Background()
	testLogger         = logrus.New()
	lvg1Name           = "lvg-cr-1"
	lvg2Name           = "lvg-cr-2"
	drive1UUID         = "uuid-drive1"
	drive2UUID         = "uuid-drive2"

	ns      = "default"
	node1ID = "node1"
	node2ID = "node2"

	apiDrive1 = api.Drive{
		UUID:         drive1UUID,
		VID:          "vid-drive1",
		PID:          "pid-drive1",
		SerialNumber: "hdd1", // depend on commands.LsblkTwoDevicesStr - /dev/sda
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         int64(1000 * util.GBYTE),
		Status:       apiV1.DriveStatusOnline,
		NodeId:       node1ID,
	}

	apiDrive2 = api.Drive{
		UUID:         drive2UUID,
		VID:          "vid-drive2",
		PID:          "pid-drive2",
		SerialNumber: "hdd2", // depend on commands.LsblkTwoDevicesStr - /dev/sdb
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         int64(333 * util.GBYTE),
		Status:       apiV1.DriveStatusOnline,
		NodeId:       node1ID,
	}

	drive1CR = drivecrd.Drive{
		TypeMeta: v1.TypeMeta{
			Kind:       "Drive",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      drive1UUID,
			Namespace: ns,
		},
		Spec: apiDrive1,
	}

	drive2CR = drivecrd.Drive{
		TypeMeta: v1.TypeMeta{
			Kind:       "Drive",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      drive2UUID,
			Namespace: ns,
		},
		Spec: apiDrive2,
	}

	lvgCR1 = lvgcrd.LVG{
		TypeMeta: v1.TypeMeta{
			Kind:       "LVG",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      lvg1Name,
			Namespace: ns,
		},
		Spec: api.LogicalVolumeGroup{
			Name:      lvg1Name,
			Node:      node1ID,
			Locations: []string{apiDrive1.UUID, apiDrive2.UUID},
			Size:      int64(1024 * 500 * util.GBYTE),
			Status:    apiV1.Creating,
		},
	}

	lvgCR2 = lvgcrd.LVG{
		TypeMeta: v1.TypeMeta{
			Kind:       "LVG",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      lvg2Name,
			Namespace: ns,
		},
		Spec: api.LogicalVolumeGroup{
			Name:      lvg2Name,
			Node:      node2ID,
			Locations: []string{},
			Size:      0,
			Status:    apiV1.Created,
		},
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
		Spec: api.AvailableCapacity{
			Location:     lvg1Name,
			NodeId:       node1ID,
			StorageClass: apiV1.StorageClassHDDLVG,
			Size:         int64(1024 * 300 * util.GBYTE),
		},
	}
	testVolume1 = api.Volume{
		Id:           "volume",
		NodeId:       node1ID,
		Location:     lvgCR1.Name,
		StorageClass: apiV1.StorageClassHDD,
		CSIStatus:    apiV1.VolumeReady,
	}

	testVolumeCR1 = vccrd.Volume{
		TypeMeta: v1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{
			Name:      testVolume1.Id,
			Namespace: ns,
		},
		Spec: testVolume1,
	}
)

func Test_NewLVGController(t *testing.T) {
	c := NewController(nil, "node", testLogger)
	assert.NotNil(t, c)
}

func TestReconcile_SuccessAnotherNode(t *testing.T) {
	c := setup(t, node1ID)
	defer teardown(t, c)
	// request for resources on another node
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR2.Name}}
	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func TestReconcile_SuccessNotFound(t *testing.T) {
	c := setup(t, node1ID)
	defer teardown(t, c)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "not-found-that-name"}}
	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func TestReconcile_SuccessCreatingLVG(t *testing.T) {
	var (
		c       = setup(t, node1ID)
		lvmOps  = &mocklu.MockWrapLVM{}
		listBlk = &mocklu.MockWrapLsblk{}
		req     = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
		lvg     = &lvgcrd.LVG{}
	)
	defer teardown(t, c)

	c.lvmOps = lvmOps
	c.listBlk = listBlk

	listBlk.On("SearchDrivePath", mock.Anything).Return("", nil)
	lvmOps.On("PVCreate", mock.Anything).Return(nil)
	lvmOps.On("VGCreate", mock.Anything, mock.Anything).Return(nil)

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = c.k8sClient.ReadCR(tCtx, req.Name, lvg)
	assert.Equal(t, apiV1.Created, lvg.Spec.Status)

	// reconciled second time
	res, err = c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	currLVG := &lvgcrd.LVG{}
	err = c.k8sClient.ReadCR(tCtx, req.Name, currLVG)
	assert.Contains(t, currLVG.ObjectMeta.Finalizers, lvgFinalizer)
}

func TestReconcile_SuccessDeletion(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		e   = &mocks.GoMockExecutor{}
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
	)
	defer teardown(t, c)

	c.lvmOps = lvm.NewLVM(e, testLogger)

	lvgToDell := lvgCR1
	lvgToDell.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	lvgToDell.ObjectMeta.Finalizers = []string{lvgFinalizer}
	err := c.k8sClient.UpdateCR(tCtx, &lvgToDell)

	e.OnCommand(fmt.Sprintf(lvm.LVsInVGCmdTmpl, lvgCR1.Name)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(lvm.VGRemoveCmdTmpl, lvgCR1.Name)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(lvm.PVsInVGCmdTmpl, lvm.EmptyName)).Return("", "", nil).Times(1)

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func TestReconcile_TryToDeleteLVGWithVolume(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
	)
	defer teardown(t, c)

	lvgToDell := lvgCR1
	lvgToDell.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	lvgToDell.ObjectMeta.Finalizers = []string{lvgFinalizer}
	err := c.k8sClient.UpdateCR(tCtx, &lvgToDell)
	assert.Nil(t, err)

	err = c.k8sClient.CreateCR(tCtx, testVolumeCR1.Name, &testVolumeCR1)
	assert.Nil(t, err)

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	lvg := &lvgcrd.LVG{}
	err = c.k8sClient.ReadCR(tCtx, lvgToDell.Name, lvg)
	assert.Nil(t, err)
	assert.NotNil(t, lvg)
	assert.Equal(t, lvgToDell.Name, lvg.Name)
}

func TestReconcile_DeletionFailed(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		e   = &mocks.GoMockExecutor{}
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
	)
	defer teardown(t, c)

	c.lvmOps = lvm.NewLVM(e, testLogger)

	lvgToDell := lvgCR1
	lvgToDell.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	lvgToDell.ObjectMeta.Finalizers = []string{lvgFinalizer}
	err := c.k8sClient.UpdateCR(tCtx, &lvgToDell)

	// expect that LVG still contains LV
	e.OnCommand(fmt.Sprintf(lvm.LVsInVGCmdTmpl, lvgCR1.Name)).Return("lv1", "", nil)

	res, err := c.Reconcile(req)
	assert.Contains(t, err.Error(), "there are LVs in LVG")
	assert.Equal(t, res, ctrl.Result{})
	fmt.Println(err)
}

func TestReconcile_FailedNoPVs(t *testing.T) {
	// expect that no one PVs were created
	var (
		c = setup(t, node1ID)
		e = &mocks.GoMockExecutor{}
		// SearchDrivePath failed for /dev/sdb
		lsblkResp = `{
			  "blockdevices":[{
				"name": "/dev/sda",
				"type": "disk",
				"serial": "hdd1"
				}]
			}`
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
	)
	defer teardown(t, c)

	c.lvmOps = lvm.NewLVM(e, testLogger)
	e.OnCommand(lsblkAllDevicesCmd).Return(lsblkResp, "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sda").Return("", "", errors.New("pvcreate failed"))

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	lvgCR := &lvgcrd.LVG{}
	err = c.k8sClient.ReadCR(tCtx, lvgCR1.Name, lvgCR)
	assert.Equal(t, apiV1.Failed, lvgCR.Spec.Status)
}

func TestReconcile_FailedVGCreate(t *testing.T) {
	var (
		c           = setup(t, node1ID)
		e           = &mocks.GoMockExecutor{}
		req         = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
		expectedErr = errors.New("vgcreate failed")
	)
	defer teardown(t, c)

	c.lvmOps = lvm.NewLVM(e, testLogger)
	e.OnCommand(lsblkAllDevicesCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sda").Return("", "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sdb").Return("", "", nil)
	e.OnCommand(fmt.Sprintf("/sbin/lvm vgcreate --yes %s /dev/sda /dev/sdb", req.Name)).
		Return("", "", expectedErr)

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	lvgCR := &lvgcrd.LVG{}
	err = c.k8sClient.ReadCR(tCtx, lvgCR1.Name, lvgCR)
	assert.Equal(t, apiV1.Failed, lvgCR.Spec.Status)
}

func Test_removeLVGArtifacts_Success(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		e   = &mocks.GoMockExecutor{}
		vg  = lvgCR1.Name
		err error
	)
	defer teardown(t, c)

	c.lvmOps = lvm.NewLVM(e, testLogger)

	e.OnCommand(fmt.Sprintf(lvm.LVsInVGCmdTmpl, lvgCR1.Name)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(lvm.VGRemoveCmdTmpl, vg)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(lvm.PVsInVGCmdTmpl, lvm.EmptyName)).Return("", "", nil).Times(1)
	err = c.removeLVGArtifacts(vg)
	assert.Nil(t, err)

	// expect that RemoveOrphanPVs failed and ignore it
	e.OnCommand(fmt.Sprintf(lvm.PVsInVGCmdTmpl, lvm.EmptyName)).
		Return("", "", errors.New("error")).Times(1)
	err = c.removeLVGArtifacts(vg)
	assert.Nil(t, err)
}

func Test_removeLVGArtifacts_Fail(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		e   = &mocks.GoMockExecutor{}
		vg  = lvgCR1.Name
		err error
	)
	defer teardown(t, c)

	c.lvmOps = lvm.NewLVM(e, testLogger)

	// expect that VG contains LV
	e.OnCommand(fmt.Sprintf(lvm.LVsInVGCmdTmpl, vg)).Return("some-lv1", "", nil).Times(1)
	err = c.removeLVGArtifacts(vg)
	assert.Equal(t, fmt.Errorf("there are LVs in LVG %s", vg), err)

	// expect that VGRemove failed
	e.OnCommand(fmt.Sprintf(lvm.LVsInVGCmdTmpl, vg)).Return("", "", nil).Times(1)
	e.OnCommand(fmt.Sprintf(lvm.VGRemoveCmdTmpl, vg)).Return("", "", errors.New("error"))
	err = c.removeLVGArtifacts(vg)
	assert.Contains(t, err.Error(), "unable to remove LVG")
}

func Test_increaseACSize(t *testing.T) {
	c := setup(t, node1ID)
	defer teardown(t, c)

	// add AC CR that point in LVGCR1
	err := c.k8sClient.CreateCR(tCtx, acCR1Name, &acCR1)
	assert.Nil(t, err)

	size := acCR1.Spec.Size
	drive := acCR1.Spec.Location
	c.increaseACSize(drive, 1)

	acList := &accrd.AvailableCapacityList{}
	err = c.k8sClient.ReadList(tCtx, acList)

	assert.Equal(t, size+1, acList.Items[0].Spec.Size)
}

// setup creates drive CRs and LVG CRs and returns Controller instance
func setup(t *testing.T, node string) *Controller {
	k8sClient, err := k8s.GetFakeKubeClient(ns, testLogger)
	assert.Nil(t, err)
	// create Drive CRs
	err = k8sClient.CreateCR(tCtx, drive1CR.Name, &drive1CR)
	assert.Nil(t, err)
	err = k8sClient.CreateCR(tCtx, drive2CR.Name, &drive2CR)
	assert.Nil(t, err)
	// create LVG CRs
	err = k8sClient.CreateCR(tCtx, lvgCR1.Name, &lvgCR1)
	assert.Nil(t, err)
	err = k8sClient.CreateCR(tCtx, lvgCR2.Name, &lvgCR2)
	assert.Nil(t, err)
	return NewController(k8sClient, node, testLogger)
}

// teardown removes drive CRs and LVG CRs
func teardown(t *testing.T, c *Controller) {
	var (
		driveList = &drivecrd.DriveList{}
		lvgList   = &lvgcrd.LVGList{}
		err       error
	)
	// remove all drive CRs
	err = c.k8sClient.ReadList(tCtx, driveList)
	assert.Nil(t, err)
	for _, d := range driveList.Items {
		err = c.k8sClient.DeleteCR(tCtx, &d)
		assert.Nil(t, err)
	}
	// remove all LVG CRs
	err = c.k8sClient.ReadList(tCtx, lvgList)
	assert.Nil(t, err)
	for _, l := range lvgList.Items {
		err = c.k8sClient.DeleteCR(tCtx, &l)
		assert.Nil(t, err)
	}
}
