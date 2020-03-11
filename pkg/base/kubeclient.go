package base

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
)

// CtxKey variable type uses for keys in context WithValue
type CtxKey string

const (
	//Constant for context request
	RequestUUID CtxKey = "RequestUUID"

	//To avoid linter error
	DefaultVolumeID = "Unknown"

	//To update Volume CR's status
	VolumeStatusAnnotationKey = "dell.emc.csi/volume-status"
)

type KubeClient struct {
	k8sClient.Client
	log *logrus.Entry
	//mutex for crd request
	Namespace string
	sync.Mutex
}

func NewKubeClient(k8sclient k8sClient.Client, logger *logrus.Logger, namespace string) *KubeClient {
	return &KubeClient{
		Client:    k8sclient,
		log:       logger.WithField("component", "KubeClient"),
		Namespace: namespace,
	}
}

//CreateCR return true if object already exist in cluster
func (k *KubeClient) CreateCR(ctx context.Context, obj runtime.Object, name string) error {
	k.Lock()
	defer k.Unlock()

	requestUUID := ctx.Value(RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}

	ll := k.log.WithFields(logrus.Fields{
		"method":      "CreateCR",
		"requestUUID": requestUUID.(string),
	})
	ll.Infof("Creating CR %s with name %s", obj.GetObjectKind().GroupVersionKind().Kind, name)
	err := k.Get(ctx, k8sClient.ObjectKey{Name: name, Namespace: k.Namespace}, obj)
	if err != nil {
		if k8sError.IsNotFound(err) {
			e := k.Create(ctx, obj)
			if e != nil {
				return e
			}
		} else {
			return err
		}
	}
	ll.Infof("CR with name %s was created successfully", name)
	return nil
}

func (k *KubeClient) ReadCR(ctx context.Context, name string, obj runtime.Object) error {
	k.Lock()
	defer k.Unlock()
	k.log.WithField("method", "ReadCR").Info("Read CR")

	return k.Get(ctx, k8sClient.ObjectKey{Name: name, Namespace: k.Namespace}, obj)
}

//TODO AK8S-381 Add field selector to ReadList method
func (k *KubeClient) ReadList(ctx context.Context, object runtime.Object) error {
	k.Lock()
	defer k.Unlock()
	k.log.WithField("method", "ReadList").Info("Reading list")

	return k.List(ctx, object, k8sClient.InNamespace(k.Namespace))
}

func (k *KubeClient) UpdateCR(ctx context.Context, obj runtime.Object) error {
	k.Lock()
	defer k.Unlock()

	requestUUID := ctx.Value(RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}

	k.log.WithFields(logrus.Fields{
		"method":      "UpdateCR",
		"requestUUID": requestUUID.(string),
	}).Infof("Updating CR %s", obj.GetObjectKind().GroupVersionKind().Kind)

	return k.Update(ctx, obj)
}

func (k *KubeClient) DeleteCR(ctx context.Context, obj runtime.Object) error {
	k.Lock()
	defer k.Unlock()

	requestUUID := ctx.Value(RequestUUID)
	if requestUUID == nil {
		requestUUID = DefaultVolumeID
	}

	k.log.WithFields(logrus.Fields{
		"method":      "DeleteCR",
		"requestUUID": requestUUID.(string),
	}).Infof("Deleting CR %s", obj.GetObjectKind().GroupVersionKind().Kind)

	return k.Delete(ctx, obj)
}

func (k *KubeClient) ChangeVolumeStatus(volumeID string, newStatus api.OperationalStatus) error {
	ll := k.log.WithFields(logrus.Fields{
		"method":   "changeVolumeStatus",
		"volumeID": volumeID,
	})

	var (
		err          error
		newStatusStr = api.OperationalStatus_name[int32(newStatus)]
		v            = &volumecrd.Volume{}
		attempts     = 10
		timeout      = 500 * time.Millisecond
		ctxV         = context.WithValue(context.Background(), RequestUUID, volumeID)
		ticker       = time.NewTicker(timeout)
	)

	defer ticker.Stop()

	ll.Infof("Try to set status to %s", newStatusStr)

	// read volume into v
	for i := 0; i < attempts; i++ {
		if err = k.ReadCR(ctxV, volumeID, v); err == nil {
			break
		}
		ll.Warnf("Unable to read CR: %v. Attempt %d out of %d.", err, i, attempts)
		<-ticker.C
	}

	// change status
	v.Spec.Status = newStatus
	if v.ObjectMeta.Annotations == nil {
		v.ObjectMeta.Annotations = make(map[string]string, 1)
	}
	v.ObjectMeta.Annotations[VolumeStatusAnnotationKey] = newStatusStr

	for i := 0; i < attempts; i++ {
		// update volume with new status
		if err = k.UpdateCR(ctxV, v); err == nil {
			return nil
		}
		ll.Warnf("Unable to update volume CR (set status to %s). Attempt %d out of %d",
			api.OperationalStatus_name[int32(newStatus)], i, attempts)
		<-ticker.C
	}

	return fmt.Errorf("unable to persist status to %s for volume %s", newStatusStr, volumeID)
}

func GetK8SClient() (k8sClient.Client, error) {
	scheme, err := prepareScheme()
	if err != nil {
		return nil, err
	}
	cl, err := k8sClient.New(ctrl.GetConfigOrDie(), k8sClient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return cl, err
}

func GetFakeKubeClient(testNs string) (*KubeClient, error) {
	scheme, err := prepareScheme()
	if err != nil {
		return nil, err
	}
	return NewKubeClient(fake.NewFakeClientWithScheme(scheme), logrus.New(), testNs), nil
}

func prepareScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	//register volume crd
	if err := volumecrd.AddToScheme(scheme); err != nil {
		return nil, err
	}
	//register available capacity crd
	if err := accrd.AddToSchemeAvailableCapacity(scheme); err != nil {
		return nil, err
	}
	//register available drive crd
	if err := drivecrd.AddToSchemeDrive(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}
