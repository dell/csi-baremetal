package capacityplanner

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
)

const (
	nodeName = "node-1"
)

func TestNewNodeCapacity(t *testing.T) {
	type args struct {
		node string
		acs  []accrd.AvailableCapacity
		acrs []acrcrd.AvailableCapacityReservation
	}

	var (
		testACHDD1    = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassHDD)
		testACHDD2    = *getTestAC(nodeName, testLargeSize, apiV1.StorageClassHDD)
		testACHDD3    = *getTestAC(nodeName+"-incorrect", testSmallSize, apiV1.StorageClassHDD)
		testACHDDLVG1 = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassHDDLVG)
		testACSSD1    = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassSSD)
		testACNVMe1   = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassNVMe)

		testACRHDD1    = *getTestACR(testSmallSize, apiV1.StorageClassHDD, []*accrd.AvailableCapacity{&testACHDD1, &testACHDD2})
		testACRHDD2    = testACRHDD1.DeepCopy()
		testACRHDDLVG1 = *getTestACR(testSmallSize, apiV1.StorageClassHDDLVG, []*accrd.AvailableCapacity{&testACHDDLVG1})
		testACRHDDLVG2 = *getTestACR(testSmallSize, apiV1.StorageClassHDDLVG, []*accrd.AvailableCapacity{&testACHDDLVG1})
	)

	testACRHDD2.Spec.Status = apiV1.MatchReservationStatus(apiV1.ReservationRejected)

	tests := []struct {
		name string
		args args
		want *nodeCapacity
	}{
		{
			name: "Empty lists",
			args: args{
				node: nodeName,
				acs:  nil,
				acrs: nil,
			},
			want: nil,
		},
		{
			name: "One AC",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACHDD1},
				acrs: nil,
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDD1}),
				acsOrder: scToACOrder{
					apiV1.StorageClassHDD:    []string{testACHDD1.Name},
					apiV1.StorageClassHDDLVG: []string{testACHDD1.Name},
					apiV1.StorageClassAny:    []string{testACHDD1.Name},
				},
				reservedACs: reservedACs{},
			},
		},
		{
			name: "ANY SC should respect the slowest",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACNVMe1, testACSSD1, testACHDD1},
				acrs: nil,
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDD1, testACSSD1, testACNVMe1}),
				acsOrder: scToACOrder{
					apiV1.StorageClassAny: []string{testACHDD1.Name, testACSSD1.Name, testACNVMe1.Name},
				},
				reservedACs: reservedACs{},
			},
		},
		{
			name: "Should sort with size",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACHDD2, testACHDD1},
				acrs: nil,
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDD1, testACHDD2}),
				acsOrder: scToACOrder{
					apiV1.StorageClassHDD: []string{testACHDD1.Name, testACHDD2.Name},
				},
				reservedACs: reservedACs{},
			},
		},
		{
			name: "Should filter AC with nodeName",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACHDD3, testACHDD1},
				acrs: nil,
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDD1}),
				acsOrder: scToACOrder{
					apiV1.StorageClassHDD: []string{testACHDD1.Name},
				},
				reservedACs: reservedACs{},
			},
		},
		{
			name: "Should select LVG AC first",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACHDD1, testACHDDLVG1},
				acrs: nil,
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDD1, testACHDDLVG1}),
				acsOrder: scToACOrder{
					apiV1.StorageClassHDD:    []string{testACHDD1.Name},
					apiV1.StorageClassHDDLVG: []string{testACHDDLVG1.Name, testACHDD1.Name},
				},
				reservedACs: reservedACs{},
			},
		},
		{
			name: "One ACR",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACHDD1},
				acrs: []acrcrd.AvailableCapacityReservation{testACRHDD1},
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDD1}),
				acsOrder: scToACOrder{
					apiV1.StorageClassHDD: []string{testACHDD1.Name},
				},
				reservedACs: reservedACs{
					testACHDD1.Name: &reservedCapacity{testSmallSize, apiV1.MatchStorageClass(apiV1.StorageClassHDD)},
					testACHDD2.Name: &reservedCapacity{testSmallSize, apiV1.MatchStorageClass(apiV1.StorageClassHDD)},
				},
			},
		},
		{
			name: "Should skip ACR in REJECTED",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACHDD1},
				acrs: []acrcrd.AvailableCapacityReservation{testACRHDD1},
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDD1}),
				acsOrder: scToACOrder{
					apiV1.StorageClassHDD: []string{testACHDD1.Name},
				},
				reservedACs: reservedACs{},
			},
		},
		{
			name: "Should summarize reserved LVG ACs",
			args: args{
				node: nodeName,
				acs:  []accrd.AvailableCapacity{testACHDDLVG1},
				acrs: []acrcrd.AvailableCapacityReservation{testACRHDDLVG1, testACRHDDLVG2},
			},
			want: &nodeCapacity{
				node: nodeName,
				acs:  buildACMap([]accrd.AvailableCapacity{testACHDDLVG1}),
				acsOrder: scToACOrder{
					apiV1.StorageClassHDDLVG: []string{testACHDDLVG1.Name},
				},
				reservedACs: reservedACs{
					testACHDDLVG1.Name: &reservedCapacity{2 * testSmallSize, apiV1.MatchStorageClass(apiV1.StorageClassHDDLVG)},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newNodeCapacity(tt.args.node, tt.args.acs, tt.args.acrs)
			if tt.want == nil {
				if got != nil {
					t.Errorf("newNodeCapacity() = %+v, want %+v", got, tt.want)
				}
			} else {
				if !reflect.DeepEqual(got.node, tt.want.node) {
					t.Errorf("newNodeCapacity().ndoe = %+v, want %+v", got.node, tt.want.node)
				}
				if !reflect.DeepEqual(got.acs, tt.want.acs) {
					t.Errorf("len(newNodeCapacity().acs) = %+v, want %+v", got.acs, tt.want.acs)
				}
				for k, v := range tt.want.reservedACs {
					if !reflect.DeepEqual(got.reservedACs[k], v) {
						t.Errorf("newNodeCapacity().reservedACs[%s] = %+v, want %+v", k, got.reservedACs[k], v)
					}
				}
				for k, v := range tt.want.acsOrder {
					if !reflect.DeepEqual(got.acsOrder[k], v) {
						t.Errorf("newNodeCapacity().acsOrder[%s] = %+v, want %+v", k, got.acsOrder[k], v)
					}
				}
			}
		})
	}
}

