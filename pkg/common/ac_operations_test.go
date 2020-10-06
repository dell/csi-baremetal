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
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

func TestACOperationsImpl_SearchAC(t *testing.T) {
	var (
		acOp = setupACOperationsTest(t, &testAC1, &testAC2, &testAC3, &testAC4)
		ac   *accrd.AvailableCapacity
		err  error
	)
	// create LVG CR(with status Created) on which testAC4 is pointed
	lvg := testLVG
	lvg.Spec.Status = apiV1.Created
	err = acOp.k8sClient.CreateCR(testCtx, lvg.Name, &lvg)
	assert.Nil(t, err)

	// expect that testAC2 with size 100GB is choose
	ac = acOp.SearchAC(testCtx, "", int64(util.GBYTE)*50, apiV1.StorageClassHDD)
	assert.NotNil(t, ac)
	assert.Equal(t, testAC2.Name, ac.Name)

	// expect that testAC3 with size 1Tb is choose
	ac = acOp.SearchAC(testCtx, testNode2Name, int64(util.GBYTE)*133, apiV1.StorageClassHDD)
	assert.NotNil(t, ac)
	assert.Equal(t, testAC3.Name, ac.Name)

	// expect that testAC4 is choose
	ac = acOp.SearchAC(testCtx, testNode2Name, int64(util.GBYTE), apiV1.StorageClassHDDLVG)
	assert.NotNil(t, ac)
	assert.Equal(t, testAC4.Name, ac.Name)

	// expect that there is no suitable AC because of size
	ac = acOp.SearchAC(testCtx, testNode1Name, int64(util.TBYTE), apiV1.StorageClassHDD)
	assert.Nil(t, ac)

	// expect that there is no suitable AC because of storage class (SSD)
	ac = acOp.SearchAC(testCtx, testNode1Name, int64(util.GBYTE), apiV1.StorageClassSSD)
	assert.Nil(t, ac)

	// expect that there is no suitable AC because of node
	ac = acOp.SearchAC(testCtx, "some-another-node", int64(util.GBYTE), apiV1.StorageClassHDD)
	assert.Nil(t, ac)

	// expect AC2 (HDD 100Gb) was selected
	ac = acOp.SearchAC(testCtx, testAC4.Spec.NodeId, int64(util.GBYTE)*80, apiV1.StorageClassAny)
	assert.NotNil(t, ac)
	assert.Equal(t, testAC2.Name, ac.Name)
}

func TestACOperationsImpl_DeleteIfEmpty(t *testing.T) {
	emptyAC := testAC1
	emptyAC.Spec.Size = 0
	var (
		acOp   = setupACOperationsTest(t, &emptyAC, &testAC4)
		acList = accrd.AvailableCapacityList{}
		err    error
	)

	// should remove testAC1 because of size < AcSizeMinThresholdBytes
	err = acOp.DeleteIfEmpty(testCtx, emptyAC.Spec.Location)
	assert.Nil(t, err)
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items)) // expect that only testAC4 remain
	assert.Equal(t, testAC4Name, acList.Items[0].Name)

	// shouldn't remove AC4
	err = acOp.DeleteIfEmpty(testCtx, testAC4.Spec.Location)
	assert.Nil(t, err)
	acList = accrd.AvailableCapacityList{}
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(acList.Items)) // expect that only testAC4 remain
	assert.Equal(t, testAC4.Spec.Size, acList.Items[0].Spec.Size)

	// should return error because AC wan't found
	err = acOp.DeleteIfEmpty(testCtx, "unknown-ac")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to find AC by location")

}

func Test_recreateACToLVGSC_Success(t *testing.T) {
	var (
		acOp  = setupACOperationsTest(t, &testAC2, &testAC3)
		newAC *accrd.AvailableCapacity
	)

	// there are no acs are provided
	newAC = acOp.RecreateACToLVGSC(testCtx, apiV1.StorageClassHDDLVG)
	assert.Nil(t, newAC)

	// ensure that there are 2 ACs
	acList := accrd.AvailableCapacityList{}
	err := acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(acList.Items))

	// expect that AC with SC HDDLVG will be created and will be size of 1Tb + 100Gb
	// testAC2 and testAC3 should be removed
	newAC = acOp.RecreateACToLVGSC(testCtx, apiV1.StorageClassHDDLVG, testAC2, testAC3)

	// check that LVG is in creating state
	lvgList := lvgcrd.LVGList{}
	err = acOp.k8sClient.ReadList(testCtx, &lvgList)
	assert.Equal(t, 1, len(lvgList.Items))
	lvg := lvgList.Items[0]
	assert.Equal(t, apiV1.Creating, lvg.Spec.Status)
	assert.Equal(t, testAC2.Spec.Size+testAC3.Spec.Size, lvg.Spec.Size)
	assert.Equal(t, testAC2.Spec.NodeId, lvg.Spec.Node)
	assert.Equal(t, 2, len(lvg.Spec.Locations))
	expectedLocation := []string{testAC2.Spec.Location, testAC3.Spec.Location}
	currentLocation := lvg.Spec.Locations
	sort.Strings(expectedLocation)
	sort.Strings(currentLocation)
	assert.Equal(t, expectedLocation, currentLocation)

	// check that AC2 and AC3 size was set to 0
	acList = accrd.AvailableCapacityList{}
	err = acOp.k8sClient.ReadList(testCtx, &acList)
	assert.Equal(t, 3, len(acList.Items))
	assert.Equal(t, apiV1.StorageClassHDD, acList.Items[0].Spec.StorageClass)
	assert.Equal(t, int64(0), acList.Items[0].Spec.Size)
	assert.Equal(t, apiV1.StorageClassHDD, acList.Items[1].Spec.StorageClass)
	assert.Equal(t, int64(0), acList.Items[1].Spec.Size)
	assert.Equal(t, apiV1.StorageClassHDDLVG, acList.Items[2].Spec.StorageClass)
}

