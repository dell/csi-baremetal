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
	remove uint8 = 2
)

const (
	// Annotations for driveCR to perform restart process
	driveRestartReplacementAnnotationKey   = "drive"
	driveRestartReplacementAnnotationValue = "add"
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
	case remove:
		if err := c.client.DeleteCR(ctx, drive); err != nil {
			log.Errorf("Failed to delete Drive %s CR", driveName)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	return ctrl.Result{}, nil
}

func (c *Controller) handleDriveUpdate(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) (uint8, error) {
	// handle offline/online drive status
	if err := c.handleDriveStatus(ctx, drive); err != nil {
		return ignore, err
	}

	// get drive fields
	usage := drive.Spec.GetUsage()
	health := drive.Spec.GetHealth()
	id := drive.Spec.GetUUID()

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
			eventMsg := fmt.Sprintf("Drive is ready for removal, %s", drive.GetDriveDescription())
			c.eventRecorder.Eventf(drive, eventing.WarningType, eventing.DriveReadyForRemoval, eventMsg)
			toUpdate = true
		}

	case apiV1.DriveUsageReleased:
		if c.restartReplacement(drive) {
			toUpdate = true
			break
		}

		status, found := getDriveAnnotationRemoval(drive.Annotations)
		if !found || status != apiV1.DriveAnnotationRemovalReady {
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
			value, found := getDriveAnnotationRemoval(vol.Annotations)
			if !found || value != apiV1.DriveAnnotationRemovalReady {
				// need to update volume annotations
				vol.Annotations[apiV1.DriveAnnotationRemoval] = apiV1.DriveAnnotationRemovalReady
				vol.Annotations[apiV1.DriveAnnotationReplacement] = apiV1.DriveAnnotationRemovalReady
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
				c.eventRecorder.Eventf(drive, eventing.ErrorType, eventing.DriveRemovalFailed, eventMsg)
			} else {
				// send warning level alert (warning for attention), good level closes issue, need only send message
				eventMsg := fmt.Sprintf("Drive successfully removed from CSI, and ready for physical removal, %s", drive.GetDriveDescription())
				c.eventRecorder.Eventf(drive, eventing.WarningType, eventing.DriveReadyForPhysicalRemoval, eventMsg)
			}
			toUpdate = true
		}
	case apiV1.DriveUsageRemoved:
		if drive.Spec.Status == apiV1.DriveStatusOffline {
			// drive was removed from the system. need to clean corresponding custom resource
			return remove, nil
		}
	case apiV1.DriveUsageFailed:
		if c.restartReplacement(drive) {
			toUpdate = true
		}
	}

	if toUpdate {
		return update, nil
	}
	return ignore, nil
}

// For support deprecated Replacement annotation
func getDriveAnnotationRemoval(annotations map[string]string) (string, bool) {
	status, found := annotations[apiV1.DriveAnnotationRemoval]
	if !found {
		status, found = annotations[apiV1.DriveAnnotationReplacement]
	}
	return status, found
}

func (c *Controller) handleDriveStatus(ctx context.Context, drive *drivecrd.Drive) error {
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		return err
	}

	switch drive.Spec.Status {
	case apiV1.DriveStatusOffline:
		for _, volume := range volumes {
			if err := c.crHelper.UpdateVolumeOpStatus(ctx, volume, apiV1.OperationalStatusMissing); err != nil {
				return err
			}
		}
	case apiV1.DriveStatusOnline:
		// move MISSING volumes to OPERATIVE status
		for _, volume := range volumes {
			if volume.Spec.OperationalStatus == apiV1.OperationalStatusMissing {
				if err := c.crHelper.UpdateVolumeOpStatus(ctx, volume, apiV1.OperationalStatusOperative); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *Controller) checkAllVolsRemoved(volumes []*volumecrd.Volume) bool {
	for _, vol := range volumes {
		if vol.Spec.CSIStatus != apiV1.Removed {
			return false
		}
	}
	return true
}

// restartReplacement restores drive.Usage to IN_USE if CR is annotated
// deletes the annotation to avoid event repeating
func (c *Controller) restartReplacement(drive *drivecrd.Drive) bool {
	if value, ok := drive.GetAnnotations()[driveRestartReplacementAnnotationKey]; ok && value == driveRestartReplacementAnnotationValue {
		drive.Spec.Usage = apiV1.DriveUsageInUse
		delete(drive.Annotations, driveRestartReplacementAnnotationKey)
		return true
	}

	return false
}
