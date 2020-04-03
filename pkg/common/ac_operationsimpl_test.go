package common

import (
	"context"
	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	crdV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	testNS = "default"

	testCtx       = context.Background()
	testNode1Name = "node1"
	testNode2Name = "node2"

	testDriveLocation1 = "drive1-sn"
	testDriveLocation2 = "drive2-sn"
	testDriveLocation3 = "drive3-sn"
	testDriveLocation4 = "drive4-sn"

	testAC1Name = fmt.Sprintf("%s-%s", testNode1Name, strings.ToLower(testDriveLocation1))
	testAC1     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC1Name, Namespace: testNS},
		Spec: api.AvailableCapacity{
			Size:         int64(base.GBYTE),
			StorageClass: api.StorageClass_HDD,
			Location:     testDriveLocation1,
			NodeId:       testNode1Name},
	}
	testAC2Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation2))
	testAC2     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC2Name, Namespace: testNS},
		Spec: api.AvailableCapacity{
			Size:         int64(base.GBYTE) * 100,
			StorageClass: api.StorageClass_HDD,
			Location:     testDriveLocation2,
			NodeId:       testNode2Name,
		},
	}
	testAC3Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation3))
	testAC3     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC3Name, Namespace: testNS},
		Spec: api.AvailableCapacity{
			Size:         int64(base.TBYTE),
			StorageClass: api.StorageClass_HDD,
			Location:     testDriveLocation3,
			NodeId:       testNode2Name,
		},
	}

	testLVGName = "lvg-1"
	testLVG     = lvgcrd.LVG{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "LVG", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testLVGName, Namespace: testNS},
		Spec: api.LogicalVolumeGroup{
			Name:      testLVGName,
			Node:      testNode2Name,
			Locations: []string{testDriveLocation4},
			Size:      int64(base.GBYTE) * 100,
			Status:    api.OperationalStatus_Creating,
		},
	}
	testAC4Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation4))
	testAC4     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC4Name, Namespace: testNS},
		Spec: api.AvailableCapacity{
			Size:         testLVG.Spec.Size,
			StorageClass: api.StorageClass_HDDLVG,
			Location:     testLVGName,
			NodeId:       testNode2Name,
		},
	}
)

func TestACOperationsImpl_SearchAC(t *testing.T) {
	var (
		acOp = setup(t, &testAC1, &testAC2, &testAC3, &testAC4)
		ac   *accrd.AvailableCapacity
		err  error
	)
	// create LVG CR(with status Created) on which testAC4 is pointed
	lvg := testLVG
	lvg.Spec.Status = api.OperationalStatus_Created
	err = acOp.k8sClient.CreateCR(testCtx, &lvg, lvg.Name)
	assert.Nil(t, err)

	// expect that testAC2 with size 100GB is choose
	ac = acOp.SearchAC(testCtx, "", int64(base.GBYTE)*50, api.StorageClass_HDD)
	assert.NotNil(t, ac)
	assert.Equal(t, testAC2.Name, ac.Name)

	// expect that testAC3 with size 1Tb is choose
	ac = acOp.SearchAC(testCtx, testNode2Name, int64(base.GBYTE)*133, api.StorageClass_HDD)
	assert.NotNil(t, ac)
	assert.Equal(t, testAC3.Name, ac.Name)

	// expect that testAC4 is choose
	ac = acOp.SearchAC(testCtx, testNode2Name, int64(base.GBYTE), api.StorageClass_HDDLVG)
	assert.NotNil(t, ac)
	assert.Equal(t, testAC4.Name, ac.Name)

	// expect that there is no suitable AC because of size
	ac = acOp.SearchAC(testCtx, testNode1Name, int64(base.TBYTE), api.StorageClass_HDD)
	assert.Nil(t, ac)

	// expect that there is no suitable AC because of storage class (SSD)
	ac = acOp.SearchAC(testCtx, testNode1Name, int64(base.GBYTE), api.StorageClass_SSD)
	assert.Nil(t, ac)

	// expect that there is no suitable AC because of node
	ac = acOp.SearchAC(testCtx, "some-another-node", int64(base.GBYTE), api.StorageClass_HDD)
	assert.Nil(t, ac)
}

func TestNewACOperationsImpl_SearchAC_WithLVGCreationSuccess(t *testing.T) {
	var (
		acOp   = setup(t, &testAC1)
		ac     *accrd.AvailableCapacity
		err    error
		acList = accrd.AvailableCapacityList{}
	)
	// expect that AC is recreated to LVG
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ac = acOp.SearchAC(testCtx, testNode1Name, int64(base.MBYTE)*500, api.StorageClass_HDDLVG)
		wg.Done()
	}()
	lvgReconcileImitation(acOp.k8sClient, api.OperationalStatus_Created, t)
	wg.Wait()
	assert.NotNil(t, ac)
	assert.Equal(t, testAC1.Spec.Size, ac.Spec.Size)

	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items))
	assert.Equal(t, api.StorageClass_HDDLVG, acList.Items[0].Spec.StorageClass)
}

