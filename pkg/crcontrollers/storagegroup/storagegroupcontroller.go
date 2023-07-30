package storagegroup

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	sgcrd "github.com/dell/csi-baremetal/api/v1/storagegroupcrd"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

const (
	sgFinalizer           = "dell.emc.csi/sg-cleanup"
	contextTimeoutSeconds = 60
	normalRequeueInterval = 1 * time.Second
)

var unsupportedDriveMatchFields = []string{"Health", "Status", "Usage", "IsClean"}

// Controller to reconcile storagegroup custom resource
type Controller struct {
	client         *k8s.KubeClient
	k8sCache       k8s.CRReader
	log            *logrus.Entry
	crHelper       k8s.CRHelper
	cachedCrHelper k8s.CRHelper
}

// NewController creates new instance of Controller structure
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of Controller
func NewController(client *k8s.KubeClient, k8sCache k8s.CRReader, log *logrus.Logger) *Controller {
	c := &Controller{
		client:         client,
		k8sCache:       k8sCache,
		crHelper:       k8s.NewCRHelperImpl(client, log),
		cachedCrHelper: k8s.NewCRHelperImpl(client, log).SetReader(k8sCache),
		log:            log.WithField("component", "StorageGroupController"),
	}
	return c
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sgcrd.StorageGroup{}).
		WithOptions(controller.Options{}).
		Watches(&source.Kind{Type: &drivecrd.Drive{}}, &handler.EnqueueRequestForObject{}).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				return c.filterDeleteEvent(e.Object)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return c.filterUpdateEvent(e.ObjectOld, e.ObjectNew)
			},
		}).
		Complete(c)
}

func (c *Controller) filterDeleteEvent(obj runtime.Object) bool {
	_, isStorageGroup := obj.(*sgcrd.StorageGroup)
	return isStorageGroup
}

func (c *Controller) filterUpdateEvent(old runtime.Object, new runtime.Object) bool {
	if newDrive, ok := new.(*drivecrd.Drive); ok {
		if oldDrive, ok := old.(*drivecrd.Drive); ok {
			return filterDriveUpdateEvent(oldDrive, newDrive)
		}
	}
	return true
}

func filterDriveUpdateEvent(old *drivecrd.Drive, new *drivecrd.Drive) bool {
	oldLabel := old.Labels[apiV1.StorageGroupLabelKey]
	newLabel, newLabeled := new.Labels[apiV1.StorageGroupLabelKey]
	return (oldLabel != newLabel) || (!old.Spec.IsClean && new.Spec.IsClean && !newLabeled)
}

// Reconcile reconciles StorageGroup custom resources
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(ctx, contextTimeoutSeconds*time.Second)
	defer cancelFn()

	// read name
	reqName := req.Name
	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "Reconcile", "name": reqName})

	if _, err := uuid.Parse(reqName); err == nil {
		drive := &drivecrd.Drive{}
		// A drive just physically removed but not yet totally deleted yet, i.e. Usage == "REMOVED" && Status == "OFFLINE"
		// will not be selected in any storage group and its existing sg label takes no effect
		if err := c.client.ReadCR(ctx, reqName, "", drive); err == nil &&
			!(drive.Spec.Usage == apiV1.DriveUsageRemoved && drive.Spec.Status == apiV1.DriveStatusOffline) {
			return c.reconcileDriveStorageGroupLabel(ctx, drive)
		} else if err != nil && !k8serrors.IsNotFound(err) {
			log.Errorf("error in reading %s as drive object: %v", reqName, err)
			return ctrl.Result{}, err
		}
	}

	storageGroup := &sgcrd.StorageGroup{}
	if err := c.client.ReadCR(ctx, reqName, "", storageGroup); err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Errorf("error in reading %s as drive or storagegroup object: %v", reqName, err)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return c.reconcileStorageGroup(ctx, storageGroup)
}

