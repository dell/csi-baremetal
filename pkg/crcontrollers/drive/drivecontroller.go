package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/events"
	metricsC "github.com/dell/csi-baremetal/pkg/metrics/common"
)

// Controller to reconcile drive custom resource
type Controller struct {
	client         *k8s.KubeClient
	crHelper       *k8s.CRHelper
	nodeID         string
	driveMgrClient api.DriveServiceClient
	eventRecorder  *events.Recorder
	log            *logrus.Entry
}

const (
	ignore uint8 = 0
	update uint8 = 1
	delete uint8 = 2
)

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient, node ID and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, nodeID string, serviceClient api.DriveServiceClient, eventRecorder *events.Recorder, log *logrus.Logger) *Controller {
	return &Controller{
		client:         client,
		crHelper:       k8s.NewCRHelper(client, log),
		nodeID:         nodeID,
		driveMgrClient: serviceClient,
		eventRecorder:  eventRecorder,
		log:            log.WithField("component", "Controller"),
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
	defer metricsC.ReconcileDuration.EvaluateDurationForType("node_drive_controller")()
	// read name
	driveName := req.Name
	// create context
	ctx, cancelFn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFn()

	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "drive/Reconcile", "name": driveName})

	// obtain corresponding drive
	drive := &drivecrd.Drive{}
	if err := c.client.ReadCR(ctx, driveName, "", drive); err != nil {
		log.Warningf("Unable to read Drive %s CR", driveName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Infof("Drive changed: %v", drive)

	status, err := c.handleDriveUpdate(ctx, log, drive)
	if err != nil {
		return ctrl.Result{RequeueAfter: base.DefaultRequeueForVolume}, err
	}
	// check status - update or delete
	switch status {
	case update:
		if err := c.client.UpdateCR(ctx, drive); err != nil {
			log.Errorf("Failed to update Drive %s CR", driveName)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	case delete:
		if err := c.client.DeleteCR(ctx, drive); err != nil {
			log.Errorf("Failed to delete Drive %s CR", driveName)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *Controller) handleDriveUpdate(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) (uint8, error) {
	// get drive fields
	status := drive.Spec.GetStatus()
	usage := drive.Spec.GetUsage()
	health := drive.Spec.GetHealth()
	id := drive.Spec.GetUUID()

    // handle offline status
	if status == apiV1.DriveStatusOffline  {
	    if usage == apiV1.DriveUsageRemoved {
	        return delete, nil
        } else {
            volumes, err := c.crHelper.GetVolumesByLocation(ctx, id)
            if err != nil {
                return ignore, err
            }

            for _, vol := range volumes {
                volume.OperationalStatus = apiV1.OperationalStatusMissing
                if err := cs.k8sClient.UpdateCR(ctxWithID, &volume); err != nil {
                    ll.Errorf("Unable to update operational status for volume ID %s: %s", volume.Spec.Id, err)
            	    isError = true
                }
            }
            return update, nil
        }
	}

	// check whether update is required
	toUpdate := false

	switch usage {
	case apiV1.DriveUsageInUse:
		if health == apiV1.HealthSuspect || health == apiV1.HealthBad {
			drive.Spec.Usage = apiV1.DriveUsageReleasing
			toUpdate = true
		}
	case apiV1.DriveUsageReleasing:
		volumes, err := c.crHelper.GetVolumesByLocation(ctx, id)
		if err != nil {
			return ignore, err
		}
		allFound := true
		for _, vol := range volumes {
			status, found := drive.Annotations[fmt.Sprintf(
				"%s/%s", apiV1.DriveAnnotationVolumeStatusPrefix, vol.Name)]
			if !found || status != apiV1.VolumeUsageReleased {
				allFound = false
				break
			}
		}
		if allFound {
			drive.Spec.Usage = apiV1.DriveUsageReleased
			eventMsg := fmt.Sprintf("Drive is ready for replacement, %s", drive.GetDriveDescription())
			c.eventRecorder.Eventf(drive, eventing.NormalType, eventing.DriveReadyForReplacement, eventMsg)
			toUpdate = true
		}

	case apiV1.DriveUsageReleased:
		status, found := drive.Annotations[apiV1.DriveAnnotationReplacement]
		if !found || status != apiV1.DriveAnnotationReplacementReady {
			break
		}
		toUpdate = true
		drive.Spec.Usage = apiV1.DriveUsageRemoving

		// check volumes annotations and update if required
		volumes, err := c.crHelper.GetVolumesByLocation(ctx, id)
		if err != nil {
			return ignore, err
		}
		for _, vol := range volumes {
			value, found := vol.Annotations[apiV1.DriveAnnotationReplacement]
			if !found || value != apiV1.DriveAnnotationReplacementReady {
				// need to update volume annotations
				vol.Annotations[apiV1.DriveAnnotationReplacement] = apiV1.DriveAnnotationReplacementReady
				if err := c.client.UpdateCR(ctx, vol); err != nil {
					log.Errorf("Failed to update volume %s annotations, error: %v", vol.Name, err)
					return ignore, err
				}
			}
		}
		fallthrough
	case apiV1.DriveUsageRemoving:
		volumes, err := c.crHelper.GetVolumesByLocation(ctx, id)
		if err != nil {
			return ignore, err
		}
		if c.checkAllVolsRemoved(volumes) {
			drive.Spec.Usage = apiV1.DriveUsageRemoved
			status, err := c.driveMgrClient.Locate(ctx, &api.DriveLocateRequest{Action: apiV1.LocateStart, DriveSerialNumber: drive.Spec.SerialNumber})
			if err != nil || status.Status != apiV1.LocateStatusOn {
				log.Errorf("Failed to locate LED of drive %s, err %v", drive.Spec.SerialNumber, err)
				drive.Spec.Usage = apiV1.DriveUsageFailed
				// send error level alert
				eventMsg := fmt.Sprintf("Failed to locale LED, %s", drive.GetDriveDescription())
				c.eventRecorder.Eventf(drive, eventing.ErrorType, eventing.DriveReplacementFailed, eventMsg)
			} else {
				// send info level alert
				eventMsg := fmt.Sprintf("Drive successfully replaced, %s", drive.GetDriveDescription())
				c.eventRecorder.Eventf(drive, eventing.NormalType, eventing.DriveSuccessfullyReplaced, eventMsg)
			}
			toUpdate = true
		}
	case apiV1.DriveUsageRemoved:
        // TODO: something
        break
	}

	if toUpdate {
		return update, nil
	}
	return ignore, nil
}

func (c *Controller) checkAllVolsRemoved(volumes []*volumecrd.Volume) bool {
	for _, vol := range volumes {
		if vol.Spec.CSIStatus != apiV1.Removed {
			return false
		}
	}
	return true
}
