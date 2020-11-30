package drive

import (
	"context"
	"time"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Controller to reconcile drive custom resource
type Controller struct {
	client   *k8s.KubeClient
	crHelper *k8s.CRHelper
	nodeID   string
	log      *logrus.Entry
}

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient, node ID and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, nodeID string, log *logrus.Logger) *Controller {
	return &Controller{
		client:   client,
		crHelper: k8s.NewCRHelper(client, log),
		nodeID:   nodeID,
		log:      log.WithField("component", "Controller"),
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
		if drive.Spec.NodeId == c.nodeID {
			return true
		}
	}
	return false
}

// Reconcile reconciles Drive custom resources
func (c *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	// read name
	driveName := req.Name
	// TODO why do we need 60 seconds here?
	// create context
	ctx, cancelFn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFn()

	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "drive/Reconcile", "name": driveName})

	// obtain corresponding drive
	drive := &drivecrd.Drive{}
	if err := c.client.ReadCR(ctx, driveName, drive); err != nil {
		log.Errorf("Failed to read Drive %s CR", driveName)
		// TODO is this correct error here?
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Infof("Drive changed: %v", drive)

	usage := drive.Spec.GetUsage()
	health := drive.Spec.GetHealth()
	id := drive.Spec.GetUUID()
	isChanged := true

	switch usage {
	case apiV1.DriveUsageInUse:
		if health == apiV1.HealthSuspect || health == apiV1.HealthBad {
			drive.Spec.Usage = apiV1.DriveUsageReleasing
			// TODO update health of volumes
			break
		}
		isChanged = false
	case apiV1.DriveUsageReleased:
		status := drive.Annotations[apiV1.DriveAnnotationReplacement]
		if status == apiV1.DriveAnnotationReplacementReady {
			// TODO need to update annotations for related volumes
			// TODO might need to check CSI status here since volume might not be removed
			volume := c.crHelper.GetVolumeByLocation(id)
			if volume == nil || volume.Spec.CSIStatus == apiV1.Removed {
				// TODO need to call drive manager to start LED locate
				drive.Spec.Usage = apiV1.DriveUsageRemoved
			} else {
				drive.Spec.Usage = apiV1.DriveUsageRemoving
			}
			break
		}
		isChanged = false
	case apiV1.DriveUsageRemoving:
		// TODO need to check CSI status here since volume might not be removed
		volume := c.crHelper.GetVolumeByLocation(id)
		if volume == nil || volume.Spec.CSIStatus == apiV1.Removed {
			// TODO need to call drive manager to start LED locate
			drive.Spec.Usage = apiV1.DriveUsageRemoved
			break
		}
		isChanged = false
	}

	// update drive CR if needed
	if isChanged {
		if err := c.client.UpdateCR(ctx, drive); err != nil {
			log.Errorf("Failed to read Drive %s CR", driveName)
			// TODO is this correct error here?
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	return ctrl.Result{}, nil
}
