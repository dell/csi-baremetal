package extender

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	coreV1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

var (
	testLogger = logrus.New()
	testCtx    = context.Background()

	testNs          = "default"
	testProvisioner = "baremetal-csi"
	testSCName1     = "baremetal-sc-qwe"
	testSCName2     = "another-one-sc"

	testSizeGb       int64 = 10
	testSizeStr            = fmt.Sprintf("%dG", testSizeGb)
	testStorageType        = v1.StorageClassHDD
	testCSIVolumeSrc       = coreV1.CSIVolumeSource{
		Driver:           testProvisioner,
		VolumeAttributes: map[string]string{base.SizeKey: testSizeStr, base.StorageTypeKey: testStorageType},
	}

	testSC1 = storageV1.StorageClass{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      testSCName1,
			Namespace: testNs,
		},
		Provisioner: testProvisioner,
		Parameters:  map[string]string{base.StorageTypeKey: testStorageType},
	}

	testSC2 = storageV1.StorageClass{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      testSCName2,
			Namespace: testNs,
		},
		Provisioner: "another-provisioner",
		Parameters:  map[string]string{base.StorageTypeKey: "another-storage"},
	}

	testPVCTypeMeta = metaV1.TypeMeta{
		Kind:       "PersistentVolumeClaim",
		APIVersion: "v1",
	}

	testPVC1Name = "pvc-with-plugin"
	testPVC1     = coreV1.PersistentVolumeClaim{
		TypeMeta: testPVCTypeMeta,
		ObjectMeta: metaV1.ObjectMeta{
			Name:      testPVC1Name,
			Namespace: testNs,
		},
		Spec: coreV1.PersistentVolumeClaimSpec{
			StorageClassName: &testSCName1,
			Resources: coreV1.ResourceRequirements{
				Requests: coreV1.ResourceList{
					coreV1.ResourceStorage: *resource.NewQuantity(testSizeGb*1024, resource.DecimalSI),
				},
			},
		},
	}

	testPVC2Name = "not-a-plugin-pvc"
	testPVC2     = coreV1.PersistentVolumeClaim{
		TypeMeta: testPVCTypeMeta,
		ObjectMeta: metaV1.ObjectMeta{
			Name:      testPVC2Name,
			Namespace: testNs,
		},
		Spec: coreV1.PersistentVolumeClaimSpec{},
	}

	testPodName = "pod1"
	testPod     = coreV1.Pod{
		TypeMeta:   metaV1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metaV1.ObjectMeta{Name: testPodName, Namespace: testNs},
		Spec:       coreV1.PodSpec{},
	}
)

func TestExtender_gatherVolumesByProvisioner_Success(t *testing.T) {
	e := setup(t)
	pod := testPod
	// append inlineVolume
	pod.Spec.Volumes = append(pod.Spec.Volumes, coreV1.Volume{
		VolumeSource: coreV1.VolumeSource{CSI: &testCSIVolumeSrc},
	})
	// append testPVC1
	pod.Spec.Volumes = append(pod.Spec.Volumes, coreV1.Volume{
		VolumeSource: coreV1.VolumeSource{
			PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC1Name,
			},
		},
	})
	// append testPVC2
	pod.Spec.Volumes = append(pod.Spec.Volumes, coreV1.Volume{
		VolumeSource: coreV1.VolumeSource{
			PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC2Name,
			},
		},
	})
	// create PVCs and SC
	applyObjs(t, e.k8sClient, &testPVC1, &testPVC2, &testSC1)

	volumes, err := e.gatherVolumesByProvisioner(testCtx, &pod)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(volumes))
}

