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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

var DefaultPESize = capacityplanner.DefaultPESize

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

	// check that LogicalVolumeGroup is in creating state
	lvgList := lvgcrd.LogicalVolumeGroupList{}
	err = acOp.k8sClient.ReadList(testCtx, &lvgList)
	assert.Equal(t, 1, len(lvgList.Items))
	lvg := lvgList.Items[0]
	assert.Equal(t, apiV1.Creating, lvg.Spec.Status)
	assert.Equal(t,
		capacityplanner.SubtractLVMMetadataSize(testAC2.Spec.Size)+
			capacityplanner.SubtractLVMMetadataSize(testAC3.Spec.Size), lvg.Spec.Size)
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
			if got := capacityplanner.AlignSizeByPE(tt.args.size); got != tt.want {
				t.Errorf("alignSizeByPE() = %v, want %v", got, tt.want)
			}
		})
	}
}
