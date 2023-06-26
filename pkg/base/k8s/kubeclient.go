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

package k8s

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	apisV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	crdV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	sgcrd "github.com/dell/csi-baremetal/api/v1/storagegroupcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	checkErr "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/logger/objects"
	"github.com/dell/csi-baremetal/pkg/metrics"
	"github.com/dell/csi-baremetal/pkg/metrics/common"
)

// CtxKey variable type uses for keys in context WithValue
type CtxKey string

const (
	// DefaultVolumeID is the constant to avoid linter error
	DefaultVolumeID = "Unknown"

	// TickerStep is the time between attempts to interact with Volume CR
	TickerStep = 500 * time.Millisecond

	// AppLabelKey matches CSI CRs with csi-baremetal app
	AppLabelKey = "app.kubernetes.io/name"
	// AppLabelShortKey matches CSI CRs with csi-baremetal app
	AppLabelShortKey = "app"
	// ReleaseLabelKey matches CSI CRs with the helm release
	ReleaseLabelKey = "release"
	// AppLabelValue matches CSI CRs with csi-baremetal app
	AppLabelValue = "csi-baremetal"
)

// KubeClient is the extension of k8s client which supports CSI custom recources
type KubeClient struct {
	k8sCl.Client
	objectsLogger objects.ObjectLogger
	Namespace     string
	metrics       metrics.Statistic
	log           *logrus.Entry
}

// CRReader is a reader interface for k8s client wrapper
type CRReader interface {
	// ReadCR reads CR
	ReadCR(ctx context.Context, name string, namespace string, obj k8sCl.Object) error
	// ReadList reads CR list
	ReadList(ctx context.Context, obj k8sCl.ObjectList) error
}

// NewKubeClient is the constructor for KubeClient struct
// Receives basic k8s client from controller-runtime, logrus logger and namespace where to work
// Returns an instance of KubeClient struct
func NewKubeClient(k8sclient k8sCl.Client, logger *logrus.Logger, objectsLogger objects.ObjectLogger, namespace string) *KubeClient {
	return &KubeClient{
		Client:        k8sclient,
		log:           logger.WithField("component", "KubeClient"),
		objectsLogger: objectsLogger,
		Namespace:     namespace,
		metrics:       common.KubeclientDuration,
	}
}

// CreateCR creates provided resource on k8s cluster with checking its existence before
// Receives golang context, name of the created object, and object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) CreateCR(ctx context.Context, name string, obj k8sCl.Object) error {
	defer k.metrics.EvaluateDurationForMethod("CreateCR")()
	requestUUID := ctx.Value(base.RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}
	ll := k.log.WithFields(logrus.Fields{
		"method":      "CreateCR",
		"requestUUID": requestUUID.(string),
	})
	crKind := obj.GetObjectKind().GroupVersionKind().Kind
	ll.Infof("Creating CR '%s': %s", crKind, k.objectsLogger.Log(obj))
	return retry.OnError(retry.DefaultBackoff, checkErr.IsSafeReturnError, func() error {
		err := k.Create(ctx, obj)
		if err != nil {
			if k8sError.IsAlreadyExists(err) {
				ll.Infof("CR %s %s already exist", crKind, name)
				return nil
			}
			ll.Errorf("Unable to create CR %s %s: %v", crKind, name, err)
			return err
		}
		ll.Infof("CR %s %s created", crKind, name)
		return nil
	})
}

// ReadCR reads specified resource from k8s cluster into a pointer of struct that implements runtime.Object
// Receives golang context, name of the read object, and object pointer where to read
// Returns error if something went wrong
func (k *KubeClient) ReadCR(ctx context.Context, name string, namespace string, obj k8sCl.Object) error {
	defer k.metrics.EvaluateDurationForMethod("ReadCR")()
	if namespace == "" {
		namespace = k.Namespace
	}
	return retry.OnError(retry.DefaultBackoff, checkErr.IsSafeReturnError, func() error {
		return k.Get(ctx, k8sCl.ObjectKey{Name: name, Namespace: namespace}, obj)
	})
}

// ReadList reads a list of specified resources into k8s resource List struct (for example v1.PodList)
// Receives golang context, and List object pointer where to read
// Returns error if something went wrong
func (k *KubeClient) ReadList(ctx context.Context, obj k8sCl.ObjectList) error {
	defer k.metrics.EvaluateDurationForMethod("ReadList")()
	return retry.OnError(retry.DefaultBackoff, checkErr.IsSafeReturnError, func() error {
		return k.List(ctx, obj)
	})
}

