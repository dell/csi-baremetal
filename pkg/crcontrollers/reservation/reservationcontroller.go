package reservation

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	metrics "github.com/dell/csi-baremetal/pkg/metrics/common"
)

const (
	contextTimeoutSeconds = 60
)

// Controller to reconcile aviliablecapacityreservation custom resource
type Controller struct {
	client        *k8s.KubeClient
	crHelper      *k8s.CRHelper
	log           *logrus.Entry
}

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient, node ID and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, log *logrus.Logger) *Controller {
	return &Controller{
		client:        client,
		crHelper:      k8s.NewCRHelper(client, log),
		log:           log.WithField("component", "ReservationController"),
	}
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&acrcrd.AvailableCapacityReservation{}).
		Complete(c)
}

// Reconcile reconciles ACR custom resources
func (c *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	defer metrics.ReconcileDuration.EvaluateDurationForType("reservation_controller")()

	ctx, cancelFn := context.WithTimeout(context.Background(), contextTimeoutSeconds*time.Second)
	defer cancelFn()

	// read name
	name := req.Name
	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "Reconcile", "name": name})

	// obtain corresponding reservation
	reservation := &acrcrd.AvailableCapacityReservation{}
	if err := c.client.ReadCR(ctx, name, "", reservation); err != nil {
		log.Errorf("Failed to read available capacity reservation %s CR", name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Infof("Reservation changed: %v", reservation)

	return ctrl.Result{}, nil
}