func TestExtender_gatherVolumesByProvisioner_Fail(t *testing.T) {
	e := setup(t)

	// sc mapping empty
	pod := testPod
	volumes, err := e.gatherVolumesByProvisioner(testCtx, &pod)
	assert.Nil(t, volumes)
	assert.NotNil(t, err)

	// constructVolumeFromCSISource failed
	pod = testPod
	badCSIVolumeSrc := testCSIVolumeSrc
	badCSIVolumeSrc.VolumeAttributes = map[string]string{}
	// append inlineVolume
	pod.Spec.Volumes = append(pod.Spec.Volumes, coreV1.Volume{
		VolumeSource: coreV1.VolumeSource{CSI: &badCSIVolumeSrc},
	})
	// create SC
	applyObjs(t, e.k8sClient, &testSC1)

	volumes, err = e.gatherVolumesByProvisioner(testCtx, &pod)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(volumes))
	assert.True(t, volumes[0].Ephemeral)
	// any because storageType key missing in csi volume sourc
	assert.Equal(t, v1.StorageClassAny, volumes[0].StorageClass)

	// unable to read PVCs (bad namespace)
	pod.Namespace = "unexisted-namespace"
	// append testPVC1
	pod.Spec.Volumes = append(pod.Spec.Volumes, coreV1.Volume{
		VolumeSource: coreV1.VolumeSource{
			PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC1Name,
			},
		},
	})
	volumes, err = e.gatherVolumesByProvisioner(testCtx, &pod)
	assert.Nil(t, volumes)
	assert.NotNil(t, err)

	// PVC doesn't contain information about size
	pod.Namespace = testNs
	pvcWithoutSize := testPVC1
	delete(pvcWithoutSize.Spec.Resources.Requests, coreV1.ResourceStorage)
	applyObjs(t, e.k8sClient, &pvcWithoutSize)

	pod.Spec.Volumes = []coreV1.Volume{{
		VolumeSource: coreV1.VolumeSource{
			PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC1Name,
			},
		},
	}}

	volumes, err = e.gatherVolumesByProvisioner(testCtx, &pod)
	assert.Nil(t, err)
	assert.NotNil(t, volumes)
	assert.Equal(t, 1, len(volumes))
	assert.Equal(t, int64(0), volumes[0].Size)
}

func TestExtender_constructVolumeFromCSISource_Success(t *testing.T) {
	e := setup(t)
	expectedSize, err := util.StrToBytes(testSizeStr)
	assert.Nil(t, err)
	expectedVolume := &genV1.Volume{
		StorageClass: util.ConvertStorageClass(testStorageType),
		Size:         expectedSize,
		Ephemeral:    true,
	}

	curr, err := e.constructVolumeFromCSISource(&testCSIVolumeSrc)
	assert.Nil(t, err)
	assert.Equal(t, expectedVolume, curr)

}

