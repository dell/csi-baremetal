package storagegroup

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

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
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

const (
	sgFinalizer             = "dell.emc.csi/sg-cleanup"
	contextTimeoutSeconds   = 60
	sgDeletionRetryInterval = 1 * time.Second

	// drive annotation record of last applied storage group label
	driveAnnotationKeyLastAppliedStorageGroupLabel = "controller.csi-baremetal.dell.com/last-applied-storage-group-label"
)

// Controller to reconcile storagegroup custom resource
type Controller struct {
	client         *k8s.KubeClient
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
			UpdateFunc: func(e event.UpdateEvent) bool {
				return c.filterUpdateEvent(e.ObjectOld, e.ObjectNew)
			},
		}).
		Complete(c)
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
	return old.Labels[apiV1.StorageGroupLabelKey] != new.Labels[apiV1.StorageGroupLabelKey] ||
		(!old.Spec.IsClean && new.Spec.IsClean && new.Labels[apiV1.StorageGroupLabelKey] == "")
}

// Reconcile reconciles StorageGroup custom resources
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(ctx, contextTimeoutSeconds*time.Second)
	defer cancelFn()

	// read name
	name := req.Name
	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "Reconcile", "name": name})

	drive := &drivecrd.Drive{}
	// A drive just physically removed but not yet totally deleted yet, i.e. Usage == "REMOVED" && Status == "OFFLINE"
	// will not be selected in any storage group and its existing sg label takes no effect
	if err := c.client.ReadCR(ctx, name, "", drive); err == nil &&
		!(drive.Spec.Usage == apiV1.DriveUsageRemoved && drive.Spec.Status == apiV1.DriveStatusOffline) {
		return c.reconcileDriveStorageGroupLabel(ctx, drive)
	} else if err != nil && !k8serrors.IsNotFound(err) {
		log.Errorf("error in reading %s as drive object: %v", name, err)
	}

	storageGroup := &sgcrd.StorageGroup{}
	if err := c.client.ReadCR(ctx, name, "", storageGroup); err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Errorf("error in reading %s as drive or storagegroup object: %v", name, err)
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
	delete(drive.Annotations, driveAnnotationKeyLastAppliedStorageGroupLabel)
	if err := c.client.UpdateCR(ctx, drive); err != nil {
		log.Errorf("failed to remove storage-group label from drive %s with error %v", drive.Name, err)
		return err
	}
	return nil
}

func (c *Controller) addDriveStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sgName string) error {
	if drive.Labels == nil {
		drive.Labels = map[string]string{}
	}
	drive.Labels[apiV1.StorageGroupLabelKey] = sgName
	if drive.Annotations == nil {
		drive.Annotations = map[string]string{}
	}
	drive.Annotations[driveAnnotationKeyLastAppliedStorageGroupLabel] = sgName
	if err := c.client.UpdateCR(ctx, drive); err != nil {
		log.Errorf("failed to add storage group %s label to drive %s with error %v", sgName, drive.Name, err)
		return err
	}
	return nil
}