// combine the following similar funcs
func (c *Controller) removeACStorageGroupLabel(ctx context.Context, log *logrus.Entry, ac *accrd.AvailableCapacity) error {
	delete(ac.Labels, apiV1.StorageGroupLabelKey)
	if err := c.client.UpdateCR(ctx, ac); err != nil {
		log.Errorf("failed to remove storage-group label from ac %s with error %v", ac.Name, err)
		return err
	}
	return nil
}

func (c *Controller) removeDriveStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) error {
	delete(drive.Labels, apiV1.StorageGroupLabelKey)
	if err := c.client.UpdateCR(ctx, drive); err != nil {
		log.Errorf("failed to remove storage-group label from drive %s with error %v", drive.Name, err)
		return err
	}
	return nil
}

func (c *Controller) updateDriveStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sgName string) error {
	if drive.Labels == nil {
		drive.Labels = map[string]string{}
	}
	drive.Labels[apiV1.StorageGroupLabelKey] = sgName
	if err := c.client.UpdateCR(ctx, drive); err != nil {
		log.Errorf("failed to add storage group %s label to drive %s with error %v", sgName, drive.Name, err)
		return err
	}
	return nil
}

func (c *Controller) updateACStorageGroupLabel(ctx context.Context, log *logrus.Entry, ac *accrd.AvailableCapacity,
	sgName string) error {
	if ac.Labels == nil {
		ac.Labels = map[string]string{}
	}
	ac.Labels[apiV1.StorageGroupLabelKey] = sgName
	if err := c.client.UpdateCR(ctx, ac); err != nil {
		log.Errorf("failed to add storage group %s label to ac %s with error %v", sgName, ac.Name, err)
		return err
	}
	return nil
}