func TestExtender_constructVolumeFromCSISource_Fail(t *testing.T) {
	var (
		e = setup(t)
		v = testCSIVolumeSrc
	)

	// missing storage type
	v.VolumeAttributes = map[string]string{}
	expected := &genV1.Volume{StorageClass: v1.StorageClassAny, Ephemeral: true}

	curr, err := e.constructVolumeFromCSISource(&v)
	assert.NotNil(t, curr)
	assert.Equal(t, expected, curr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to detect storage class from attributes")

	// missing size
	v.VolumeAttributes[base.StorageTypeKey] = testStorageType
	expected = &genV1.Volume{StorageClass: util.ConvertStorageClass(testStorageType), Ephemeral: true}
	curr, err = e.constructVolumeFromCSISource(&v)
	assert.NotNil(t, curr)
	assert.Equal(t, expected, curr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to detect size from attributes")

	// unable to convert size
	v.VolumeAttributes[base.StorageTypeKey] = testStorageType
	sizeStr := "12S12"
	v.VolumeAttributes[base.SizeKey] = sizeStr
	expected = &genV1.Volume{StorageClass: util.ConvertStorageClass(testStorageType), Ephemeral: true}
	curr, err = e.constructVolumeFromCSISource(&v)
	assert.NotNil(t, curr)
	assert.Equal(t, expected, curr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), sizeStr)
}

func TestExtender_filterSuccess(t *testing.T) {
	var (
		node1Name = "NODE-1"
		node2Name = "NODE-2"
		node3Name = "NODE-3"
		node1UID  = "node-1111-uuid"
		node2UID  = "node-2222-uuid"
		node3UID  = "node-3333-uuid"

		e *Extender
	)

	nodes := []coreV1.Node{
		{ObjectMeta: metaV1.ObjectMeta{UID: types.UID(node1UID), Name: node1Name}},
		{ObjectMeta: metaV1.ObjectMeta{UID: types.UID(node2UID), Name: node2Name}},
		{ObjectMeta: metaV1.ObjectMeta{UID: types.UID(node3UID), Name: node3Name}},
	}

	// empty volumes
	e = setup(t)
	matched, failed, err := e.filter(testCtx, nodes, nil)
	assert.Nil(t, err)
	assert.Nil(t, failed)
	assert.Equal(t, len(nodes), len(matched))

	// different scenarios
	e = setup(t)

	acs := []accrd.AvailableCapacity{
		// NODE-1 ACs, HDD[50Gb, 100Gb]
		*e.k8sClient.ConstructACCR(uuid.New().String(),
			genV1.AvailableCapacity{NodeId: node1UID, StorageClass: v1.StorageClassHDD, Size: 50*int64(util.GBYTE) + int64(util.MBYTE)}),
		*e.k8sClient.ConstructACCR(uuid.New().String(),
			genV1.AvailableCapacity{NodeId: node1UID, StorageClass: v1.StorageClassHDD, Size: 100*int64(util.GBYTE) + int64(util.MBYTE)}),
		// NODE-2 ACs, HDD[100Gb], SSD[50Gb]
		*e.k8sClient.ConstructACCR(uuid.New().String(),
			genV1.AvailableCapacity{NodeId: node2UID, StorageClass: v1.StorageClassHDD, Size: 100*int64(util.GBYTE) + int64(util.MBYTE)}),
		*e.k8sClient.ConstructACCR(uuid.New().String(),
			genV1.AvailableCapacity{NodeId: node2UID, StorageClass: v1.StorageClassSSD, Size: 50*int64(util.GBYTE) + int64(util.MBYTE)}),
		// NODE-3 ACs, HDDLVG[150Gb], SSDLVG[100Gb]
		*e.k8sClient.ConstructACCR(uuid.New().String(),
			genV1.AvailableCapacity{NodeId: node3UID, StorageClass: v1.StorageClassHDDLVG, Size: 150 * int64(util.GBYTE)}),
		*e.k8sClient.ConstructACCR(uuid.New().String(),
			genV1.AvailableCapacity{NodeId: node3UID, StorageClass: v1.StorageClassSSDLVG, Size: 100 * int64(util.GBYTE)}),
	}

	// create all AC
	for _, ac := range acs {
		assert.Nil(t, e.k8sClient.Create(testCtx, &ac))
	}

	testCases := []struct {
		Volumes           []*genV1.Volume
		ExpectedNodeNames []string
		Msg               string
	}{
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassHDD, Size: 50 * int64(util.GBYTE)},
				{StorageClass: v1.StorageClassHDD, Size: 100 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{node1Name},
			Msg:               "Volumes: HDD[50Gb, 100Gb]; Expected nodes: [NODE-1]",
		},
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassSSD, Size: 50 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{node2Name},
			Msg:               "Volumes: SSD[50Gb]; Expected nodes: [NODE-2]",
		},
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassAny, Size: 150 * int64(util.GBYTE)},
				{StorageClass: v1.StorageClassAny, Size: 100 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{node3Name},
			Msg:               "Volumes: HDDLVG[150Gb], SSDLVG[100Gb]; Expected nodes: [NODE-3]",
		},
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassHDD, Size: 80 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{node1Name, node2Name},
			Msg:               "Volumes: HDD[80Gb]; Expected nodes: [NODE-1, NODE-2]",
		},
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassHDDLVG, Size: 50 * int64(util.GBYTE)},
				{StorageClass: v1.StorageClassHDDLVG, Size: 50 * int64(util.GBYTE)},
				{StorageClass: v1.StorageClassHDDLVG, Size: 50 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{node1Name, node3Name},
			Msg:               "Volumes: HDDLVG[50Gb, 50Gb, 50Gb]; Expected nodes: [NODE-1, NODE-3]",
		},
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassHDDLVG, Size: 100 * int64(util.GBYTE)},
				{StorageClass: v1.StorageClassSSDLVG, Size: 50 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{node2Name, node3Name},
			Msg:               "Volumes: HDDLVG[100Gb], SSDLVG[50Gb]; Expected nodes: [NODE-2, NODE-3]",
		},
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassHDDLVG, Size: 100 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{node1Name, node2Name, node3Name},
			Msg:               "Volumes: HDDLVG[100Gb]; Expected nodes: [NODE-1, NODE-2, NODE-3]",
		},
		{
			Volumes: []*genV1.Volume{
				{StorageClass: v1.StorageClassHDDLVG, Size: 100 * int64(util.GBYTE)},
				{StorageClass: v1.StorageClassHDDLVG, Size: 100 * int64(util.GBYTE)},
			},
			ExpectedNodeNames: []string{},
			Msg:               "Volumes: HDDLVG[100Gb], SSDLVG[50Gb]; Expected nodes: []",
		},
	}

	for _, testCase := range testCases {
		matchedNodes, failedNode, err := e.filter(testCtx, nodes, testCase.Volumes)
		assert.Equal(t, len(nodes)-len(matchedNodes), len(failedNode), testCase.Msg)
		matchedNodeNames := getNodeNames(matchedNodes)
		assert.Equal(t, len(testCase.ExpectedNodeNames), len(matchedNodes),
			fmt.Sprintf("Matched nodes %v. Test case: %v", matchedNodeNames, testCase.Msg))
		assert.Nil(t, err, testCase.Msg)

		// check ACRs
		acrList := &acrcrd.AvailableCapacityReservationList{}
		assert.Nil(t, e.k8sClient.ReadList(testCtx, acrList), testCase.Msg)
		if len(testCase.ExpectedNodeNames) > 0 {
			assert.Equal(t, len(testCase.Volumes), len(acrList.Items), testCase.Msg)
		}

		reservedACCount := 0
		for _, acr := range acrList.Items {
			reservedACCount += len(acr.Spec.Reservations)
		}
		assert.Equal(t, len(testCase.ExpectedNodeNames)*len(testCase.Volumes), reservedACCount, testCase.Msg)

		for _, n := range testCase.ExpectedNodeNames {
			assert.True(t, util.ContainsString(matchedNodeNames, n),
				fmt.Sprintf("Matched nodes: %v, msg - %s", matchedNodeNames, testCase.Msg))
		}
		removeAllACRs(e.k8sClient, t)
	}
}

