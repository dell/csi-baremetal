package base

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	apisV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	crdV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
)

// CtxKey variable type uses for keys in context WithValue
type CtxKey string

const (
	// Constant for context request
	RequestUUID CtxKey = "RequestUUID"

	// To avoid linter error
	DefaultVolumeID = "Unknown"

	// Time between attempts to interact with Volume CR
	TickerStep = 500 * time.Millisecond
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

func (k *KubeClient) CreateCR(ctx context.Context, name string, obj runtime.Object) error {
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

	err := k.Get(ctx, k8sClient.ObjectKey{Name: name, Namespace: k.Namespace}, obj)
	if err != nil {
		if k8sError.IsNotFound(err) {
			ll.Infof("Creating CR %s with name %s", obj.GetObjectKind().GroupVersionKind().Kind, name)
			return k.Create(ctx, obj)
		}
		ll.Infof("Unable to check whether CR %s exist or no", name)
		return err
	}
	ll.Infof("CR %s has already exist", name)
	return nil
}

func (k *KubeClient) ReadCR(ctx context.Context, name string, obj runtime.Object) error {
	k.Lock()
	defer k.Unlock()

	return k.Get(ctx, k8sClient.ObjectKey{Name: name, Namespace: k.Namespace}, obj)
}

func (k *KubeClient) ReadList(ctx context.Context, obj runtime.Object) error {
	k.Lock()
	defer k.Unlock()
	k.log.WithField("method", "ReadList").Info("Reading list")

	return k.List(ctx, obj, k8sClient.InNamespace(k.Namespace))
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
	}).Infof("Deleting CR %s, %v", obj.GetObjectKind().GroupVersionKind().Kind, obj)

	return k.Delete(ctx, obj)
}

func (k *KubeClient) ConstructACCR(name string, apiAC api.AvailableCapacity) *accrd.AvailableCapacity {
	return &accrd.AvailableCapacity{
		TypeMeta: apisV1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
		},
		Spec: apiAC,
	}
}

func (k *KubeClient) ConstructLVGCR(name string, apiLVG api.LogicalVolumeGroup) *lvgcrd.LVG {
	return &lvgcrd.LVG{
		TypeMeta: apisV1.TypeMeta{
			Kind:       "LVG",
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
		},
		Spec: apiLVG,
	}
}

func (k *KubeClient) ConstructVolumeCR(name string, apiVolume api.Volume) *volumecrd.Volume {
	return &volumecrd.Volume{
		TypeMeta: apisV1.TypeMeta{
			Kind:       "Volume",
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
		},
		Spec: apiVolume,
	}
}

func (k *KubeClient) ConstructDriveCR(name string, apiDrive api.Drive) *drivecrd.Drive {
	return &drivecrd.Drive{
		TypeMeta: apisV1.TypeMeta{
			Kind:       "Drive",
			APIVersion: crdV1.APIV1Version,
		},
		ObjectMeta: apisV1.ObjectMeta{
			Name:      name,
			Namespace: k.Namespace,
		},
		Spec: apiDrive,
	}
}

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

func (k *KubeClient) UpdateCRWithAttempts(ctx context.Context, obj runtime.Object, attempts int) error {
	ll := k.log.WithField("method", "UpdateCRWithAttempts")

	var (
		err    error
		ticker = time.NewTicker(TickerStep)
	)

	defer ticker.Stop()

	for i := 0; i < attempts; i++ {
		if err = k.UpdateCR(ctx, obj); err == nil {
			return nil
		} else if k8sError.IsNotFound(err) {
			return err
		}
		ll.Warnf("Unable to update volume CR. Attempt %d out of %d with err %v", i, attempts, err)
		<-ticker.C
	}

	return err
}

// GetVGNameByLVGCRName read LVG CR with name lvgCRName and returns LVG CR.Spec.Name
// method is used for LVG based on system VG because system VG name != LVG CR name
// in case of error returns empty string and error
func (k *KubeClient) GetVGNameByLVGCRName(ctx context.Context, lvgCRName string) (string, error) {
	lvgCR := lvgcrd.LVG{}
	if err := k.ReadCR(ctx, lvgCRName, &lvgCR); err != nil {
		return "", err
	}
	return lvgCR.Spec.Name, nil
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
	//register drive crd
	if err := drivecrd.AddToSchemeDrive(scheme); err != nil {
		return nil, err
	}
	//register LVG crd
	if err := lvgcrd.AddToSchemeLVG(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}
