package controller

import (
	"context"
	"strings"

	"github.com/coreos/rkt/tests/testutils/logger"
	"github.com/sirupsen/logrus"
	v13 "k8s.io/api/core/v1"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	v1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewControllerServer(client client.Client) *CSIControllerServer {
	return &CSIControllerServer{client}
}

// interface implementation for ControllerServer
type CSIControllerServer struct {
	client.Client
}

func (s CSIControllerServer) CreateVolume(context.Context, *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	//Temporary to pass linter
	_, _ = s.getPods(context.TODO(), "", "")
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) DeleteVolume(context.Context, *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) ControllerPublishVolume(context.Context, *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) ControllerUnpublishVolume(context.Context, *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) ValidateVolumeCapabilities(context.Context, *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) CreateSnapshot(context.Context, *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) DeleteSnapshot(context.Context, *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) ListSnapshots(context.Context, *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) ControllerExpandVolume(context.Context, *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
func (s CSIControllerServer) CreateVolumeCRD(ctx context.Context, volume api.Volume, namespace string) (*v1.Volume, error) {
	vol := &v1.Volume{
		TypeMeta: v12.TypeMeta{
			Kind:       "Volume",
			APIVersion: "volume.dell.com/v1",
		},
		ObjectMeta: v12.ObjectMeta{
			//Currently name is volume id
			Name:      volume.Id,
			Namespace: namespace,
		},
		Spec:   v1.VolumeSpec{Volume: volume},
		Status: v1.VolumeStatus{},
	}
	instance := &v1.Volume{}
	err := s.Get(ctx, client.ObjectKey{Name: volume.Id, Namespace: namespace}, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			e := s.Create(ctx, vol)
			if e != nil {
				return nil, e
			}
		} else {
			return nil, err
		}
	}
	return vol, nil
}

func (s CSIControllerServer) ReadVolume(ctx context.Context, name string, namespace string) (*v1.Volume, error) {
	volume := v1.Volume{}
	err := s.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &volume)
	if err != nil {
		return nil, err
	}
	return &volume, nil
}

func (s CSIControllerServer) ReadVolumeList(ctx context.Context, namespace string) (*v1.VolumeList, error) {
	volumes := v1.VolumeList{}
	err := s.List(ctx, &volumes, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	return &volumes, nil
}

func (s *CSIControllerServer) getPods(ctx context.Context, mask string, namespace string) ([]*v13.Pod, error) {
	pods := v13.PodList{}
	err := s.List(ctx, &pods, client.InNamespace(namespace))
	logrus.Info(pods)
	if err != nil {
		logger.Errorf("Failed to get podes: %v", err)
		return nil, err
	}
	p := make([]*v13.Pod, 0)
	for i := range pods.Items {
		podName := pods.Items[i].ObjectMeta.Name
		if strings.Contains(podName, mask) {
			p = append(p, &pods.Items[i])
		}
	}
	return p, nil
}
