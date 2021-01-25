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
	crCache "sigs.k8s.io/controller-runtime/pkg/cache"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"
	crApiutil "sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	crdV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	nodecrd "github.com/dell/csi-baremetal/api/v1/csibmnodecrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
)

// CtxKey variable type uses for keys in context WithValue
type CtxKey string

const (
	// DefaultVolumeID is the constant to avoid linter error
	DefaultVolumeID = "Unknown"

	// TickerStep is the time between attempts to interact with Volume CR
	TickerStep = 500 * time.Millisecond
)

// KubeClient is the extension of k8s client which supports CSI custom recources
type KubeClient struct {
	k8sCl.Client
	log       *logrus.Entry
	Namespace string
}

// NewKubeClient is the constructor for KubeClient struct
// Receives basic k8s client from controller-runtime, logrus logger and namespace where to work
// Returns an instance of KubeClient struct
func NewKubeClient(k8sclient k8sCl.Client, logger *logrus.Logger, namespace string) *KubeClient {
	return &KubeClient{
		Client:    k8sclient,
		log:       logger.WithField("component", "KubeClient"),
		Namespace: namespace,
	}
}

// CreateCR creates provided resource on k8s cluster with checking its existence before
// Receives golang context, name of the created object, and object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) CreateCR(ctx context.Context, name string, obj runtime.Object) error {
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
func (k *KubeClient) ReadCR(ctx context.Context, name string, obj runtime.Object) error {
	return k.Get(ctx, k8sCl.ObjectKey{Name: name}, obj)
}

// ReadList reads a list of specified resources into k8s resource List struct (for example v1.PodList)
// Receives golang context, and List object pointer where to read
// Returns error if something went wrong
func (k *KubeClient) ReadList(ctx context.Context, obj runtime.Object) error {
	return k.List(ctx, obj)
}

// UpdateCR updates provided resource on k8s cluster
// Receives golang context and updated object that implements k8s runtime.Object interface
// Returns error if something went wrong
func (k *KubeClient) UpdateCR(ctx context.Context, obj runtime.Object) error {
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
			Name: name,
		},
		Spec: apiAC,
	}
}

// ConstructACRCR constructs AvailableCapacityReservation custom resource from api.AvailableCapacityReservation struct
// Receives an instance of api.AvailableCapacityReservation struct
// Returns pointer on AvailableCapacityReservation CR struct
func (k *KubeClient) ConstructACRCR(apiACR api.AvailableCapacityReservation) *acrcrd.AvailableCapacityReservation {
	return &acrcrd.AvailableCapacityReservation{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.AvailableCapacityReservationKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name: apiACR.Name,
		},
		Spec: apiACR,
	}
}

// ConstructLVGCR constructs LVG custom resource from api.LogicalVolumeGroup struct
// Receives a name for k8s ObjectMeta and an instance of api.LogicalVolumeGroup struct
// Returns an instance of LVG CR struct
func (k *KubeClient) ConstructLVGCR(name string, apiLVG api.LogicalVolumeGroup) *lvgcrd.LVG {
	return &lvgcrd.LVG{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.LVGKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name: name,
		},
		Spec: apiLVG,
	}
}

// ConstructVolumeCR constructs Volume custom resource from api.Volume struct
// Receives a name for k8s ObjectMeta and an instance of api.Volume struct
// Returns an instance of Volume CR struct
func (k *KubeClient) ConstructVolumeCR(name string, apiVolume api.Volume) *volumecrd.Volume {
	return &volumecrd.Volume{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.VolumeKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name: name,
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
			Name: name,
		},
		Spec: apiDrive,
	}
}

// ConstructCSIBMNodeCR constructs CSIBMNode custom resource from api.CSIBMNode struct
// Receives a name for k8s ObjectMeta and an instance of api.CSIBMNode struct
// Returns an instance of CSIBMNode CR struct
func (k *KubeClient) ConstructCSIBMNodeCR(name string, csiNode api.CSIBMNode) *nodecrd.CSIBMNode {
	return &nodecrd.CSIBMNode{
		TypeMeta: apisV1.TypeMeta{
			Kind:       crdV1.CSIBMNodeKind,
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name: name,
		},
		Spec: csiNode,
	}
}

// ReadCRWithAttempts reads specified resource from k8s cluster into a pointer of struct that implements runtime.Object
// with specified amount of attempts. Fails right away if resource is not found
// Receives golang context, name of the read object, and object pointer where to read
// Returns error if something went wrong
func (k *KubeClient) ReadCRWithAttempts(name string, obj runtime.Object, attempts int) error {
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
		if err = k.ReadCR(context.Background(), name, obj); err == nil {
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
	pods := coreV1.PodList{}

	if err := k.List(ctx, &pods); err != nil {
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

// GetK8SCachedClient returns k8s client with cache support
func GetK8SCachedClient(stop <-chan struct{}, logger *logrus.Logger) (k8sCl.Client, error) {
	config := ctrl.GetConfigOrDie()

	scheme, err := PrepareScheme()
	if err != nil {
		return nil, err
	}
	// Create the mapper provider
	mapper, err := crApiutil.NewDynamicRESTMapper(config)
	if err != nil {
		return nil, err
	}
	// Create the cache for the cached read client and registering informers
	cache, err := crCache.New(config, crCache.Options{
		Scheme: scheme,
		Mapper: mapper,
	})
	if err != nil {
		return nil, err
	}
	apiReader, err := k8sCl.New(config, k8sCl.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	writeObj := k8sCl.DelegatingClient{
		Reader: &k8sCl.DelegatingReader{
			CacheReader:  cache,
			ClientReader: apiReader,
		},
		Writer:       apiReader,
		StatusClient: apiReader,
	}
	// start cache and wait for sync
	go func() {
		if err := cache.Start(stop); err != nil {
			logger.Errorf("fail to start cache: %v", err)
		}
	}()

	logger.Info("Wait for cache sync")
	cache.WaitForCacheSync(stop)

	return writeObj, nil
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
	// register LVG crd
	if err := lvgcrd.AddToSchemeLVG(scheme); err != nil {
		return nil, err
	}

	// register csi node crd
	if err := nodecrd.AddToSchemeCSIBMNode(scheme); err != nil {
		return nil, err
	}

	return scheme, nil
}
