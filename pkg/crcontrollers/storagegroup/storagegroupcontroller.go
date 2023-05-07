package storagegroup

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	sgcrd "github.com/dell/csi-baremetal/api/v1/storagegroupcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

const (
	contextTimeoutSeconds = 60
)

// Controller to reconcile storagegroup custom resource
type Controller struct {
	client *k8s.KubeClient
	log    *logrus.Entry
}

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, log *logrus.Logger) *Controller {
	c := &Controller{
		client: client,
		log:    log.WithField("component", "StorageGroupController"),
	}
	return c
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sgcrd.StorageGroup{}).
		WithOptions(controller.Options{}).
		Complete(c)
}

// Reconcile reconciles StorageGroup custom resources
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(ctx, contextTimeoutSeconds*time.Second)
	defer cancelFn()

	// read name
	name := req.Name
	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "Reconcile", "name": name})

	// obtain corresponding storage group
	storageGroup := &sgcrd.StorageGroup{}
	if err := c.client.ReadCR(ctx, name, "", storageGroup); err != nil {
		log.Warningf("Failed to read StorageGroup %s", name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Infof("Reconcile StorageGroup: %v", storageGroup)
	return ctrl.Result{}, nil
}