// UpdateCR updates provided resource on k8s cluster
// Receives golang context and updated object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) UpdateCR(ctx context.Context, obj k8sCl.Object) error {
	defer k.metrics.EvaluateDurationForMethod("UpdateCR")()
	requestUUID := ctx.Value(base.RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}

	k.log.WithFields(logrus.Fields{
		"method":      "UpdateCR",
		"requestUUID": requestUUID,
	}).Infof("Updating CR '%s': %s", obj.GetObjectKind().GroupVersionKind().Kind, k.objectsLogger.Log(obj))

	return retry.OnError(retry.DefaultBackoff, checkErr.IsSafeReturnError, func() error {
		return k.Update(ctx, obj)
	})
}

// DeleteCR deletes provided resource from k8s cluster
// Receives golang context and removable object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) DeleteCR(ctx context.Context, obj k8sCl.Object) error {
	defer k.metrics.EvaluateDurationForMethod("DeleteCR")()
	requestUUID := ctx.Value(base.RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}

	k.log.WithFields(logrus.Fields{
		"method":      "DeleteCR",
		"requestUUID": requestUUID.(string),
	}).Infof("Deleting CR '%s': %s", obj.GetObjectKind().GroupVersionKind().Kind, k.objectsLogger.Log(obj))

	return retry.OnError(retry.DefaultBackoff, checkErr.IsSafeReturnError, func() error {
		return k.Delete(ctx, obj)
	})
}

// ConstructACCR constructs AvailableCapacity custom resource from api.AvailableCapacity struct
// Receives a name for k8s ObjectMeta and an instance of api.AvailableCapacity struct
// Returns an instance of AvailableCapacity CR struct
func (k *KubeClient) ConstructACCR(name string, apiAC api.AvailableCapacity) *accrd.AvailableCapacity {
	return &accrd.AvailableCapacity{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.AvailableCapacityKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:   name,
			Labels: constructDefaultAppMap(),
		},
		Spec: apiAC,
	}
}

// ConstructACRCR constructs AvailableCapacityReservation custom resource from api.AvailableCapacityReservation struct
// Receives name and instance of api.AvailableCapacityReservation struct
// Returns pointer on AvailableCapacityReservation CR struct
func (k *KubeClient) ConstructACRCR(name string, apiACR api.AvailableCapacityReservation) *acrcrd.AvailableCapacityReservation {
	return &acrcrd.AvailableCapacityReservation{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.AvailableCapacityReservationKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:   name,
			Labels: constructDefaultAppMap(),
		},
		Spec: apiACR,
	}
}

// ConstructLVGCR constructs LogicalVolumeGroup custom resource from api.LogicalVolumeGroup struct
// Receives a name for k8s ObjectMeta and an instance of api.LogicalVolumeGroup struct
// Returns an instance of LogicalVolumeGroup CR struct
func (k *KubeClient) ConstructLVGCR(name, storageGroup string, apiLVG api.LogicalVolumeGroup) *lvgcrd.LogicalVolumeGroup {
	return &lvgcrd.LogicalVolumeGroup{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.LVGKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:   name,
			Labels: constructLVGCRLabels(storageGroup),
		},
		Spec: apiLVG,
	}
}

// ConstructVolumeCR constructs Volume custom resource from api.Volume struct
// Receives a name for k8s ObjectMeta and an instance of api.Volume struct
// Returns an instance of Volume CR struct
func (k *KubeClient) ConstructVolumeCR(name, namespace string, labels map[string]string,
	apiVolume api.Volume) *volumecrd.Volume {
	return &volumecrd.Volume{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.VolumeKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: apiVolume,
	}
}

// ConstructDriveCR constructs Drive custom resource from api.Drive struct
// Receives a name for k8s ObjectMeta and an instance of api.Drive struct
// Returns an instance of Drive CR struct
func (k *KubeClient) ConstructDriveCR(name string, apiDrive api.Drive) *drivecrd.Drive {
	return &drivecrd.Drive{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.DriveKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:   name,
			Labels: constructDefaultAppMap(),
		},
		Spec: apiDrive,
	}
}

// ConstructCSIBMNodeCR constructs Node custom resource from api.Node struct
// Receives a name for k8s ObjectMeta and an instance of api.Node struct
// Returns an instance of Node CR struct
func (k *KubeClient) ConstructCSIBMNodeCR(name string, csiNode api.Node) *nodecrd.Node {
	return &nodecrd.Node{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.CSIBMNodeKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:   name,
			Labels: constructDefaultAppMap(),
		},
		Spec: csiNode,
	}
}