func TestNewACOperationsImpl_SearchAC_WithLVGCreationFail(t *testing.T) {
	var (
		acOp   = setup(t, &testAC1)
		ac     *accrd.AvailableCapacity
		err    error
		acList = accrd.AvailableCapacityList{}
	)
	// expect that AC is recreated to LVG
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ac = acOp.SearchAC(testCtx, testNode1Name, int64(base.MBYTE)*500, api.StorageClass_HDDLVG)
		wg.Done()
	}()
	lvgReconcileImitation(acOp.k8sClient, api.OperationalStatus_FailedToCreate, t)
	wg.Wait()
	assert.Nil(t, ac)

	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))
}

func TestACOperationsImpl_UpdateACSizeOrDelete(t *testing.T) {
	var (
		acOp   = setup(t, &testAC1, &testAC4)
		acList = accrd.AvailableCapacityList{}
		err    error
	)

	// should remove testAC1 because of HDD SC
	err = acOp.UpdateACSizeOrDelete(&testAC1, 0)
	assert.Nil(t, err)
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items)) // expect that only testAC4 remain
	assert.Equal(t, testAC4Name, acList.Items[0].Name)

	// should increase testAC4 size
	err = acOp.UpdateACSizeOrDelete(&testAC4, int64(base.GBYTE)*30)
	assert.Nil(t, err)
	acList = accrd.AvailableCapacityList{}
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, int64(base.GBYTE)*(100+30), acList.Items[0].Spec.Size)

	// should remove testAC4 because of size < acSizeMinThresholdBytes
	acOp = setup(t, &testAC4)
	err = acOp.UpdateACSizeOrDelete(&testAC4, -(testAC4.Spec.Size - 1024))
	assert.Nil(t, err)
	acList = accrd.AvailableCapacityList{}
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(acList.Items))
}

func Test_recreateACToLVGSC_Success(t *testing.T) {
	acOp := setup(t, &testAC2, &testAC3)

	// ensure that there are 2 ACs
	acList := accrd.AvailableCapacityList{}
	err := acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(acList.Items))

	// expect that AC with SC HDDLVG will be created and will be size of 1Tb + 100Gb
	// testAC2 and testAC3 should be removed
	var (
		wg    sync.WaitGroup
		newAC *accrd.AvailableCapacity
	)
	wg.Add(1)
	go func() {
		newAC = acOp.recreateACToLVGSC(api.StorageClass_HDDLVG, &testAC2, &testAC3)
		wg.Done()
	}()
	lvgReconcileImitation(acOp.k8sClient, api.OperationalStatus_Created, t)
	wg.Wait()
	assert.NotNil(t, newAC)
	assert.Equal(t, testAC2.Spec.Size+testAC3.Spec.Size, newAC.Spec.Size)
	assert.Equal(t, testAC2.Spec.NodeId, newAC.Spec.NodeId)

	// check LVG that was created
	lvgList := lvgcrd.LVGList{}
	err = acOp.k8sClient.ReadList(testCtx, &lvgList)
	assert.Equal(t, 1, len(lvgList.Items))
	lvg := lvgList.Items[0]
	assert.Equal(t, testAC2.Spec.Size+testAC3.Spec.Size, lvg.Spec.Size)
	assert.Equal(t, testAC2.Spec.NodeId, lvg.Spec.Node)
	assert.Equal(t, 2, len(lvg.Spec.Locations))
	expectedLocation := []string{testAC2.Spec.Location, testAC3.Spec.Location}
	currentLocation := lvg.Spec.Locations
	sort.Strings(expectedLocation)
	sort.Strings(currentLocation)
	assert.Equal(t, expectedLocation, currentLocation)

	// check that AC2 and AC3 was removed
	acList = accrd.AvailableCapacityList{}
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Equal(t, 1, len(acList.Items))
	assert.Equal(t, api.StorageClass_HDDLVG, acList.Items[0].Spec.StorageClass)
}

func TestACOperationsImpl_recreateACToLVGSC_Fail(t *testing.T) {
	acOp := setup(t, &testAC2, &testAC3)

	// ensure that there are 2 ACs
	acList := accrd.AvailableCapacityList{}
	err := acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(acList.Items))

	// expect that AC with SC HDDLVG will be created and will be size of 1Tb + 100Gb
	// testAC2 and testAC3 should be removed
	var (
		wg    sync.WaitGroup
		newAC *accrd.AvailableCapacity
	)
	wg.Add(1)
	go func() {
		newAC = acOp.recreateACToLVGSC(api.StorageClass_HDDLVG, &testAC2, &testAC3)
		wg.Done()
	}()
	lvgReconcileImitation(acOp.k8sClient, api.OperationalStatus_FailedToCreate, t)
	wg.Wait()
	assert.Nil(t, newAC)

	// check that AC2 and AC3 was removed and new AC wasn't created
	acList = accrd.AvailableCapacityList{}
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Equal(t, 0, len(acList.Items))
}

