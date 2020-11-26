package drive

import (
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Controller to reconcile drive custom resource
type Controller struct {
	k8sClient	*k8s.KubeClient
	nodeId    	string
	log 		*logrus.Entry
}

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient, node ID and logrus logger
// Returns an instance of Controller
func NewController(k8sClient *k8s.KubeClient, nodeId string, log *logrus.Logger) *Controller {
	return &Controller{
		k8sClient: k8sClient,
		nodeId:    nodeId,
		log:       log.WithField("component", "Controller"),
	}
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&drivecrd.Drive{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return c.filterCRs(e.Object)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return c.filterCRs(e.Object)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return c.filterCRs(e.ObjectOld)
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return c.filterCRs(e.Object)
			},
		}).
		Complete(c)
}

func (c *Controller) filterCRs(obj runtime.Object) bool {
	if drive, ok := obj.(*drivecrd.Drive); ok {
		if drive.Spec.NodeId == c.nodeId {
			return true
		}
	}
	return false
}

// Reconcile reconciles Drive custom resources
func (ctr *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	// customize logging
	log := ctr.log.WithFields(logrus.Fields{"method": "drive/Reconcile", "name": req.Name})

	// TODO what is in req?
	log.Infof("Request: %s", req)

	return ctrl.Result{}, nil
}