func (c *Controller) findAndAddMatchedStorageGroupLabel(ctx context.Context, drive *drivecrd.Drive,
	ac *accrd.AvailableCapacity) (ctrl.Result, error) {
	log := c.log.WithFields(logrus.Fields{"method": "findAndAddMatchedStorageGroupLabel", "name": drive.Name})

	sgList := &sgcrd.StorageGroupList{}
	if err := c.k8sCache.ReadList(ctx, sgList); err != nil {
		log.Errorf("failed to read storage group list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	for _, storageGroup := range sgList.Items {
		sg := storageGroup

		if (sg.Status.Phase == apiV1.StorageGroupPhaseSynced || (sg.Status.Phase == apiV1.StorageGroupPhaseSyncing &&
			sg.Spec.DriveSelector.NumberDrivesPerNode == 0)) && c.isDriveSelectedByValidMatchFields(log, &drive.Spec,
			&sg.Spec.DriveSelector.MatchFields) {
			if sg.Spec.DriveSelector.NumberDrivesPerNode == 0 {
				log.Infof("Expect to add label of storagegroup %s to drive %s", sg.Name, drive.Name)
				if err := c.updateDriveStorageGroupLabel(ctx, log, drive, sg.Name); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				log.Infof("Successfully add label of storagegroup %s to drive %s", sg.Name, drive.Name)
				if err := c.updateACStorageGroupLabel(ctx, log, ac, sg.Name); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
				log.Infof("Successfully add label of storagegroup %s to drive %s's corresponding AC", sg.Name, drive.Name)
				return ctrl.Result{}, nil
			}

			log.Debugf("drive %s will probably be selected by storagegroup %s", drive.Name, sg.Name)
			if sg.Status.Phase == apiV1.StorageGroupPhaseSynced {
				// trigger the subsequent reconciliation of the potentially-matched storage group
				sg.Status.Phase = apiV1.StorageGroupPhaseSyncing
				if err := c.client.UpdateCR(ctx, &sg); err != nil {
					log.Errorf("Unable to update StorageGroup status with error: %v", err)
					return ctrl.Result{Requeue: true}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (c *Controller) handleSeparateDriveStorageGroupLabelAddition(ctx context.Context, log *logrus.Entry,
	drive *drivecrd.Drive, ac *accrd.AvailableCapacity, driveSGLabel string, lvgExists bool) (ctrl.Result, error) {
	if !drive.Spec.IsClean || lvgExists || ac.Spec.Size == 0 {
		log.Warnf("We shouldn't storage group label to not clean drive %s", drive.Name)

		// if this drive has been really selected by this existing sg with NumberDrivesPerNode > 0 and SYNCED status,
		// we need to also trigger the resync of this sg afterward
		sg := &sgcrd.StorageGroup{}
		if err := c.k8sCache.ReadCR(ctx, driveSGLabel, "", sg); err != nil && !k8serrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, err
		}
		if sg.Spec.DriveSelector.NumberDrivesPerNode > 0 && sg.Status.Phase == apiV1.StorageGroupPhaseSynced &&
			c.isDriveSelectedByValidMatchFields(log, &drive.Spec, &sg.Spec.DriveSelector.MatchFields) {
			sg.Status.Phase = apiV1.StorageGroupPhaseSyncing
			if err := c.client.UpdateCR(ctx, sg); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}

		if err := c.removeDriveStorageGroupLabel(ctx, log, drive); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}

	if err := c.updateACStorageGroupLabel(ctx, log, ac, driveSGLabel); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	log.Infof("successfully add label of storage group %s to drive %s and its corresponding AC manually", driveSGLabel, drive.Name)
	return ctrl.Result{}, nil
}

func (c *Controller) handleManualDriveStorageGroupLabelRemoval(ctx context.Context, log *logrus.Entry,
	drive *drivecrd.Drive, ac *accrd.AvailableCapacity, acSGLabel string) (ctrl.Result, error) {
	volumes, err := c.cachedCrHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		log.Errorf("Error when getting volumes on drive %s: %v", drive.Name, err)
		return ctrl.Result{Requeue: true}, err
	}
	if len(volumes) > 0 {
		log.Warnf("We shouldn't remove label of storage group %s from drive %s with existing volumes",
			acSGLabel, drive.Name)
		if err := c.updateDriveStorageGroupLabel(ctx, log, drive, acSGLabel); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}

	sg := &sgcrd.StorageGroup{}
	err = c.k8sCache.ReadCR(ctx, acSGLabel, "", sg)
	switch {
	case err == nil && c.isDriveSelectedByValidMatchFields(log, &drive.Spec, &sg.Spec.DriveSelector.MatchFields):
		log.Warnf("We shouldn't remove label of storage group %s from drive %s still selected by this storage group",
			acSGLabel, drive.Name)
		if err := c.updateDriveStorageGroupLabel(ctx, log, drive, acSGLabel); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	case err != nil && !k8serrors.IsNotFound(err):
		log.Errorf("Failed to read StorageGroup %s with error: %v", acSGLabel, err)
		return ctrl.Result{Requeue: true}, err

	// the case that the drive's storage group label removal is valid and we should sync the removal to AC
	default:
		if err := c.removeACStorageGroupLabel(ctx, log, ac); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Infof("successfully remove label of storage group %s from drive %s and its corresponding AC",
			acSGLabel, drive.Name)
	}
	return ctrl.Result{}, nil
}

// Here, we will sync the storage-group label of single drive object if applicable
func (c *Controller) reconcileDriveStorageGroupLabel(ctx context.Context, drive *drivecrd.Drive) (ctrl.Result, error) {
	log := c.log.WithFields(logrus.Fields{"method": "reconcileDriveStorageGroupLabel", "name": drive.Name})

	location := drive.Name
	lvg, err := c.cachedCrHelper.GetLVGByDrive(ctx, drive.Spec.UUID)
	if err != nil {
		log.Errorf("Error when getting LVG by drive %s: %v", drive.Name, err)
		return ctrl.Result{Requeue: true}, err
	}
	if lvg != nil {
		location = lvg.Name
	}

	ac, err := c.cachedCrHelper.GetACByLocation(location)
	if err != nil {
		log.Errorf("Error when getting AC of drive %s: %v", drive.Name, err)
		if err != errTypes.ErrorNotFound {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{RequeueAfter: normalRequeueInterval}, nil
	}

	acSGLabel, acSGLabeled := ac.Labels[apiV1.StorageGroupLabelKey]
	driveSGLabel, driveSGLabeled := drive.Labels[apiV1.StorageGroupLabelKey]
	if acSGLabel == driveSGLabel {
		if !acSGLabeled && !driveSGLabeled && drive.Spec.IsClean && lvg == nil && ac.Spec.Size > 0 {
			return c.findAndAddMatchedStorageGroupLabel(ctx, drive, ac)
		}
		return ctrl.Result{}, nil
	}

	// Current manual sg labeling support
	log.Debugf("Handle separate change of storage group label of drive %s", drive.Name)

	switch {
	// add new storagegroup label to drive separately
	case !acSGLabeled && driveSGLabeled:
		return c.handleSeparateDriveStorageGroupLabelAddition(ctx, log, drive, ac, driveSGLabel, lvg != nil)

	// remove storagegroup label from drive manually
	case acSGLabeled && !driveSGLabeled:
		return c.handleManualDriveStorageGroupLabelRemoval(ctx, log, drive, ac, acSGLabel)

	// actually just the case acSGLabeled && driveSGLabeled here, i.e. update drive's storage group label manually,
	// need to restore the update
	default:
		log.Warnf("We cannot update the drive %s's storage group label from %s to %s", drive.Name, acSGLabel, driveSGLabel)
		if err := c.updateDriveStorageGroupLabel(ctx, log, drive, acSGLabel); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}
}

func (c *Controller) reconcileStorageGroup(ctx context.Context, storageGroup *sgcrd.StorageGroup) (ctrl.Result, error) {
	log := c.log.WithFields(logrus.Fields{"method": "reconcileStorageGroup", "name": storageGroup.Name})

	log.Debugf("Reconcile StorageGroup: %+v", storageGroup)

	// StorageGroup Deletion request
	if !storageGroup.DeletionTimestamp.IsZero() {
		if storageGroup.Status.Phase != apiV1.StorageGroupPhaseRemoving {
			storageGroup.Status.Phase = apiV1.StorageGroupPhaseRemoving
			if err := c.client.UpdateCR(ctx, storageGroup); err != nil {
				log.Errorf("Unable to update StorageGroup status with error: %v.", err)
				return ctrl.Result{Requeue: true}, err
			}
		}
		return c.handleStorageGroupDeletion(ctx, log, storageGroup)
	}

	if !util.ContainsString(storageGroup.Finalizers, sgFinalizer) {
		// append finalizer
		log.Debugf("Appending finalizer for StorageGroup")
		storageGroup.Finalizers = append(storageGroup.Finalizers, sgFinalizer)
		if err := c.client.UpdateCR(ctx, storageGroup); err != nil {
			log.Errorf("Unable to append finalizer %s to StorageGroup with error: %v.", sgFinalizer, err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	if storageGroup.Status.Phase == "" {
		if !c.isStorageGroupValid(log, storageGroup) {
			storageGroup.Status.Phase = apiV1.StorageGroupPhaseInvalid
			if err := c.client.UpdateCR(ctx, storageGroup); err != nil {
				log.Errorf("Unable to update StorageGroup status with error: %v.", err)
				return ctrl.Result{Requeue: true}, err
			}
			return ctrl.Result{}, nil
		}
		// Pass storage group valiation, change to SYNCING status phase
		storageGroup.Status.Phase = apiV1.StorageGroupPhaseSyncing
		if err := c.client.UpdateCR(ctx, storageGroup); err != nil {
			log.Errorf("Unable to update StorageGroup status with error: %v.", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	if storageGroup.Status.Phase == apiV1.StorageGroupPhaseSyncing {
		return c.handleStorageGroupCreationOrUpdate(ctx, log, storageGroup)
	}

	return ctrl.Result{}, nil
}

func (c *Controller) handleStorageGroupDeletion(ctx context.Context, log *logrus.Entry,
	sg *sgcrd.StorageGroup) (ctrl.Result, error) {
	log.Infof("handle deletion of storage group %s", sg.Name)

	drivesList := &drivecrd.DriveList{}
	if err := c.k8sCache.ReadList(ctx, drivesList); err != nil {
		log.Errorf("failed to read drives list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	var labelRemovalErrMsgs []string

	// whether there is some drive with existing volumes in this storage group
	driveHasExistingVolumes := false
	for _, drive := range drivesList.Items {
		drive := drive
		if drive.Labels[apiV1.StorageGroupLabelKey] == sg.Name {
			successful, err := c.removeDriveAndACStorageGroupLabel(ctx, log, &drive, sg.Name)
			if err != nil {
				labelRemovalErrMsgs = append(labelRemovalErrMsgs, err.Error())
			} else if !successful {
				driveHasExistingVolumes = true
			}
		}
	}
	if len(labelRemovalErrMsgs) > 0 {
		return ctrl.Result{Requeue: true}, fmt.Errorf(strings.Join(labelRemovalErrMsgs, "\n"))
	}
	if driveHasExistingVolumes {
		log.Warnf("Storage group %s has drive with existing volumes. The deletion will be retried later.", sg.Name)
		return ctrl.Result{RequeueAfter: normalRequeueInterval}, nil
	}
	log.Infof("deletion of storage group %s successfully completed", sg.Name)
	return c.removeFinalizer(ctx, log, sg)
}

func (c *Controller) removeFinalizer(ctx context.Context, log *logrus.Entry,
	sg *sgcrd.StorageGroup) (ctrl.Result, error) {
	if util.ContainsString(sg.Finalizers, sgFinalizer) {
		sg.Finalizers = util.RemoveString(sg.Finalizers, sgFinalizer)
		if err := c.client.UpdateCR(ctx, sg); err != nil {
			log.Errorf("Unable to remove finalizer %s from StorageGroup with error: %v", sgFinalizer, err)
			return ctrl.Result{Requeue: true}, err
		}
	}
	return ctrl.Result{}, nil
}

func (c *Controller) handleStorageGroupCreationOrUpdate(ctx context.Context, log *logrus.Entry,
	sg *sgcrd.StorageGroup) (ctrl.Result, error) {
	log.Infof("handle creation or update of storage group %s", sg.Name)

	drivesList := &drivecrd.DriveList{}
	if err := c.k8sCache.ReadList(ctx, drivesList); err != nil {
		log.Errorf("failed to read drives list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}
	noDriveSelected := true
	drivesCount := map[string]int32{}
	driveSelector := sg.Spec.DriveSelector

	var labelingErrMsgs []string

	// used for candidate drives to be selected by storagegroup with numberDrivesPerNode > 0
	var candidateDrives []*drivecrd.Drive

	for _, d := range drivesList.Items {
		drive := d

		// A drive just physically removed but not yet totally deleted yet, i.e. Usage == "REMOVED" && Status == "OFFLINE"
		// will not be selected in any storage group and its existing sg label takes no effect
		if drive.Spec.Usage == apiV1.DriveUsageRemoved && drive.Spec.Status == apiV1.DriveStatusOffline {
			continue
		}

		existingStorageGroup, exists := drive.Labels[apiV1.StorageGroupLabelKey]
		if exists {
			if existingStorageGroup == sg.Name {
				noDriveSelected = false
				if driveSelector.NumberDrivesPerNode > 0 {
					drivesCount[drive.Spec.NodeId]++
				}
			}
			log.Debugf("Drive %s has already been selected by storage group %s", drive.Name, existingStorageGroup)
			continue
		}

		if c.isDriveSelectedByValidMatchFields(log, &drive.Spec, &driveSelector.MatchFields) &&
			(driveSelector.NumberDrivesPerNode == 0 || drivesCount[drive.Spec.NodeId] < driveSelector.NumberDrivesPerNode) {
			if driveSelector.NumberDrivesPerNode > 0 {
				candidateDrives = append(candidateDrives, &drive)
				continue
			}

			driveLabeled, err := c.addDriveAndACStorageGroupLabel(ctx, log, &drive, sg.Name)
			if driveLabeled {
				noDriveSelected = false
			} else if err != nil {
				labelingErrMsgs = append(labelingErrMsgs, err.Error())
			}
		}
	}

	for _, d := range candidateDrives {
		drive := d
		if drivesCount[drive.Spec.NodeId] < driveSelector.NumberDrivesPerNode {
			driveLabeled, err := c.addDriveAndACStorageGroupLabel(ctx, log, drive, sg.Name)
			if driveLabeled {
				noDriveSelected = false
				drivesCount[drive.Spec.NodeId]++
			} else if err != nil {
				labelingErrMsgs = append(labelingErrMsgs, err.Error())
			}
		}
	}

	if noDriveSelected {
		log.Warnf("No drive can be selected by current storage group %s", sg.Name)
	}

	if len(labelingErrMsgs) != 0 {
		return ctrl.Result{Requeue: true}, fmt.Errorf(strings.Join(labelingErrMsgs, "\n"))
	}
	sg.Status.Phase = apiV1.StorageGroupPhaseSynced
	if err := c.client.UpdateCR(ctx, sg); err != nil {
		log.Errorf("Unable to update StorageGroup status with error: %v.", err)
		return ctrl.Result{Requeue: true}, err
	}
	log.Infof("creation or update of storage group %s completed", sg.Name)
	return ctrl.Result{}, nil
}

func (c *Controller) isDriveSelectedByValidMatchFields(log *logrus.Entry, drive *api.Drive, matchFields *map[string]string) bool {
	for fieldName, fieldValue := range *matchFields {
		isSupportedDriveMatchField := true
		for _, unsupportedField := range unsupportedDriveMatchFields {
			if fieldName == unsupportedField {
				isSupportedDriveMatchField = false
				break
			}
		}
		if !isSupportedDriveMatchField {
			continue
		}

		driveField := reflect.ValueOf(drive).Elem().FieldByName(fieldName)
		switch driveField.Type().String() {
		case "string":
			if driveField.String() != fieldValue {
				return false
			}
		case "int64":
			fieldValueInt64, _ := strconv.ParseInt(fieldValue, 10, 64)
			if driveField.Int() != fieldValueInt64 {
				return false
			}
		case "bool":
			fieldValueBool, _ := strconv.ParseBool(fieldValue)
			if driveField.Bool() != fieldValueBool {
				return false
			}
		default:
			// the case of unexpected field type of the field which may be added to drive CR in the future
			log.Warnf("unexpected field type %s for field %s with value %s in matchFields",
				driveField.Type().String(), fieldName, fieldValue)
			return false
		}
	}
	return true
}

func (c *Controller) isMatchFieldsValid(log *logrus.Entry, matchFields *map[string]string) bool {
	for fieldName, fieldValue := range *matchFields {
		driveField := reflect.ValueOf(&api.Drive{}).Elem().FieldByName(fieldName)
		if !driveField.IsValid() {
			log.Warnf("Invalid field %s in driveSelector.matchFields!", fieldName)
			return false
		}
		switch driveField.Type().String() {
		case "string":
		case "int64":
			if _, err := strconv.ParseInt(fieldValue, 10, 64); err != nil {
				log.Warnf("Invalid field value %s for field %s. Parsing error: %v", fieldValue, fieldName, err)
				return false
			}
		case "bool":
			if _, err := strconv.ParseBool(fieldValue); err != nil {
				log.Warnf("Invalid field value %s for field %s. Parsing error: %v", fieldValue, fieldName, err)
				return false
			}
		default:
			// the case of unexpected field type of the field which may be added to drive CR in the future
			log.Warnf("unexpected field type %s for field %s with value %s in matchFields",
				driveField.Type().String(), fieldName, fieldValue)
			return false
		}
	}
	return true
}

// TODO Need more check on whether storagegroup is valid
func (c *Controller) isStorageGroupValid(log *logrus.Entry, sg *sgcrd.StorageGroup) bool {
	return c.isMatchFieldsValid(log, &sg.Spec.DriveSelector.MatchFields)
}

func (c *Controller) removeDriveAndACStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sgName string) (bool, error) {
	log.Infof("try to remove storagegroup label of drive %s and its corresponding AC", drive.Name)

	// check whether this drive has existing volumes
	volumes, err := c.cachedCrHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		log.Errorf("failed to get volumes on drive %s: %v", drive.Name, err)
		return false, err
	}
	if len(volumes) > 0 {
		log.Warnf("Drive %s has existing volumes and its sg label can't be removed.", drive.Name)
		return false, nil
	}

	location := drive.Name
	lvg, err := c.cachedCrHelper.GetLVGByDrive(ctx, drive.Name)
	if err != nil {
		log.Errorf("error when getting LVG of drive %s: %v", drive.GetName(), err)
		return false, err
	} else if lvg != nil {
		location = lvg.Name
	}
	ac, err := c.cachedCrHelper.GetACByLocation(location)
	if err != nil && err != errTypes.ErrorNotFound {
		log.Errorf("error when getting AC of drive %s: %v", drive.Name, err)
		return false, err
	}
	if ac != nil {
		if ac.Labels[apiV1.StorageGroupLabelKey] != sgName {
			log.Warnf("ac %s's storage group label is not %s", ac.Name, sgName)
		}
		if err = c.removeACStorageGroupLabel(ctx, log, ac); err != nil {
			return false, err
		}
		log.Infof("successfully remove storagegroup label of drive %s's corresponding AC", drive.Name)
	}

	if err = c.removeDriveStorageGroupLabel(ctx, log, drive); err != nil {
		return false, err
	}
	log.Infof("successfully remove storagegroup label of drive %s", drive.Name)
	return true, nil
}

func (c *Controller) addDriveAndACStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sgName string) (bool, error) {
	log.Infof("Expect to add label of storagegroup %s to drive %s and its corresponding AC", sgName, drive.Name)

	if !drive.Spec.IsClean {
		log.Warnf("not clean drive %s can't be selected by current storage group.", drive.Name)
		return false, nil
	}

	ac, err := c.cachedCrHelper.GetACByLocation(drive.Name)
	if err != nil {
		if err != errTypes.ErrorNotFound {
			log.Errorf("Error when getting AC by the location of drive %s: %v", drive.Spec.UUID, err)
			return false, err
		}
		lvg, e := c.cachedCrHelper.GetLVGByDrive(ctx, drive.Name)
		if e != nil || lvg != nil {
			return false, e
		}
		// AC really doesn't exist yet, need to wait for AC created and reconcile again
		return false, err
	}
	// not clean AC, i.e. not clean drive
	if ac != nil && ac.Spec.Size == 0 {
		if ac.Spec.Size == 0 {
			log.Warnf("not clean drive %s can't be selected by current storage group.", drive.Name)
		}
		return false, nil
	}

	// the corresponding non-lvg ac exists and has free space, add storage-group label to the drive and corresponding ac
	if err = c.updateDriveStorageGroupLabel(ctx, log, drive, sgName); err != nil {
		return false, err
	}
	log.Infof("Successfully add label of storagegroup %s to drive %s", sgName, drive.Name)

	if err = c.updateACStorageGroupLabel(ctx, log, ac, sgName); err != nil {
		return true, err
	}
	log.Infof("Successfully add label of storagegroup %s to drive %s and its corresponding AC", sgName, drive.Name)

	return true, nil
}
