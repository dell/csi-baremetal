package controller

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"os"
	"testing"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"github.com/sirupsen/logrus"
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
	err := volumecrd.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	err = v12.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	err = accrd.AddToSchemeAvailableCapacity(scheme)
	if err != nil {
		panic(err)
	}
	svc = NewControllerService(fake.NewFakeClientWithScheme(scheme), logrus.New())

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

func TestControllerServer_CreateAvailableCapacity(t *testing.T) {
	availableCapacity := api.AvailableCapacity{
		Size:     1000,
		Type:     api.StorageClass_HDD,
		Location: "drive",
		NodeId:   "nodeId",
	}

	availableCapacityCRD, err := svc.CreateAvailableCapacity(ctx, availableCapacity, namespace, name)
	assert.Nil(t, err)
	assert.Equal(t, availableCapacityCRD.Spec, availableCapacity, "Capacities are not equal")
}

func TestControllerServer_ReadCRD(t *testing.T) {
	vol := volumecrd.Volume{}
	err := svc.ReadCRD(ctx, name, namespace, &vol)
	assert.Nil(t, err)
	assert.Equal(t, vol.ObjectMeta.Name, name, "Wrong volume crd")

	ac := accrd.AvailableCapacity{}
	err = svc.ReadCRD(ctx, name, namespace, &ac)
	assert.Nil(t, err)
	assert.Equal(t, ac.ObjectMeta.Name, name, "Wrong volume crd")

	err = svc.ReadCRD(ctx, "notexistingcrd", namespace, &ac)
	assert.NotNil(t, err)
}

func TestControllerServer_ReadListCRD(t *testing.T) {
	volumeList := volumecrd.VolumeList{}
	err := svc.ReadListCRD(ctx, namespace, &volumeList)
	assert.Nil(t, err)
	for _, v := range volumeList.Items {
		assert.Equal(t, v.Namespace, namespace, "Namespaces are not equals")
	}

	capacityList := accrd.AvailableCapacityList{}
	err = svc.ReadListCRD(ctx, namespace, &capacityList)
	assert.Nil(t, err)
	for _, capacity := range capacityList.Items {
		assert.Equal(t, capacity.Namespace, namespace, "Namespaces are not equals")
	}
}

func TestCSIControllerService_UpdateAvailableCapacity(t *testing.T) {
	ac := accrd.AvailableCapacity{}
	err := svc.ReadCRD(ctx, name, namespace, &ac)
	newSize := ac.Spec.Size * 2
	ac.Spec.Size = newSize
	assert.Nil(t, err)
	err = svc.UpdateAvailableCapacity(ctx, ac)
	assert.Nil(t, err)
	assert.Equal(t, ac.Spec.Size, newSize, "Sizes are not equals")

	capacity := ac.DeepCopy()
	err = svc.UpdateAvailableCapacity(ctx, ac)
	assert.Nil(t, err)
	assert.Equal(t, &ac, capacity, "AvailableCapacity is not the same as before update")
}

func TestCSIControllerService_DeleteAvailableCapacity(t *testing.T) {
	ac := accrd.AvailableCapacity{}
	err := svc.ReadCRD(ctx, name, namespace, &ac)
	assert.Nil(t, err)
	err = svc.DeleteAvailableCapacity(ctx, ac)
	assert.Nil(t, err)
	err = svc.ReadCRD(ctx, name, namespace, &ac)
	assert.NotNil(t, err)
}

func TestCSIControllerService_initAvailableCapacityCache(t *testing.T) {
	svc.communicators = make(map[NodeID]api.VolumeManagerClient)

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

func equals(volume api.Volume, volume2 volumecrd.Volume) bool {
	return volume.Id == volume2.Spec.Id &&
		volume.Status == volume2.Spec.Status &&
		volume.Health == volume2.Spec.Health &&
		volume.Location == volume2.Spec.Location &&
		volume.Type == volume2.Spec.Type &&
		volume.Mode == volume2.Spec.Mode &&
		volume.Size == volume2.Spec.Size &&
		volume.Owner == volume2.Spec.Owner
}
