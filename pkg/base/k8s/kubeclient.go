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
	ctrl "sigs.k8s.io/controller-runtime"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	crdV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
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
	// AppLabelValue matches CSI CRs with csi-baremetal app
	AppLabelValue = "csi-baremetal"
)

// KubeClient is the extension of k8s client which supports CSI custom recources
type KubeClient struct {
	k8sCl.Client
	log       *logrus.Entry
	Namespace string
	metrics   metrics.Statistic
}

// CRReader is a reader interface for k8s client wrapper
type CRReader interface {
	// ReadCR reads CR
	ReadCR(ctx context.Context, name string, namespace string, obj runtime.Object) error
	// ReadList reads CR list
	ReadList(ctx context.Context, obj runtime.Object) error
}

// NewKubeClient is the constructor for KubeClient struct
// Receives basic k8s client from controller-runtime, logrus logger and namespace where to work
// Returns an instance of KubeClient struct
func NewKubeClient(k8sclient k8sCl.Client, logger *logrus.Logger, namespace string) *KubeClient {
	return &KubeClient{
		Client:    k8sclient,
		log:       logger.WithField("component", "KubeClient"),
		Namespace: namespace,
		metrics:   common.KubeclientDuration,
	}
}

// CreateCR creates provided resource on k8s cluster with checking its existence before
// Receives golang context, name of the created object, and object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) CreateCR(ctx context.Context, name string, obj runtime.Object) error {
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
	ll.Infof("Creating CR %s: %v", crKind, obj)
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
}

// ReadCR reads specified resource from k8s cluster into a pointer of struct that implements runtime.Object
// Receives golang context, name of the read object, and object pointer where to read
// Returns error if something went wrong
func (k *KubeClient) ReadCR(ctx context.Context, name string, namespace string, obj runtime.Object) error {
	defer k.metrics.EvaluateDurationForMethod("ReadCR")()
	if namespace == "" {
		return k.Get(ctx, k8sCl.ObjectKey{Name: name, Namespace: k.Namespace}, obj)
	}
	return k.Get(ctx, k8sCl.ObjectKey{Name: name, Namespace: namespace}, obj)
}

// ReadList reads a list of specified resources into k8s resource List struct (for example v1.PodList)
// Receives golang context, and List object pointer where to read
// Returns error if something went wrong
func (k *KubeClient) ReadList(ctx context.Context, obj runtime.Object) error {
	defer k.metrics.EvaluateDurationForMethod("ReadList")()
	return k.List(ctx, obj)
}

// UpdateCR updates provided resource on k8s cluster
// Receives golang context and updated object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) UpdateCR(ctx context.Context, obj runtime.Object) error {
	defer k.metrics.EvaluateDurationForMethod("UpdateCR")()
	requestUUID := ctx.Value(base.RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}

	k.log.WithFields(logrus.Fields{
		"method":      "UpdateCR",
		"requestUUID": requestUUID.(string),
	}).Infof("Updating CR %s, %v", obj.GetObjectKind().GroupVersionKind().Kind, obj)

	return k.Update(ctx, obj)
}

// DeleteCR deletes provided resource from k8s cluster
// Receives golang context and removable object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) DeleteCR(ctx context.Context, obj runtime.Object) error {
	defer k.metrics.EvaluateDurationForMethod("DeleteCR")()
	requestUUID := ctx.Value(base.RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}

	k.log.WithFields(logrus.Fields{
		"method":      "DeleteCR",
		"requestUUID": requestUUID.(string),
	}).Infof("Deleting CR %s, %v", obj.GetObjectKind().GroupVersionKind().Kind, obj)

	return k.Delete(ctx, obj)
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
func (k *KubeClient) ConstructLVGCR(name string, apiLVG api.LogicalVolumeGroup) *lvgcrd.LogicalVolumeGroup {
	return &lvgcrd.LogicalVolumeGroup{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.LVGKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:   name,
			Labels: constructDefaultAppMap(),
		},
		Spec: apiLVG,
	}
}

// ConstructVolumeCR constructs Volume custom resource from api.Volume struct
// Receives a name for k8s ObjectMeta and an instance of api.Volume struct
// Returns an instance of Volume CR struct
func (k *KubeClient) ConstructVolumeCR(name, namespace, appName string, apiVolume api.Volume) *volumecrd.Volume {
	return &volumecrd.Volume{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.VolumeKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    constructCustomAppMap(appName),
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

// ReadCRWithAttempts reads specified resource from k8s cluster into a pointer of struct that implements runtime.Object
// with specified amount of attempts. Fails right away if resource is not found
// Receives golang context, name of the read object, and object pointer where to read
// Returns error if something went wrong
func (k *KubeClient) ReadCRWithAttempts(name string, namespace string, obj runtime.Object, attempts int) error {
	ll := k.log.WithFields(logrus.Fields{
		"method":   "ReadCRWithAttempts",
		"volumeID": name,
	})

	var (
		err    error
		ticker = time.NewTicker(TickerStep)
	)

	defer ticker.Stop()

	// read volume into v
	for i := 0; i < attempts; i++ {
		if err = k.ReadCR(context.Background(), name, namespace, obj); err == nil {
			return nil
		} else if k8sError.IsNotFound(err) {
			return err
		}
		ll.Warnf("Unable to read CR: %v. Attempt %d out of %d.", err, i, attempts)
		<-ticker.C
	}
	return err
}

// UpdateCRWithAttempts updates provided resource on k8s cluster with specified amount of attempts
// Fails right away if resource is not found or was changed
// Receives golang context and updated object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) UpdateCRWithAttempts(ctx context.Context, obj runtime.Object, attempts int) error {
	ll := k.log.WithField("method", "UpdateCRWithAttempts")

	var (
		err    error
		ticker = time.NewTicker(TickerStep)
		ctxVal = context.WithValue(context.Background(), base.RequestUUID, ctx.Value(base.RequestUUID))
	)

	defer ticker.Stop()

	for i := 0; i < attempts; i++ {
		err = k.UpdateCR(ctxVal, obj)
		if err == nil {
			return nil
		}
		// immediately return if object was removed or modified
		if k8sError.IsNotFound(err) || k8sError.IsConflict(err) {
			return err
		}
		ll.Warnf("Unable to update volume CR. Attempt %d out of %d with err %v", i, attempts, err)
		<-ticker.C
	}

	return err
}

// GetPods returns list of pods which names contain mask
// Receives golang context and mask for pods filtering
// Returns slice of coreV1.Pod or error if something went wrong
// todo use labels instead of mask
func (k *KubeClient) GetPods(ctx context.Context, mask string) ([]*coreV1.Pod, error) {
	defer k.metrics.EvaluateDurationForMethod("GetPods")()
	pods := coreV1.PodList{}

	if err := k.List(ctx, &pods, k8sCl.InNamespace(k.Namespace)); err != nil {
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

	if err := k.List(ctx, &nodes); err != nil {
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

	return scheme, nil
}

// constructAppMap creates the map contains app labels
func constructDefaultAppMap() map[string]string {
	return constructCustomAppMap(AppLabelValue)
}

func constructCustomAppMap(appName string) map[string]string {
	return map[string]string{
		AppLabelKey:      appName,
		AppLabelShortKey: appName,
	}
}
