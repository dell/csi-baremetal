package lvm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	crdV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
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
			APIVersion: crdV1.APIV1Version,
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
			APIVersion: crdV1.APIV1Version,
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
			APIVersion: crdV1.APIV1Version,
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
			APIVersion: crdV1.APIV1Version,
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

	acCR1Name = "ac1"
	acCR1     = accrd.AvailableCapacity{
		TypeMeta: v1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      acCR1Name,
			Namespace: ns,
		},
		Spec: api.AvailableCapacity{
			Location:     lvg1Name,
			NodeId:       node1ID,
			StorageClass: api.StorageClass_HDDLVG,
			Size:         int64(1024 * 300 * base.GBYTE),
		},
	}
)

func Test_NewLVGController(t *testing.T) {
	c := NewLVGController(nil, "node", testLogger)
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

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)

	lvgToDell := lvgCR1
	lvgToDell.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	lvgToDell.ObjectMeta.Finalizers = []string{lvgFinalizer}
	err := c.k8sClient.UpdateCR(tCtx, &lvgToDell)

	e.OnCommand(fmt.Sprintf(base.LVsInVGCmdTmpl, lvgCR1.Name)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(base.VGRemoveCmdTmpl, lvgCR1.Name)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(base.PVsInVGCmdTmpl, base.EmptyName)).Return("", "", nil).Times(1)

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})
}

func TestReconcile_DeletionFailed(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		e   = &mocks.GoMockExecutor{}
		req = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: lvgCR1.Name}}
	)
	defer teardown(t, c)

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)

	lvgToDell := lvgCR1
	lvgToDell.ObjectMeta.DeletionTimestamp = &v1.Time{Time: time.Now()}
	lvgToDell.ObjectMeta.Finalizers = []string{lvgFinalizer}
	err := c.k8sClient.UpdateCR(tCtx, &lvgToDell)

	// expect that LVG still contains LV
	e.OnCommand(fmt.Sprintf(base.LVsInVGCmdTmpl, lvgCR1.Name)).Return("lv1", "", nil)

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
		// SearchDrivePathBySN failed for /dev/sdb
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

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)
	e.OnCommand(base.LsblkCmd).Return(lsblkResp, "", nil)
	e.OnCommand("/sbin/lvm pvcreate --yes /dev/sda").Return("", "", errors.New("pvcreate failed"))

	res, err := c.Reconcile(req)
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	lvgCR := &lvgcrd.LVG{}
	err = c.k8sClient.ReadCR(tCtx, lvgCR1.Name, lvgCR)
	assert.Equal(t, api.OperationalStatus_FailedToCreate, lvgCR.Spec.Status)
}

func TestReconcile_FailedVGCreate(t *testing.T) {
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
	assert.Nil(t, err)
	assert.Equal(t, res, ctrl.Result{})

	lvgCR := &lvgcrd.LVG{}
	err = c.k8sClient.ReadCR(tCtx, lvgCR1.Name, lvgCR)
	assert.Equal(t, api.OperationalStatus_FailedToCreate, lvgCR.Spec.Status)
}

func Test_removeLVGArtifacts_Success(t *testing.T) {
	var (
		c   = setup(t, node1ID)
		e   = &mocks.GoMockExecutor{}
		vg  = lvgCR1.Name
		err error
	)
	defer teardown(t, c)

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)

	e.OnCommand(fmt.Sprintf(base.LVsInVGCmdTmpl, lvgCR1.Name)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(base.VGRemoveCmdTmpl, vg)).Return("", "", nil)
	e.OnCommand(fmt.Sprintf(base.PVsInVGCmdTmpl, base.EmptyName)).Return("", "", nil).Times(1)
	err = c.removeLVGArtifacts(vg)
	assert.Nil(t, err)

	// expect that RemoveOrphanPVs failed and ignore it
	e.OnCommand(fmt.Sprintf(base.PVsInVGCmdTmpl, base.EmptyName)).
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

	c.linuxUtils = base.NewLinuxUtils(e, testLogger)

	// expect that VG contains LV
	e.OnCommand(fmt.Sprintf(base.LVsInVGCmdTmpl, vg)).Return("some-lv1", "", nil).Times(1)
	err = c.removeLVGArtifacts(vg)
	assert.Equal(t, fmt.Errorf("there are LVs in LVG %s", vg), err)

	// expect that VGRemove failed
	e.OnCommand(fmt.Sprintf(base.LVsInVGCmdTmpl, vg)).Return("", "", nil).Times(1)
	e.OnCommand(fmt.Sprintf(base.VGRemoveCmdTmpl, vg)).Return("", "", errors.New("error"))
	err = c.removeLVGArtifacts(vg)
	assert.Contains(t, err.Error(), "unable to remove LVG")
}

func Test_RemoveChildAC(t *testing.T) {
	c := setup(t, node1ID)
	defer teardown(t, c)

	// add AC CR that point in LVGCR1
	err := c.k8sClient.CreateCR(tCtx, &acCR1, acCR1Name)
	assert.Nil(t, err)

	c.removeChildAC(acCR1.Spec.Location)

	// expect that there are no any AC CR
	acList := &accrd.AvailableCapacityList{}
	err = c.k8sClient.ReadList(tCtx, acList)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))

	// add AC CR that point in lvgCR1
	err = c.k8sClient.CreateCR(tCtx, &acCR1, acCR1Name)
	assert.Nil(t, err)
	// try to remove AC that point on lvgCR2
	c.removeChildAC(lvgCR2.Name)

	// expect that there are no any AC CR were removed
	acList = &accrd.AvailableCapacityList{}
	err = c.k8sClient.ReadList(tCtx, acList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items))
}

// setup creates drive CRs and LVG CRs and returns LVGController instance
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

// teardown removes drive CRs and LVG CRs
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