func TestExtender_getSCNameStorageType_Success(t *testing.T) {
	e := setup(t)
	// create 2 storage classes
	applyObjs(t, e.k8sClient, &testSC1, &testSC2)

	m, err := e.scNameStorageTypeMapping(testCtx)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(m))
	assert.Equal(t, m[testSCName1], testStorageType)
}

func TestExtender_getSCNameStorageType_Fail(t *testing.T) {
	e := setup(t)

	m, err := e.scNameStorageTypeMapping(testCtx)
	assert.Nil(t, m)
	assert.NotNil(t, err)
}

func TestExtender_freeACByNodeAndSCMap(t *testing.T) {
	var (
		node1 = "node-1111"
		node2 = "node-2222"
	)
	e := setup(t)
	ac1 := e.k8sClient.ConstructACCR(uuid.New().String(), genV1.AvailableCapacity{NodeId: node1, StorageClass: v1.StorageClassHDD, Size: 100*int64(util.GBYTE) + int64(util.MBYTE)})
	ac2 := e.k8sClient.ConstructACCR(uuid.New().String(), genV1.AvailableCapacity{NodeId: node2, StorageClass: v1.StorageClassHDD, Size: 100*int64(util.GBYTE) + int64(util.MBYTE)})
	acr1 := e.k8sClient.ConstructACRCR(genV1.AvailableCapacityReservation{
		Name:         uuid.New().String(),
		StorageClass: ac1.Spec.StorageClass,
		Size:         0,
		Reservations: []string{ac1.Name},
	})

	applyObjs(t, e.k8sClient, ac1, ac2)

	res, err := e.unreservedACByNodeAndSCMap()
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, res[node1][v1.StorageClassHDD][ac1.Name], ac1)
	assert.Equal(t, res[node2][v1.StorageClassHDD][ac2.Name], ac2)

	e = setup(t)
	applyObjs(t, e.k8sClient, ac1, ac2, acr1)

	res, err = e.unreservedACByNodeAndSCMap()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(res))
	assert.Equal(t, res[node2][v1.StorageClassHDD][ac2.Name], ac2)
}

