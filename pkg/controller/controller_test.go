package controller

import (
	"os"
	"testing"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	v1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	v12 "k8s.io/api/core/v1"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	svc *CSIControllerService
	ctx context.Context
)

const (
	name      = "id"
	namespace = "default"
)

func TestMain(m *testing.M) {
	scheme := runtime.NewScheme()
	err := v1.AddToScheme(scheme)
	if err != nil {
		os.Exit(1)
	}

	err = v12.AddToScheme(scheme)
	if err != nil {
		os.Exit(1)
	}
	svc = NewControllerService(fake.NewFakeClientWithScheme(scheme))

	ctx = context.TODO()
	code := m.Run()
	os.Exit(code)
}

func TestControllerServer_CreateVolumeCRD(t *testing.T) {
	volume := api.Volume{
		Id:       name,
		Owner:    "pod",
		Size:     1000,
		Mode:     0,
		Type:     "Type",
		Location: "location",
		Health:   0,
		Status:   0,
	}

	volumeCRD, err := svc.CreateVolumeCRD(ctx, volume, namespace)
	assert.Nil(t, err)

	if !(equals(volume, *volumeCRD)) {
		t.Error("Volumes are not equal")
	}
}

func TestControllerServer_ReadVolume(t *testing.T) {
	volume, err := svc.ReadVolume(ctx, name, namespace)
	assert.Nil(t, err)
	assert.Equal(t, volume.ObjectMeta.Name, name, "Wrong volume crd")
}

func TestControllerServer_ReadVolumeList(t *testing.T) {
	volumeList, err := svc.ReadVolumeList(ctx, namespace)
	assert.Nil(t, err)

	for _, v := range volumeList.Items {
		assert.Equal(t, v.Namespace, namespace, "Namespaces are not equals")
	}
}

func TestCSIControllerServer_getPods(t *testing.T) {
	pods, e := svc.getPods(ctx, name)
	assert.Nil(t, e)
	assert.Empty(t, pods)
	pod := &v12.Pod{
		ObjectMeta: v13.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v12.PodSpec{},
	}
	err := svc.Create(ctx, pod)
	assert.Nil(t, err)
	pods, e = svc.getPods(ctx, name)
	assert.Contains(t, pods, pod)
	assert.Nil(t, e)
	copypod := pod.DeepCopy()
	pod.Name = name + "1"
	err = svc.Create(ctx, pod)
	assert.Nil(t, err)
	pods, e = svc.getPods(ctx, name)
	assert.Contains(t, pods, pod, copypod)
	assert.Nil(t, e)
	pod.Name = "pod"
	err = svc.Create(ctx, pod)
	assert.Nil(t, err)
	pods, e = svc.getPods(ctx, name)
	assert.NotContains(t, pods, pod)
	assert.Nil(t, e)
}

func equals(volume api.Volume, volume2 v1.Volume) bool {
	return volume.Id == volume2.Spec.Volume.Id &&
		volume.Status == volume2.Spec.Volume.Status &&
		volume.Health == volume2.Spec.Volume.Health &&
		volume.Location == volume2.Spec.Volume.Location &&
		volume.Type == volume2.Spec.Volume.Type &&
		volume.Mode == volume2.Spec.Volume.Mode &&
		volume.Size == volume2.Spec.Volume.Size &&
		volume.Owner == volume2.Spec.Volume.Owner
}
