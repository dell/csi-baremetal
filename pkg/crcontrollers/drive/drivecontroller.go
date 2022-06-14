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
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
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
	wait   uint8 = 3
)

const (
	// Annotations for driveCR to manipulate Usage
	driveActionAnnotationKey         = "action"
	driveActionAddAnnotationValue    = "add"
	driveActionRemoveAnnotationValue = "remove"
	// Deprecated annotations to to perform DR restart process
	driveRestartReplacementAnnotationKeyDeprecated   = "drive"
	driveRestartReplacementAnnotationValueDeprecated = "add"
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
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	defer metricsC.ReconcileDuration.EvaluateDurationForType("node_drive_controller")()
	// read name
	driveName := req.Name
	// create context
	ctx, cancelFn := context.WithTimeout(ctx, 60*time.Second)
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
	case wait:
		return ctrl.Result{RequeueAfter: base.DefaultTimeoutForVolumeUpdate}, nil
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
			eventMsg := fmt.Sprintf("Drive is ready for documented removal procedure. %s", drive.GetDriveDescription())
			c.eventRecorder.Eventf(drive, eventing.DriveReadyForRemoval, eventMsg)
			toUpdate = true
		}

	case apiV1.DriveUsageReleased:
		if c.checkAndPlaceStatusInUse(drive) {
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
	case apiV1.DriveUsageRemoving:
		return c.handleDriveUsageRemoving(ctx, log, drive)
	case apiV1.DriveUsageRemoved:
		return c.handleDriveUsageRemoved(ctx, log, drive)
	case apiV1.DriveUsageFailed:
		if c.checkAndPlaceStatusInUse(drive) {
			if err := c.changeVolumeUsageAfterActionAnnotation(ctx, log, drive); err != nil {
				log.Errorf("Failed to update volume on drive %s, error: %v", drive.Name, err)
				return ignore, err
			}
			toUpdate = true
			break
		}
		if c.checkAndPlaceStatusRemoved(drive) {
			toUpdate = true
			break
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

func (c *Controller) changeVolumeUsageAfterActionAnnotation(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) error {
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.GetUUID())
	if err != nil {
		return err
	}
	for _, vol := range volumes {
		value, found := vol.GetAnnotations()[apiV1.VolumeAnnotationRelease]
		if found && value == apiV1.VolumeAnnotationReleaseFailed {
			prevUsage := vol.Spec.Usage
			vol.Spec.Usage = apiV1.VolumeUsageInUse
			annotations := vol.GetAnnotations()
			delete(annotations, apiV1.VolumeAnnotationRelease)
			vol.SetAnnotations(annotations)
			if err := c.client.UpdateCR(ctx, vol); err != nil {
				log.Errorf("Unable to change volume %s usage status from %s to %s, error: %v.",
					vol.Name, prevUsage, vol.Spec.Usage, err)
				return err
			}
		}
	}
	return nil
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

func (c *Controller) getVolsStatuses(volumes []*volumecrd.Volume) map[string]string {
	statuses := map[string]string{}

	for _, vol := range volumes {
		statuses[vol.Name] = vol.Spec.CSIStatus
	}
	return statuses
}

func (c *Controller) checkAllVolsRemoved(volumes []*volumecrd.Volume) bool {
	for _, vol := range volumes {
		if vol.Spec.CSIStatus != apiV1.Removed {
			return false
		}
	}
	return true
}

// placeStatusInUse places drive.Usage to IN_USE if CR is annotated
// deletes the annotation to avoid event repeating
func (c *Controller) checkAndPlaceStatusInUse(drive *drivecrd.Drive) bool {
	if value, ok := drive.GetAnnotations()[driveActionAnnotationKey]; ok && value == driveActionAddAnnotationValue {
		drive.Spec.Usage = apiV1.DriveUsageInUse
		delete(drive.Annotations, driveActionAnnotationKey)
		return true
	}

	// check with deprecated annotation
	if value, ok := drive.GetAnnotations()[driveRestartReplacementAnnotationKeyDeprecated]; ok && value == driveRestartReplacementAnnotationValueDeprecated {
		drive.Spec.Usage = apiV1.DriveUsageInUse
		delete(drive.Annotations, driveRestartReplacementAnnotationKeyDeprecated)
		return true
	}

	return false
}

// placeStatusRemoved places drive.Usage to REMOVED if CR is annotated
// deletes the annotation to avoid event repeating
func (c *Controller) checkAndPlaceStatusRemoved(drive *drivecrd.Drive) bool {
	if value, ok := drive.GetAnnotations()[driveActionAnnotationKey]; ok && value == driveActionRemoveAnnotationValue {
		drive.Spec.Usage = apiV1.DriveUsageRemoved
		delete(drive.Annotations, driveActionAnnotationKey)

		eventMsg := fmt.Sprintf("Drive was removed via annotation, %s", drive.GetDriveDescription())
		c.eventRecorder.Eventf(drive, eventing.DriveRemovedByForce, eventMsg)

		return true
	}

	return false
}

func (c *Controller) handleDriveUsageRemoving(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) (uint8, error) {
	// wait all volumes have REMOVED status
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		return ignore, err
	}
	if !c.checkAllVolsRemoved(volumes) {
		log.Debugf("Waiting all volumes in REMOVED status, current statuses: %v", c.getVolsStatuses(volumes))
		return wait, nil
	}

	// wait lvg is removed if exist
	lvg, err := c.crHelper.GetLVGByDrive(ctx, drive.Spec.UUID)
	if err != nil && err != errTypes.ErrorNotFound {
		return ignore, err
	}
	if lvg != nil {
		log.Debugf("Waiting LVG %s remove", lvg.Name)
		return wait, nil
	}

	drive.Spec.Usage = apiV1.DriveUsageRemoved
	if drive.Spec.Status == apiV1.DriveStatusOnline {
		c.locateDriveLED(ctx, log, drive)
	} else {
		// We can not set locate for missing disks, try to locate Node instead
		log.Infof("Try to locate node LED %s", drive.Spec.NodeId)
		if _, locateErr := c.driveMgrClient.LocateNode(ctx, &api.NodeLocateRequest{Action: apiV1.LocateStart}); locateErr != nil {
			log.Errorf("Failed to start node locate: %s", locateErr.Error())
			return ignore, locateErr
		}
	}
	return update, nil
}

func (c *Controller) handleDriveUsageRemoved(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) (uint8, error) {
	if drive.Spec.Status != apiV1.DriveStatusOffline {
		return ignore, nil
	}
	// drive was removed from the system. need to clean corresponding custom resource
	// try to stop node LED
	if err := c.stopLocateNodeLED(ctx, log, drive); err != nil {
		return ignore, err
	}
	if err := c.removeRelatedAC(ctx, log, drive); err != nil {
		return ignore, err
	}
	return remove, nil
}

func (c *Controller) locateDriveLED(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) {
	// try to enable LED
	status, err := c.driveMgrClient.Locate(ctx, &api.DriveLocateRequest{Action: apiV1.LocateStart, DriveSerialNumber: drive.Spec.SerialNumber})
	if err != nil || (status.Status != apiV1.LocateStatusOn && status.Status != apiV1.LocateStatusNotAvailable) {
		log.Errorf("Failed to locate LED of drive %s, LED status - %+v, err %v", drive.Spec.SerialNumber, status, err)
		drive.Spec.Usage = apiV1.DriveUsageFailed
		// send error level alert
		eventMsg := fmt.Sprintf("Failed to locale LED, %s", drive.GetDriveDescription())
		c.eventRecorder.Eventf(drive, eventing.DriveRemovalFailed, eventMsg)
	} else {
		// send warning level alert (warning for attention), good level closes issue, need only send message
		eventMsg := fmt.Sprintf("Drive successfully removed from CSI, and ready for physical removal, %s", drive.GetDriveDescription())
		c.eventRecorder.Eventf(drive, eventing.DriveReadyForPhysicalRemoval, eventMsg)
	}
}

func (c *Controller) stopLocateNodeLED(ctx context.Context, log *logrus.Entry, curDrive *drivecrd.Drive) error {
	driveList := &drivecrd.DriveList{}
	if err := c.client.ReadList(ctx, driveList); err != nil {
		log.Errorf("Unable to read Drive List")
		return err
	}

	for _, drive := range driveList.Items {
		if drive.Name == curDrive.Name {
			continue
		}
		if drive.Spec.GetUsage() == apiV1.DriveUsageRemoved {
			log.Infof("Drive %s is still in REMOVED. Decline node locate stop request.", drive.Name)
			return nil
		}
	}

	_, err := c.driveMgrClient.LocateNode(ctx, &api.NodeLocateRequest{Action: apiV1.LocateStop})
	if err != nil {
		log.Errorf("Failed to disable node locate: %s", err.Error())
		return err
	}

	return nil
}

func (c *Controller) removeRelatedAC(ctx context.Context, log *logrus.Entry, curDrive *drivecrd.Drive) error {
	ac, err := c.crHelper.GetACByLocation(curDrive.GetName())
	if err != nil && err != errTypes.ErrorNotFound {
		log.Errorf("Failed to get AC for Drive %s: %s", curDrive.GetName(), err.Error())
		return err
	}
	if err == errTypes.ErrorNotFound {
		log.Warnf("AC for Drive %s is not found", curDrive.GetName())
		return nil
	}

	if err = c.client.DeleteCR(ctx, ac); err != nil {
		log.Errorf("Failed to delete AC %s related to drive %s: %s", ac.GetName(), curDrive.GetName(), err.Error())
		return err
	}

	return nil
}