func TestSelectACForVolume(t *testing.T) {
	type args struct {
		nc  *nodeCapacity
		vol *genV1.Volume
	}

	var (
		testACHDD1 = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassHDD)
		testACHDD2 = *getTestAC(nodeName, testLargeSize, apiV1.StorageClassHDD)
		//testACHDD3 = *getTestAC(nodeName+"-incorrect", testSmallSize, apiV1.StorageClassHDD)
		testACHDDLVG1 = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassHDDLVG)
		testACSSD1    = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassSSD)
		testACNVMe1   = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassNVMe)

		testACRHDD1    = *getTestACR(testSmallSize, apiV1.StorageClassHDD, []*accrd.AvailableCapacity{&testACHDD1})
		testACRHDDLVG1 = *getTestACR(testSmallSize, apiV1.StorageClassHDDLVG, []*accrd.AvailableCapacity{&testACHDD2})
		//testACRHDDLVG2 = *getTestACR(testSmallSize, apiV1.StorageClassHDDLVG, []*accrd.AvailableCapacity{&testACHDDLVG1})
	)

	tests := []struct {
		name string
		args args
		want *accrd.AvailableCapacity
	}{
		{
			name: "One HDD AC",
			args: args{
				nc:  newNodeCapacity(nodeName, []accrd.AvailableCapacity{testACHDD1}, nil),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassHDD),
			},
			want: &testACHDD1,
		},
		{
			name: "One SSD AC",
			args: args{
				nc:  newNodeCapacity(nodeName, []accrd.AvailableCapacity{testACSSD1}, nil),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassSSD),
			},
			want: &testACSSD1,
		},
		{
			name: "One NVMe AC",
			args: args{
				nc:  newNodeCapacity(nodeName, []accrd.AvailableCapacity{testACNVMe1}, nil),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassNVMe),
			},
			want: &testACNVMe1,
		},
		{
			name: "Should respect LVG SC for LVG volume request",
			args: args{
				nc:  newNodeCapacity(nodeName, []accrd.AvailableCapacity{testACHDD1, testACHDDLVG1}, nil),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassHDDLVG),
			},
			want: &testACHDDLVG1,
		},
		{
			name: "Should reject reserved AC",
			args: args{
				nc: newNodeCapacity(nodeName,
					[]accrd.AvailableCapacity{testACHDD1},
					[]acrcrd.AvailableCapacityReservation{testACRHDD1}),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassHDD),
			},
			want: nil,
		},
		{
			name: "Should reject reserved AC (LVG)",
			args: args{
				nc: newNodeCapacity(nodeName,
					[]accrd.AvailableCapacity{testACHDDLVG1},
					[]acrcrd.AvailableCapacityReservation{testACRHDDLVG1}),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassHDD),
			},
			want: nil,
		},
		{
			name: "Should reserve non-LVG for LVG",
			args: args{
				nc: newNodeCapacity(nodeName,
					[]accrd.AvailableCapacity{testACHDD1},
					nil),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassHDDLVG),
			},
			want: &testACHDD1,
		},
		{
			name: "Should reserve non-LVG for LVG with ACR",
			args: args{
				nc: newNodeCapacity(nodeName,
					[]accrd.AvailableCapacity{testACHDD2},
					[]acrcrd.AvailableCapacityReservation{testACRHDDLVG1}),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassHDDLVG),
			},
			want: &testACHDD2,
		},
		{
			name: "Should respect HDD AC for ANY SC",
			args: args{
				nc: newNodeCapacity(nodeName,
					[]accrd.AvailableCapacity{testACHDD1, testACSSD1},
					nil),
				vol: getTestVol(nodeName, testSmallSize, apiV1.StorageClassAny),
			},
			want: &testACHDD1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.nc.selectACForVolume(tt.args.vol)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectACForVolume() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_nodeCapacity_String(t *testing.T) {
	var (
		testACHDD1 = *getTestAC(nodeName, testSmallSize, apiV1.StorageClassHDD)
		nc         = newNodeCapacity(nodeName, []accrd.AvailableCapacity{testACHDD1}, nil)
	)

	str := nc.String()
	assert.True(t, strings.Contains(str, testACHDD1.Name))
}
