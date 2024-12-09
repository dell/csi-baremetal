package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	sgcrd "github.com/dell/csi-baremetal/api/v1/storagegroupcrd"
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
	crHelper       k8s.CRHelper
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

	// annotations and keys for fake-attach
	fakeAttachVolumeAnnotation = "fake-attach"
	fakeAttachVolumeKey        = "yes"

	allDRVolumesFakeAttachedAnnotation = "all-dr-volumes-fake-attached"
	allDRVolumesFakeAttachedKey        = "yes"
)

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient, node ID and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, nodeID string, serviceClient api.DriveServiceClient, eventRecorder *events.Recorder, log *logrus.Logger) *Controller {
	return &Controller{
		client:         client,
		crHelper:       k8s.NewCRHelperImpl(client, log),
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
			log.Errorf("Failed to update Drive %s CR, error: %s", driveName, err.Error())
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
	// handle drive labeled or not
	if err := c.handleDriveLableUpdate(ctx, log, drive); err != nil {
		log.Warnf("handle drive %s label update event failure: %s", drive.Name, err.Error())
	}
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
		return c.handleDriveUsageReleasing(ctx, log, drive)

	case apiV1.DriveUsageReleased:
		if c.checkAndPlaceStatusInUse(drive) {
			toUpdate = true
			break
		}

		value, foundAllDRVolFakeAttach := drive.Annotations[allDRVolumesFakeAttachedAnnotation]
		fakeAttachDR := !drive.Spec.IsClean && foundAllDRVolFakeAttach && value == allDRVolumesFakeAttachedKey
		if drive.Spec.IsClean || fakeAttachDR {
			log.Infof("Initiating automatic removal of drive: %s", drive.GetName())
		} else {
			status, found := getDriveAnnotationRemoval(drive.Annotations)
			if !found || status != apiV1.DriveAnnotationRemovalReady {
				break
			}
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
				if err := c.updateVolumeAnnotations(ctx, vol); err != nil {
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

func (c *Controller) updateVolumeAnnotations(ctx context.Context, volume *volumecrd.Volume) error {
	if volume.Annotations == nil {
		volume.Annotations = make(map[string]string)
	}

	volume.Annotations[apiV1.DriveAnnotationRemoval] = apiV1.DriveAnnotationRemovalReady
	volume.Annotations[apiV1.DriveAnnotationReplacement] = apiV1.DriveAnnotationRemovalReady

	return c.client.UpdateCR(ctx, volume)
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

// checkAllVolsWithoutFakeAttachRemoved checks if all volumes are removed and not 'fake attached',
// returns true if any volume's CSIStatus is not 'REMOVED' or it's 'fake attached'
func (c *Controller) checkAllVolsWithoutFakeAttachRemoved(volumes []*volumecrd.Volume) bool {
	for _, v := range volumes {
		if v.Spec.CSIStatus != apiV1.Removed && !c.isFakeAttach(v) {
			return false
		}
	}
	return true
}

// checkAllLVGVolsWithoutFakeAttachRemoved checks if all volumes in the given LogicalVolumeGroup are removed or 'fake attached'.
//
// Parameters:
// - lvg: a pointer to the LogicalVolumeGroup object.
// - volumes: a slice of pointers to the Volume objects.
//
// Returns:
// - bool: true if all volumes in the given LogicalVolumeGroup are removed or 'fake attached'
func (c *Controller) checkAllLVGVolsWithoutFakeAttachRemoved(lvg *lvgcrd.LogicalVolumeGroup, volumes []*volumecrd.Volume) bool {
	for _, v := range volumes {
		if v.Spec.LocationType == apiV1.LocationTypeLVM && v.Spec.Location == lvg.Name && !c.isFakeAttach(v) {
			return false
		}
	}
	return true
}

func (c *Controller) isFakeAttach(v *volumecrd.Volume) bool {
	value, found := v.Annotations[fakeAttachVolumeAnnotation]
	return found && value == fakeAttachVolumeKey
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

func (c *Controller) handleDriveUsageReleasing(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) (uint8, error) {
	log.Debugf("releasing drive: %s", drive.Name)
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.GetUUID())
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
	if !allFound {
		return ignore, nil
	}

	drive.Spec.Usage = apiV1.DriveUsageReleased
	eventMsg := fmt.Sprintf("Drive is ready for documented removal procedure. %s", drive.GetDriveDescription())
	c.eventRecorder.Eventf(drive, eventing.DriveReadyForRemoval, eventMsg)

	return update, nil
}

func (c *Controller) handleDriveUsageRemoving(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) (uint8, error) {
	// wait all volumes without fake-attach have REMOVED status
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		return ignore, err
	}
	if !c.checkAllVolsWithoutFakeAttachRemoved(volumes) {
		log.Debugf(
			"Waiting all volumes without fake-attach in REMOVED status, current statuses: %v",
			c.getVolsStatuses(volumes),
		)
		return wait, nil
	}

	// wait lvg without fake-attach is removed if exist
	lvg, err := c.crHelper.GetLVGByDrive(ctx, drive.Spec.UUID)
	if err != nil && err != errTypes.ErrorNotFound {
		return ignore, err
	}

	if lvg != nil && !c.checkAllLVGVolsWithoutFakeAttachRemoved(lvg, volumes) {
		log.Debugf("Waiting LVG without fake-attach %s remove", lvg.Name)
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
	if err := c.triggerStorageGroupResyncIfApplicable(ctx, log, drive); err != nil {
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
		eventMsg := fmt.Sprintf("Failed to locate LED, %s", drive.GetDriveDescription())
		c.eventRecorder.Eventf(drive, eventing.DriveLocateLEDFailed, eventMsg)
	} else {
		// send warning level alert (warning for attention), good level closes issue, need only send message
		drive.Spec.LEDState = fmt.Sprintf("%d", status.Status)
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

func (c *Controller) triggerStorageGroupResyncIfApplicable(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) error {
	if drive.Labels[apiV1.StorageGroupLabelKey] != "" {
		storageGroup := &sgcrd.StorageGroup{}
		err := c.client.ReadCR(ctx, drive.Labels[apiV1.StorageGroupLabelKey], "", storageGroup)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Warnf("no existing storage group %s", drive.Labels[apiV1.StorageGroupLabelKey])
				return nil
			}
			log.Errorf("error in reading storagegroup %s: %v", drive.Labels[apiV1.StorageGroupLabelKey], err)
			return err
		}

		if storageGroup.Status.Phase != apiV1.StorageGroupPhaseInvalid &&
			storageGroup.Status.Phase != apiV1.StorageGroupPhaseRemoving &&
			storageGroup.Spec.DriveSelector.NumberDrivesPerNode > 0 {
			if storageGroup.Status.Phase == apiV1.StorageGroupPhaseSynced {
				storageGroup.Status.Phase = apiV1.StorageGroupPhaseSyncing
			}
			if storageGroup.Annotations == nil {
				storageGroup.Annotations = map[string]string{}
			}
			// also add annotation for specific drive removal to storage group
			annotationKey := fmt.Sprintf("%s/%s", apiV1.StorageGroupAnnotationDriveRemovalPrefix, drive.Name)
			storageGroup.Annotations[annotationKey] = apiV1.StorageGroupAnnotationDriveRemovalDone
			if err := c.client.UpdateCR(ctx, storageGroup); err != nil {
				log.Errorf("Unable to update StorageGroup with error: %v.", err)
				return err
			}
		}
	}
	return nil
}

// handle drive lable update
func (c *Controller) handleDriveLableUpdate(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) error {
	var taintLabelExisted bool
	labels := drive.GetLabels()
	// if drive is taint and labeled, need to update related ac label
	if effect, ok := labels[apiV1.DriveTaintKey]; ok && effect == apiV1.DriveTaintValue {
		taintLabelExisted = true
	}

	// get ac location for lvg/non-lvg case
	// lvg ac: lvg location
	// non-lvg ac:  drive name
	location := drive.GetName()
	lvg, err := c.crHelper.GetLVGByDrive(ctx, drive.Spec.UUID)
	if err != nil {
		log.Infof("LVG for drive %s is not found: %s", drive.GetName(), err.Error())
	} else if lvg != nil {
		location = lvg.GetName()
	}
	// sync label to related ac with the drive
	ac, err := c.crHelper.GetACByLocation(location)
	if err != nil && err != errTypes.ErrorNotFound {
		log.Errorf("Failed to get AC for Drive %s: %s", drive.GetName(), err.Error())
		return err
	}
	if err == errTypes.ErrorNotFound {
		log.Warnf("AC for Drive %s is not found", drive.GetName())
		return nil
	}

	acLabels := ac.GetLabels()
	if acLabels == nil {
		// label is not existed on CR, just create it and attach to CR
		acLabels = map[string]string{}
		ac.Labels = acLabels
	}
	if taintLabelExisted {
		acLabels[apiV1.DriveTaintKey] = apiV1.DriveTaintValue
	} else {
		delete(acLabels, apiV1.DriveTaintKey)
	}

	if err = c.client.UpdateCR(ctx, ac); err != nil {
		log.Errorf("Failed to update AC %s labels related to drive %s: %s", ac.GetName(), drive.GetName(), err.Error())
		return err
	}
	log.Infof("Update AC %s labels to related drive %s successful", ac.GetName(), drive.GetName())
	return nil
}