// GetPods returns list of pods which names contain mask
// Receives golang context and mask for pods filtering
// Returns slice of coreV1.Pod or error if something went wrong
// todo use labels instead of mask
func (k *KubeClient) GetPods(ctx context.Context, mask string) ([]*coreV1.Pod, error) {
	defer k.metrics.EvaluateDurationForMethod("GetPods")()
	pods := coreV1.PodList{}

	if err := retry.OnError(retry.DefaultBackoff, checkErr.IsSafeReturnError, func() error {
		return k.List(ctx, &pods, k8sCl.InNamespace(k.Namespace))
	}); err != nil {
		return nil, err
	}
	p := make([]*coreV1.Pod, 0)
	for i := range pods.Items {
		podName := pods.Items[i].ObjectMeta.Name
		if strings.Contains(podName, mask) {
			p = append(p, &pods.Items[i])
		}
	}
	return p, nil
}

// GetNodes returns list of nodes
// Receives golang context
// Returns slice of coreV1.Node or error if something went wrong
func (k *KubeClient) GetNodes(ctx context.Context) ([]coreV1.Node, error) {
	defer k.metrics.EvaluateDurationForMethod("GetNodes")()
	nodes := coreV1.NodeList{}

	if err := retry.OnError(retry.DefaultBackoff, checkErr.IsSafeReturnError, func() error {
		return k.List(ctx, &nodes)
	}); err != nil {
		return nil, err
	}

	return nodes.Items, nil
}

// GetSystemDriveUUIDs returns system drives uuid
// Receives golang context
// Returns return slice of string - system drives uuids
func (k *KubeClient) GetSystemDriveUUIDs() []string {
	defer k.metrics.EvaluateDurationForMethod("GetSystemDriveUUIDs")()
	ll := k.log.WithField("method", "GetSystemDriveUUIDs")
	var driveList drivecrd.DriveList
	if err := k.ReadList(context.Background(), &driveList); err != nil {
		ll.Errorf("Failed to read Drive list, error: %v", err)
		return nil
	}
	drivesUUIDs := make([]string, 0)
	for _, drive := range driveList.Items {
		if drive.Spec.IsSystem {
			drivesUUIDs = append(drivesUUIDs, drive.Spec.UUID)
		}
	}
	if len(drivesUUIDs) == 0 {
		ll.Errorf("Failed to collect system drives, there are no system disks")
	}
	return drivesUUIDs
}

// GetK8SClient returns controller-runtime k8s client with modified scheme which includes CSI custom resources
// Returns controller-runtime/pkg/Client which can work with CSI CRs or error if something went wrong
func GetK8SClient() (k8sCl.Client, error) {
	scheme, err := PrepareScheme()
	if err != nil {
		return nil, err
	}
	cl, err := k8sCl.New(ctrl.GetConfigOrDie(), k8sCl.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return cl, err
}

// PrepareScheme registers CSI custom resources to runtime.Scheme
// Returns modified runtime.Scheme or error if something went wrong
func PrepareScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	// register volume crd
	if err := volumecrd.AddToScheme(scheme); err != nil {
		return nil, err
	}
	// register available capacity crd
	if err := accrd.AddToSchemeAvailableCapacity(scheme); err != nil {
		return nil, err
	}
	// register available capacity reservation crd
	if err := acrcrd.AddToSchemeACR(scheme); err != nil {
		return nil, err
	}
	// register drive crd
	if err := drivecrd.AddToSchemeDrive(scheme); err != nil {
		return nil, err
	}
	// register LogicalVolumeGroup crd
	if err := lvgcrd.AddToSchemeLVG(scheme); err != nil {
		return nil, err
	}

	// register csi node crd
	if err := nodecrd.AddToSchemeCSIBMNode(scheme); err != nil {
		return nil, err
	}

	// register csi storagegroup crd
	err := sgcrd.AddToSchemeStorageGroup(scheme)
	if err != nil {
		return nil, err
	}

	return scheme, nil
}

// constructAppMap creates the map contains app labels
func constructDefaultAppMap() (labels map[string]string) {
	labels = map[string]string{
		AppLabelKey:      AppLabelValue,
		AppLabelShortKey: AppLabelValue,
	}
	return
}

func constructLVGCRLabels(storageGroup string) (labels map[string]string) {
	labels = constructDefaultAppMap()
	if storageGroup != "" {
		labels[crdV1.StorageGroupLabelKey] = storageGroup
	}
	return labels
}
