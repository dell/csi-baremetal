package controller

import (
	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	v1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

var (
	server *CSIControllerServer
	ctx    context.Context
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
	server = NewControllerServer(fake.NewFakeClientWithScheme(scheme))
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
	volumeCRD, err := server.CreateVolumeCRD(ctx, volume, namespace)
	if err != nil {
		t.Fatalf("Error creating crd: %v", err)
	}
	if !(equals(volume, *volumeCRD)) {
		t.Error("Volumes are not equal")
	}
}

func TestControllerServer_ReadVolume(t *testing.T) {
	volume, err := server.ReadVolume(ctx, name, namespace)
	if err != nil {
		t.Fatalf("Error reading crd: %v", err)
	}
	if volume.ObjectMeta.Name != name {
		t.Error("Wrong volume crd")
	}
}

func TestControllerServer_ReadVolumeList(t *testing.T) {
	volumeList, err := server.ReadVolumeList(ctx, namespace)
	if err != nil {
		t.Fatalf("Error reading crd: %v", err)
	}
	for _, v := range volumeList.Items {
		if v.Namespace != namespace {
			t.Error("Namespaces are not equals")
		}
	}
}

func equals(volume api.Volume, volume2 v1.Volume) bool {
	if volume.Id != volume2.Spec.Volume.Id {
		return false
	}
	if volume.Status != volume2.Spec.Volume.Status {
		return false
	}
	if volume.Health != volume2.Spec.Volume.Health {
		return false
	}
	if volume.Location != volume2.Spec.Volume.Location {
		return false
	}
	if volume.Type != volume2.Spec.Volume.Type {
		return false
	}
	if volume.Mode != volume2.Spec.Volume.Mode {
		return false
	}
	if volume.Size != volume2.Spec.Volume.Size {
		return false
	}
	if volume.Owner != volume2.Spec.Volume.Owner {
		return false
	}
	return true
}