func TestACOperationsImpl_acNodeMapping(t *testing.T) {
	acOp := setupACOperationsTest(t)
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
	acOp := setupACOperationsTest(t, &testAC1, &testAC2, &testAC3)

	acNodeMap := acOp.acNodeMapping([]accrd.AvailableCapacity{testAC1, testAC2, testAC3})
	balancedNode := acOp.balanceAC(acNodeMap, int64(util.MBYTE), apiV1.StorageClassHDD)
	assert.Equal(t, testNode2Name, balancedNode)

	// should return empty string if there are no AC with appropriate size
	balancedNode = acOp.balanceAC(acNodeMap, int64(util.TBYTE)*100, apiV1.StorageClassHDD)
	assert.Equal(t, "", balancedNode)

	// should return empty string if there are no AC with appropriate SC
	balancedNode = acOp.balanceAC(acNodeMap, int64(util.TBYTE)*100, apiV1.StorageClassHDDLVG)
	assert.Equal(t, "", balancedNode)

	// should return empty is acNodeMap is empty
	balancedNode = acOp.balanceAC(map[string][]*accrd.AvailableCapacity{}, int64(util.TBYTE)*100, apiV1.StorageClassHDD)
	assert.Equal(t, "", balancedNode)
}

func TestACOperationsImpl_tryToFindAC(t *testing.T) {
	var (
		acOp = NewACOperationsImpl(nil, testLogger)
		acs  = []*accrd.AvailableCapacity{&testAC1, &testAC2, &testAC3, &testAC4}
		ac   *accrd.AvailableCapacity
	)

	kubeClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)
	acOp.k8sClient = kubeClient

	ac = acOp.tryToFindAC(acs, apiV1.StorageClassHDD, int64(util.GBYTE)*89)
	assert.NotNil(t, ac)
	assert.Equal(t, ac.Name, testAC2.Name)

	ac = acOp.tryToFindAC(acs, "", int64(util.GBYTE)*89)
	assert.NotNil(t, ac)
	assert.Equal(t, ac.Name, testAC4.Name)
}

// creates fake k8s client and creates AC CRs based on provided acs
// returns instance of ACOperationsImpl based on created k8s client
func setupACOperationsTest(t *testing.T, acs ...*accrd.AvailableCapacity) *ACOperationsImpl {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)
	assert.NotNil(t, k8sClient)

	for _, ac := range acs {
		err := k8sClient.CreateCR(testCtx, ac.Name, ac)
		assert.Nil(t, err)
	}
	return NewACOperationsImpl(k8sClient, testLogger)
}

// lvgReconcileImitation this is an Reconcile imitation, expect only 1 LVG is present
// read LVG list until it size in not 1 and then set status to newStatus for LVG CR
func lvgReconcileImitation(k8sClient *k8s.KubeClient, newStatus string) error {
	var (
		lvgCRList lvgcrd.LVGList
		err       error
		ticker    = time.NewTicker(50 * time.Millisecond)
	)
	println("Reconciling ...")
	for {
		if err = k8sClient.ReadList(context.Background(), &lvgCRList); err != nil {
			return err
		}

		if len(lvgCRList.Items) == 1 {
			break
		}
		<-ticker.C
	}
	ticker.Stop()
	lvgCRList.Items[0].Spec.Status = newStatus
	return k8sClient.UpdateCR(context.Background(), &lvgCRList.Items[0])
}

func Test_AlignSizeByPE(t *testing.T) {
	type args struct {
		size int64
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "100Mi",
			args: args{
				size: 104857600,
			},
			want: 104857600,
		},
		{
			name: "lower than 1Mi",
			args: args{
				size: 1,
			},
			want: DefaultPESize,
		},
		{
			name: "slightly lower than 4Mi",
			args: args{
				size: 4194303,
			},
			want: DefaultPESize,
		},
		{
			name: "slightly higher than 4Mi",
			args: args{
				size: 4194305,
			},
			want: 2 * DefaultPESize,
		},
		{
			name: "Negative",
			args: args{
				size: -104857600,
			},
			want: -104857600,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AlignSizeByPE(tt.args.size); got != tt.want {
				t.Errorf("alignSizeByPE() = %v, want %v", got, tt.want)
			}
		})
	}
}