func TestACOperationsImpl_waitUntilLVGWillBeCreated(t *testing.T) {
	acOp := setup(t)
	lvgCR := testLVG
	lvgCR.Spec.Status = api.OperationalStatus_Created

	err := acOp.k8sClient.CreateCR(testCtx, &lvgCR, lvgCR.Name)
	assert.Nil(t, err)

	// lvgCR have Created status
	lvg := acOp.waitUntilLVGWillBeCreated(testCtx, lvgCR.Name)
	assert.NotNil(t, lvg)
	assert.Equal(t, lvgCR.Spec, *lvg)

	// lvgCR have Failed status
	lvgCR.Spec.Status = api.OperationalStatus_FailedToCreate
	err = acOp.k8sClient.UpdateCR(testCtx, &lvgCR)
	lvg = acOp.waitUntilLVGWillBeCreated(testCtx, lvgCR.Name)
	assert.Nil(t, lvg)

	// context is done
	var (
		wg           sync.WaitGroup
		ctx, closeFn = context.WithTimeout(context.Background(), time.Second*2)
		lvg2         *api.LogicalVolumeGroup
	)
	defer closeFn()
	lvgCR.Spec.Status = api.OperationalStatus_Creating
	err = acOp.k8sClient.UpdateCR(testCtx, &lvgCR)
	assert.Nil(t, err)
	wg.Add(1)
	go func() {
		lvg2 = acOp.waitUntilLVGWillBeCreated(ctx, lvgCR.Name)
		wg.Done()
	}()
	ctx.Done()
	println("ctx was done")
	wg.Wait()
	assert.Nil(t, lvg2)
}

func TestACOperationsImpl_acNodeMapping(t *testing.T) {
	acOp := setup(t)
	// AC1 locates on node1, AC2 and AC3 locate on node2
	acList := []accrd.AvailableCapacity{testAC1, testAC2, testAC3}

	acNodeMap := acOp.acNodeMapping(acList)
	assert.Equal(t, 2, len(acNodeMap))
	assert.Equal(t, 1, len(acNodeMap[testNode1Name]))
	assert.Equal(t, 2, len(acNodeMap[testNode2Name]))

	// should return not nil for empty list
	acNodeMap = acOp.acNodeMapping([]accrd.AvailableCapacity{})
	assert.NotNil(t, acNodeMap)
	assert.Equal(t, 0, len(acNodeMap))
}

func TestACOperationsImpl_balanceAC(t *testing.T) {
	acOp := setup(t, &testAC1, &testAC2, &testAC3)

	acNodeMap := acOp.acNodeMapping([]accrd.AvailableCapacity{testAC1, testAC2, testAC3})
	balancedNode := acOp.balanceAC(acNodeMap, int64(base.MBYTE), api.StorageClass_HDD)
	assert.Equal(t, testNode2Name, balancedNode)

	// should return empty string if there are no AC with appropriate size
	balancedNode = acOp.balanceAC(acNodeMap, int64(base.TBYTE)*100, api.StorageClass_HDD)
	assert.Equal(t, "", balancedNode)

	// should return empty string if there are no AC with appropriate SC
	balancedNode = acOp.balanceAC(acNodeMap, int64(base.TBYTE)*100, api.StorageClass_HDDLVG)
	assert.Equal(t, "", balancedNode)

	// should return empty is acNodeMap is empty
	balancedNode = acOp.balanceAC(map[string][]*accrd.AvailableCapacity{}, int64(base.TBYTE)*100, api.StorageClass_HDD)
	assert.Equal(t, "", balancedNode)
}

// creates fake k8s client and creates AC CRs based on provided acs
// returns instance of ACOperationsImpl based on created k8s client
func setup(t *testing.T, acs ...*accrd.AvailableCapacity) *ACOperationsImpl {
	k8sClient, err := base.GetFakeKubeClient(testNS)
	assert.Nil(t, err)
	assert.NotNil(t, k8sClient)

	for _, ac := range acs {
		err := k8sClient.CreateCR(testCtx, ac, ac.Name)
		assert.Nil(t, err)
	}
	return NewACOperationsImpl(k8sClient, logrus.New())
}

// lvgReconcileImitation this is an Reconcile imitation, expect only 1 LVG is present
// read LVG list until it size in not 1 and then set status to newStatus for LVG CR
func lvgReconcileImitation(k8sClient *base.KubeClient, newStatus api.OperationalStatus, t *testing.T) {
	var (
		lvgCRList lvgcrd.LVGList
		err       error
		ticker    = time.NewTicker(50 * time.Millisecond)
	)
	println("Reconciling ...")
	for {
		err = k8sClient.ReadList(testCtx, &lvgCRList)
		assert.Nil(t, err)
		if len(lvgCRList.Items) == 1 {
			break
		}
		<-ticker.C
	}
	ticker.Stop()
	lvgCRList.Items[0].Spec.Status = newStatus
	err = k8sClient.UpdateCR(testCtx, &lvgCRList.Items[0])
}
