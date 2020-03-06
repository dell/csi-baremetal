package base

import (
	"context"
	"sync"

	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// CtxKey variable type uses for keys in context WithValue
type CtxKey string

//Constant for context request
const RequestUUID CtxKey = "RequestUUID"

//To avoid linter error
const DefaultVolumeID = "Unknown"

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