func TestExtender_scVolumeMapping(t *testing.T) {
	var (
		volumes = []*genV1.Volume{
			{StorageClass: v1.StorageClassHDD},
			{StorageClass: v1.StorageClassHDD},
			{StorageClass: v1.StorageClassHDDLVG},
		}

		e = setup(t)
	)

	res := e.scVolumeMapping(volumes)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, 2, len(res[v1.StorageClassHDD]))
	assert.Equal(t, 1, len(res[v1.StorageClassHDDLVG]))
}

func TestExtender_createACRsSuccess(t *testing.T) {
	var (
		node1 = "node-1111"
		node2 = "node-2222"
		vol1  = &genV1.Volume{StorageClass: v1.StorageClassHDD}
		vol2  = &genV1.Volume{StorageClass: v1.StorageClassHDDLVG}
		ac11  = &accrd.AvailableCapacity{ObjectMeta: metaV1.ObjectMeta{Name: uuid.New().String()},
			Spec: genV1.AvailableCapacity{NodeId: node1, StorageClass: v1.StorageClassHDD}}
		ac12 = &accrd.AvailableCapacity{ObjectMeta: metaV1.ObjectMeta{Name: uuid.New().String()},
			Spec: genV1.AvailableCapacity{NodeId: node1, StorageClass: v1.StorageClassHDDLVG}}
		ac21 = &accrd.AvailableCapacity{ObjectMeta: metaV1.ObjectMeta{Name: uuid.New().String()},
			Spec: genV1.AvailableCapacity{NodeId: node2, StorageClass: v1.StorageClassHDD}}
		ac22 = &accrd.AvailableCapacity{ObjectMeta: metaV1.ObjectMeta{Name: uuid.New().String()},
			Spec: genV1.AvailableCapacity{NodeId: node2, StorageClass: v1.StorageClassHDDLVG}}

		in = map[string]map[*genV1.Volume]*accrd.AvailableCapacity{
			node1: {
				vol1: ac11,
				vol2: ac12,
			},
			node2: {
				vol1: ac21,
				vol2: ac22,
			},
		}

		e = setup(t)
	)

	err := e.createACRs(testCtx, in)
	assert.Nil(t, err)

	acrList := &acrcrd.AvailableCapacityReservationList{}
	assert.Nil(t, e.k8sClient.ReadList(testCtx, acrList))

	assert.Equal(t, 2, len(acrList.Items))

	for _, acr := range acrList.Items {
		assert.Equal(t, 2, len(acr.Spec.Reservations))
		if acr.Spec.StorageClass == v1.StorageClassHDD {
			assert.True(t, util.ContainsString(acr.Spec.Reservations, ac11.Name))
			assert.True(t, util.ContainsString(acr.Spec.Reservations, ac21.Name))
		} else {
			assert.True(t, util.ContainsString(acr.Spec.Reservations, ac12.Name))
			assert.True(t, util.ContainsString(acr.Spec.Reservations, ac22.Name))
		}
	}
}

func setup(t *testing.T) *Extender {
	k, err := k8s.GetFakeKubeClient(testNs, testLogger)
	assert.Nil(t, err)

	kubeClient := k8s.NewKubeClient(k, testLogger, testNs)
	return &Extender{
		k8sClient:   kubeClient,
		namespace:   testNs,
		provisioner: testProvisioner,
		useACRs:     true,
		logger:      testLogger.WithField("component", "Extender"),
	}
}

func applyObjs(t *testing.T, k8sClient *k8s.KubeClient, objs ...runtime.Object) {
	for _, obj := range objs {
		assert.Nil(t, k8sClient.Create(testCtx, obj))
	}
}

func getNodeNames(nodes []coreV1.Node) []string {
	nodeNames := make([]string, 0)
	for _, n := range nodes {
		nodeNames = append(nodeNames, n.Name)
	}
	return nodeNames
}

func removeAllACRs(k *k8s.KubeClient, t *testing.T) {
	acrList := acrcrd.AvailableCapacityReservationList{}
	assert.Nil(t, k.ReadList(testCtx, &acrList))
	for _, acr := range acrList.Items {
		assert.Nil(t, k.DeleteCR(testCtx, &acr))
	}
}
