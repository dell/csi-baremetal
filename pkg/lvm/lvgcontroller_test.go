package lvm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var (
	tCtx       = context.Background()
	testLogger = logrus.New()
	lvg1Name   = "lvg-cr-1"
	lvg2Name   = "lvg-cr-2"
	drive1UUID = "uuid-drive1"
	drive2UUID = "uuid-drive2"

	ns      = "default"
	node1ID = "node1"
	node2ID = "node2"

	apiDrive1 = api.Drive{
		UUID:         drive1UUID,
		VID:          "vid-drive1",
		PID:          "pid-drive1",
		SerialNumber: "hdd1", // depend on commands.LsblkTwoDevicesStr - /dev/sda
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         int64(1000 * base.GBYTE),
		Status:       api.Status_ONLINE,
		NodeId:       node1ID,
	}

	apiDrive2 = api.Drive{
		UUID:         drive2UUID,
		VID:          "vid-drive2",
		PID:          "pid-drive2",
		SerialNumber: "hdd2", // depend on commands.LsblkTwoDevicesStr - /dev/sdb
		Health:       api.Health_GOOD,
		Type:         api.DriveType_HDD,
		Size:         int64(333 * base.GBYTE),
		Status:       api.Status_ONLINE,
		NodeId:       node1ID,
	}

	drive1CR = drivecrd.Drive{
		TypeMeta: v1.TypeMeta{
			Kind:       "Drive",
			APIVersion: "drive.dell.com/v1",
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
			APIVersion: "drive.dell.com/v1",
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
			APIVersion: "lvg.dell.com/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      lvg1Name,
			Namespace: ns,
		},
		Spec: api.LogicalVolumeGroup{
			Name:      lvg1Name,
			Node:      node1ID,
			Locations: []string{apiDrive1.UUID, apiDrive2.UUID},
			Size:      int64(1024 * 500 * base.GBYTE),
			Status:    api.OperationalStatus_Creating,
		},
	}

	lvgCR2 = lvgcrd.LVG{
		TypeMeta: v1.TypeMeta{
			Kind:       "LVG",
			APIVersion: "lvg.dell.com/v1",
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
			Status:    api.OperationalStatus_Created,
		},
	}
)

func Test_NewLVGController(t *testing.T) {
	c := NewLVGController(nil, "node", testLogger)
	assert.NotNil(t, c)
}

func Test_ReconcileSuccessAnotherNode(t *testing.T) {
	c := setup(t, node1ID)
	defer teardown(t, c)
	// request for resources on another node
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR2.Name}}
	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func Test_ReconcileSuccessNotFound(t *testing.T) {
	c := setup(t, node1ID)
	defer teardown(t, c)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "not-found-that-name"}}
	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func Test_ReconcileSuccessCreatingLVG(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		e   = &mocks.GoMockExecutor{}
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
		lvg = &lvgcrd.LVG{}
	)
	defer teardown(t, c)

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)
	e.OnCommand(base.LsblkCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sda").Return("", "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sdb").Return("", "", nil)
	e.OnCommand(fmt.Sprintf("/sbin/lvm vgcreate --yes %s /dev/sda /dev/sdb", req.Name)).
		Return("", "", nil)

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
	err = c.k8sClient.ReadCR(tCtx, req.Name, lvg)
	assert.Equal(t, lvg.Spec.Status, api.OperationalStatus_Created)

	// reconciled second time
	res, err = c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func Test_ReconcileFailedNoPVs(t *testing.T) {
	// expect that no one PVs were created
	var (
		c = setup(t, node1ID)
		e = &mocks.GoMockExecutor{}
		// SearchDrivePathBySN failed for /dev/sdb
		lsblkResp = `{
			  "blockdevices":[{
				"name": "/dev/sda",
				"type": "disk",
				"serial": "hdd1"
				}]
			}`
		req         = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
		expectedErr = errors.New("no one PVs were created")
	)
	defer teardown(t, c)

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)
	e.OnCommand(base.LsblkCmd).Return(lsblkResp, "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sda").Return("", "", errors.New("pvcreate failed"))

	res, err := c.Reconcile(req)
	assert.NotNil(t, err)
	assert.Equal(t, err, expectedErr)
	assert.Equal(t, res, ctrl.Result{})
}

func Test_ReconcileFailedVGCreate(t *testing.T) {
	var (
		c           = setup(t, node1ID)
		e           = &mocks.GoMockExecutor{}
		req         = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
		expectedErr = errors.New("vgcreate failed")
	)
	defer teardown(t, c)

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)
	e.OnCommand(base.LsblkCmd).Return(mocks.LsblkTwoDevicesStr, "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sda").Return("", "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sdb").Return("", "", nil)
	e.OnCommand(fmt.Sprintf("/sbin/lvm vgcreate --yes %s /dev/sda /dev/sdb", req.Name)).
		Return("", "", expectedErr)

	res, err := c.Reconcile(req)
	assert.NotNil(t, err)
	assert.Equal(t, err, expectedErr)
	assert.Equal(t, res, ctrl.Result{})
}

func setup(t *testing.T, node string) *LVGController {
	k8sClient, err := base.GetFakeKubeClient(ns)
	assert.Nil(t, err)
	// create Drive CRs
	err = k8sClient.CreateCR(tCtx, &drive1CR, drive1CR.Name)
	assert.Nil(t, err)
	err = k8sClient.CreateCR(tCtx, &drive2CR, drive2CR.Name)
	assert.Nil(t, err)
	// create LVG CRs
	err = k8sClient.CreateCR(tCtx, &lvgCR1, lvgCR1.Name)
	assert.Nil(t, err)
	err = k8sClient.CreateCR(tCtx, &lvgCR2, lvgCR2.Name)
	assert.Nil(t, err)
	return NewLVGController(k8sClient, node, testLogger)
}

func teardown(t *testing.T, c *LVGController) {
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
