package scheduler

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

var (
	testLogger = logrus.New()

	testSCName = pluginNameMask + "-hddlvg"

	testSizeStr      = "10G"
	testStorageType  = "HDD"
	testCSIVolumeSrc = k8sV1.CSIVolumeSource{
		Driver:           fmt.Sprintf("%s-hdd", pluginNameMask),
		VolumeAttributes: map[string]string{base.SizeKey: testSizeStr, base.StorageTypeKey: testStorageType},
	}

	testPVCTypeMeta = metaV1.TypeMeta{
		Kind:       "PersistentVolumeClaim",
		APIVersion: "v1",
	}

	testPVC1Name = "pvc-with-plugin"
	testPVC1     = k8sV1.PersistentVolumeClaim{
		TypeMeta: testPVCTypeMeta,
		ObjectMeta: metaV1.ObjectMeta{
			Name:      testPVC1Name,
			Namespace: namespace,
		},
		Spec: k8sV1.PersistentVolumeClaimSpec{
			StorageClassName: &testSCName,
			Resources: k8sV1.ResourceRequirements{
				Requests: k8sV1.ResourceList{
					k8sV1.ResourceStorage: resource.MustParse(testSizeStr),
				},
			},
		},
	}

	testPVC2Name = "not-a-plugin-pvc"
	testPVC2     = k8sV1.PersistentVolumeClaim{
		TypeMeta: testPVCTypeMeta,
		ObjectMeta: metaV1.ObjectMeta{
			Name:      testPVC2Name,
			Namespace: namespace,
		},
		Spec: k8sV1.PersistentVolumeClaimSpec{},
	}

	testPodName = "pod1"
	testPod     = k8sV1.Pod{
		TypeMeta:   metaV1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metaV1.ObjectMeta{Name: testPodName, Namespace: namespace},
		Spec:       k8sV1.PodSpec{},
	}
)

func TestNewExtender(t *testing.T) {
	e := NewExtender(logrus.New())
	assert.NotNil(t, e)
	assert.NotNil(t, e.k8sClient)
	assert.NotNil(t, e.logger)
	assert.Equal(t, namespace, e.k8sClient.Namespace)
}

func TestExtender_gatherVolumesByProvisioner_Success(t *testing.T) {
	e := setup()
	pod := testPod
	// append inlineVolume
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sV1.Volume{
		VolumeSource: k8sV1.VolumeSource{CSI: &testCSIVolumeSrc},
	})
	// append testPVC1
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sV1.Volume{
		VolumeSource: k8sV1.VolumeSource{
			PersistentVolumeClaim: &k8sV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC1Name,
			},
		},
	})
	// append testPVC2
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sV1.Volume{
		VolumeSource: k8sV1.VolumeSource{
			PersistentVolumeClaim: &k8sV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC2Name,
			},
		},
	})
	// create PVC
	err := applyPVC(e.k8sClient, &testPVC1, &testPVC2)

	volumes, err := e.gatherVolumesByProvisioner(context.Background(), &pod)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(volumes))
}

func TestExtender_gatherVolumesByProvisioner_Fail(t *testing.T) {
	e := setup()

	// constructVolumeFromCSISource failed
	pod := testPod
	badCSIVolumeSrc := testCSIVolumeSrc
	badCSIVolumeSrc.VolumeAttributes = map[string]string{}
	// append inlineVolume
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sV1.Volume{
		VolumeSource: k8sV1.VolumeSource{CSI: &badCSIVolumeSrc},
	})

	volumes, err := e.gatherVolumesByProvisioner(context.Background(), &pod)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(volumes))
	assert.True(t, volumes[0].Ephemeral)
	assert.Equal(t, v1.StorageClassAny, volumes[0].StorageClass)

	// unable to read PVCs (bad namespace)
	pod.Namespace = "unexisted-namespace"
	// append testPVC1
	pod.Spec.Volumes = append(pod.Spec.Volumes, k8sV1.Volume{
		VolumeSource: k8sV1.VolumeSource{
			PersistentVolumeClaim: &k8sV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC1Name,
			},
		},
	})
	volumes, err = e.gatherVolumesByProvisioner(context.Background(), &pod)
	assert.Nil(t, volumes)
	assert.NotNil(t, err)

	// PVC doesn't contain information about size
	pod.Namespace = namespace
	pvcWithoutSize := testPVC1
	delete(pvcWithoutSize.Spec.Resources.Requests, k8sV1.ResourceStorage)
	assert.Nil(t, applyPVC(e.k8sClient, &pvcWithoutSize))

	pod.Spec.Volumes = []k8sV1.Volume{{
		VolumeSource: k8sV1.VolumeSource{
			PersistentVolumeClaim: &k8sV1.PersistentVolumeClaimVolumeSource{
				ClaimName: testPVC1Name,
			},
		},
	}}

	volumes, err = e.gatherVolumesByProvisioner(context.Background(), &pod)
	assert.Nil(t, err)
	assert.NotNil(t, volumes)
	assert.Equal(t, 1, len(volumes))
	assert.Equal(t, int64(0), volumes[0].Size)
}

func TestExtender_constructVolumeFromCSISource_Success(t *testing.T) {
	e := setup()
	expectedSize, err := util.StrToBytes(testSizeStr)
	assert.Nil(t, err)
	expectedVolume := &genV1.Volume{StorageClass: testStorageType, Size: expectedSize, Ephemeral: true}

	curr, err := e.constructVolumeFromCSISource(&testCSIVolumeSrc)
	assert.Nil(t, err)
	assert.Equal(t, expectedVolume, curr)

}

func TestExtender_constructVolumeFromCSISource_Fail(t *testing.T) {
	var (
		e = setup()
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
	expected = &genV1.Volume{StorageClass: testStorageType, Ephemeral: true}
	curr, err = e.constructVolumeFromCSISource(&v)
	assert.NotNil(t, curr)
	assert.Equal(t, expected, curr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unable to detect size from attributes")

	// unable to convert size
	v.VolumeAttributes[base.StorageTypeKey] = testStorageType
	sizeStr := "12S12"
	v.VolumeAttributes[base.SizeKey] = sizeStr
	expected = &genV1.Volume{StorageClass: testStorageType, Ephemeral: true}
	curr, err = e.constructVolumeFromCSISource(&v)
	assert.NotNil(t, curr)
	assert.Equal(t, expected, curr)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), sizeStr)
}

func setup() *Extender {
	k, err := k8s.GetFakeKubeClient(namespace, testLogger)
	if err != nil {
		panic(err)
	}

	e := NewExtender(testLogger)
	e.k8sClient = k8s.NewKubeClient(k, testLogger, namespace)

	return e
}

func applyPVC(k8sClient *k8s.KubeClient, pvcs ...*k8sV1.PersistentVolumeClaim) error {
	for _, pvc := range pvcs {
		pvc := pvc
		if err := k8sClient.Create(context.Background(), pvc); err != nil {
			return err
		}
	}
	return nil
}