func (c *Controller) addACStorageGroupLabel(ctx context.Context, log *logrus.Entry, ac *accrd.AvailableCapacity,
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

func (c *Controller) findAndAddMatchedStorageGroupLabelToDrive(ctx context.Context, drive *drivecrd.Drive) (ctrl.Result, error) {
	log := c.log.WithFields(logrus.Fields{"method": "findAndAddMatchedStorageGroupLabelToDrive", "name": drive.Name})

	sgList := &sgcrd.StorageGroupList{}
	if err := c.client.ReadList(ctx, sgList); err != nil {
		log.Errorf("failed to read storage group list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	for _, storageGroup := range sgList.Items {
		sg := storageGroup

		if sg.Status.Phase == "" && sg.DeletionTimestamp.IsZero() {
			if !c.isStorageGroupValid(log, &sg) {
				sg.Status.Phase = apiV1.StorageGroupPhaseInvalid
				if err := c.client.UpdateCR(ctx, &sg); err != nil {
					log.Errorf("Unable to update StorageGroup status with error: %v", err)
					return ctrl.Result{Requeue: true}, err
				}
				continue
			}
			// Pass storage group valiation, change to SYNCING status phase
			sg.Status.Phase = apiV1.StorageGroupPhaseSyncing
			if err := c.client.UpdateCR(ctx, &sg); err != nil {
				log.Errorf("Unable to update StorageGroup status with error: %v", err)
				return ctrl.Result{Requeue: true}, err
			}
		}

		if (sg.Status.Phase == apiV1.StorageGroupPhaseSynced || (sg.Status.Phase == apiV1.StorageGroupPhaseSyncing &&
			sg.Spec.DriveSelector.NumberDrivesPerNode == 0)) && c.isDriveSelectedByValidMatchFields(log, &drive.Spec,
			&sg.Spec.DriveSelector.MatchFields) {
			if sg.Spec.DriveSelector.NumberDrivesPerNode == 0 {
				if _, err := c.addDriveAndACStorageGroupLabel(ctx, log, drive, sg.Name); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
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

// Here, we will sync the storage-group label of single drive object if applicable
func (c *Controller) reconcileDriveStorageGroupLabel(ctx context.Context, drive *drivecrd.Drive) (ctrl.Result, error) {
	log := c.log.WithFields(logrus.Fields{"method": "reconcileDriveStorageGroupLabel", "name": drive.Name})

	driveSGLabel, driveSGLabeled := drive.Labels[apiV1.StorageGroupLabelKey]
	driveLastSGLabelRecord, driveLastSGLabelRecorded := drive.Annotations[driveAnnotationKeyLastAppliedStorageGroupLabel]
	if driveSGLabel == driveLastSGLabelRecord {
		if !driveSGLabeled && !driveLastSGLabelRecorded {
			return c.findAndAddMatchedStorageGroupLabelToDrive(ctx, drive)
		}

		driveReallyClean, err := c.addDriveAndACStorageGroupLabel(ctx, log, drive, driveSGLabel)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		if !driveReallyClean {
			// remove the selection of this drive from its previous storage group
			if err = c.removeDriveStorageGroupLabel(ctx, log, drive); err != nil {
				return ctrl.Result{Requeue: true}, err
			}

			sg := &sgcrd.StorageGroup{}
			if err = c.client.ReadCR(ctx, driveSGLabel, "", sg); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			if sg.Spec.DriveSelector.NumberDrivesPerNode > 0 && sg.Status.Phase == apiV1.StorageGroupPhaseSynced {
				sg.Status.Phase = apiV1.StorageGroupPhaseSyncing
				if err = c.client.UpdateCR(ctx, sg); err != nil {
					return ctrl.Result{Requeue: true}, err
				}
			}
		}
		return ctrl.Result{}, nil
	}

	log.Debugf("Restore manual change of drive %s's storage group label", drive.Name)
	if driveLastSGLabelRecorded {
		drive.Labels[apiV1.StorageGroupLabelKey] = drive.Annotations[driveAnnotationKeyLastAppliedStorageGroupLabel]
	} else {
		delete(drive.Labels, apiV1.StorageGroupLabelKey)
	}
	if err := c.client.UpdateCR(ctx, drive); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
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
	if err := c.client.ReadList(ctx, drivesList); err != nil {
		log.Errorf("failed to read drives list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	var labelRemovalErrMsgs []string

	// whether there is some drive with existing volumes in this storage group
	driveHasExistingVolumes := false
	for _, drive := range drivesList.Items {
		drive := drive
		if drive.Labels[apiV1.StorageGroupLabelKey] == sg.Name {
			successful, err := c.removeDriveAndACStorageGroupLabel(ctx, log, &drive, sg)
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
		return ctrl.Result{RequeueAfter: sgDeletionRetryInterval}, nil
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
	if err := c.client.ReadList(ctx, drivesList); err != nil {
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
	sg *sgcrd.StorageGroup) (bool, error) {
	log.Infof("try to remove storagegroup label of drive %s and its corresponding AC", drive.Name)

	// check whether this drive has existing volumes
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		log.Errorf("failed to get volumes on drive %s: %v", drive.Name, err)
		return false, err
	}
	if len(volumes) > 0 {
		log.Warnf("Drive %s has existing volumes and its sg label can't be removed.", drive.Name)
		return false, nil
	}

	ac, err := c.getACByDriveLocation(log, drive)
	if ac == nil && err != nil {
		return false, err
	}
	if ac != nil {
		if ac.Labels[apiV1.StorageGroupLabelKey] != sg.Name {
			log.Warnf("ac %s's storage group label is not %s", ac.Name, sg.Name)
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

	ac, err := c.getACByDriveLocation(log, drive)
	if ac == nil || ac.Spec.Size == 0 {
		if ac.Spec.Size == 0 {
			log.Warnf("not clean drive %s can't be selected by current storage group.", drive.Name)
		}
		return false, err
	}

	// the corresponding non-lvg ac exists and has free space, add storage-group label to the drive and corresponding ac
	if drive.Labels[apiV1.StorageGroupLabelKey] == sgName {
		log.Infof("label of storagegroup %s has been already added to drive %s", sgName, drive.Name)
	} else {
		if err = c.addDriveStorageGroupLabel(ctx, log, drive, sgName); err != nil {
			return false, err
		}
		log.Infof("Successfully add label of storagegroup %s to drive %s", sgName, drive.Name)
	}

	if err = c.addACStorageGroupLabel(ctx, log, ac, sgName); err != nil {
		return true, err
	}
	log.Infof("Successfully add label of storagegroup %s to drive %s and its corresponding AC", sgName, drive.Name)
	return true, nil
}

func (c *Controller) getACByDriveLocation(log *logrus.Entry, drive *drivecrd.Drive) (*accrd.AvailableCapacity, error) {
	ac, err := c.crHelper.GetACByLocation(drive.Spec.UUID)
	if err != nil {
		log.Errorf("Error when getting AC by the location of drive %s: %v", drive.Spec.UUID, err)
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	return ac, nil
}
