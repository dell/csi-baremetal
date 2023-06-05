package storagegroup

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
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
	sgFinalizer               = "dell.emc.csi/sg-cleanup"
	sgTempStatusAnnotationKey = "storagegroup.csi-baremetal.dell.com/status"
	contextTimeoutSeconds     = 60
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
	var (
		oldDrive *drivecrd.Drive
		newDrive *drivecrd.Drive
		ok       bool
	)
	if oldDrive, ok = old.(*drivecrd.Drive); !ok {
		// TODO need to restrict storage group update event handling
		return true
	}
	if newDrive, ok = new.(*drivecrd.Drive); ok {
		return filterDrive(oldDrive, newDrive)
	}
	return false
}

func filterDrive(old *drivecrd.Drive, new *drivecrd.Drive) bool {
	return old.Labels[apiV1.StorageGroupLabelKey] != new.Labels[apiV1.StorageGroupLabelKey]
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
	if err := c.client.ReadCR(ctx, name, "", drive); err == nil {
		return c.syncDriveStorageGroupLabel(ctx, drive)
	}

	storageGroup := &sgcrd.StorageGroup{}
	if err := c.client.ReadCR(ctx, name, "", storageGroup); err != nil {
		log.Warningf("Failed to read StorageGroup %s", name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Debugf("Reconcile StorageGroup: %v", storageGroup)

	// StorageGroup Deletion request
	if !storageGroup.DeletionTimestamp.IsZero() {
		return c.handleStorageGroupDeletion(ctx, log, storageGroup)
	}

	if !util.ContainsString(storageGroup.Finalizers, sgFinalizer) {
		// append finalizer
		log.Debugf("Appending finalizer for StorageGroup")
		storageGroup.Finalizers = append(storageGroup.Finalizers, sgFinalizer)
		if err := c.client.UpdateCR(ctx, storageGroup); err != nil {
			log.Errorf("Unable to append finalizer %s to StorageGroup, error: %v.", sgFinalizer, err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	sgStatus, ok := storageGroup.Annotations[sgTempStatusAnnotationKey]
	if !ok || sgStatus == apiV1.Creating {
		return c.handleStorageGroupCreation(ctx, log, storageGroup)
	}

	return ctrl.Result{}, nil
}

// combine the following similar funcs
func (c *Controller) removeACStorageGroupLabel(ctx context.Context, log *logrus.Entry, ac *accrd.AvailableCapacity) error {
	delete(ac.Labels, apiV1.StorageGroupLabelKey)
	if err1 := c.client.UpdateCR(ctx, ac); err1 != nil {
		log.Errorf("failed to remove storage-group label from ac %s with error %s", ac.Name, err1.Error())
		return err1
	}
	return nil
}

func (c *Controller) removeDriveStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive) error {
	delete(drive.Labels, apiV1.StorageGroupLabelKey)
	if err1 := c.client.UpdateCR(ctx, drive); err1 != nil {
		log.Errorf("failed to remove storage-group label from drive %s with error %s", drive.Name, err1.Error())
		return err1
	}
	return nil
}

func (c *Controller) addDriveStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sgName string) error {
	drive.Labels[apiV1.StorageGroupLabelKey] = sgName
	if err1 := c.client.UpdateCR(ctx, drive); err1 != nil {
		log.Errorf("failed to add storage group %s label to drive %s with error %s", sgName, drive.Name, err1.Error())
		return err1
	}
	return nil
}

func (c *Controller) addACStorageGroupLabel(ctx context.Context, log *logrus.Entry, ac *accrd.AvailableCapacity,
	sgName string) error {
	ac.Labels[apiV1.StorageGroupLabelKey] = sgName
	if err1 := c.client.UpdateCR(ctx, ac); err1 != nil {
		log.Errorf("failed to add storage group %s label to ac %s with error %s", sgName, ac.Name, err1.Error())
		return err1
	}
	return nil
}

func (c *Controller) syncDriveStorageGroupLabel(ctx context.Context, drive *drivecrd.Drive) (ctrl.Result, error) {
	log := c.log.WithFields(logrus.Fields{"method": "syncDriveStorageGroupLabel", "name": drive.Name})

	location := drive.Name
	lvg, err := c.crHelper.GetLVGByDrive(ctx, drive.Spec.UUID)
	if err != nil {
		log.Errorf("Error when getting LVG by drive %s: %v", drive.Name, err)
		return ctrl.Result{Requeue: true}, err
	}
	if lvg != nil {
		location = lvg.Name
	}

	ac, err := c.cachedCrHelper.GetACByLocation(location)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	acSGLabel, acSGLabeled := ac.Labels[apiV1.StorageGroupLabelKey]
	driveSGLabel, driveSGLabeled := drive.Labels[apiV1.StorageGroupLabelKey]
	if acSGLabel == driveSGLabel {
		return ctrl.Result{}, nil
	}

	log.Debugf("Sync storage group label of drive %s", drive.Name)

	switch {
	// add new storagegroup label to drive
	case !acSGLabeled && driveSGLabeled:
		if lvg != nil {
			log.Warnf("We can't add storage group label to drive %s with existing LVG", drive.Name)
			if err := c.removeDriveStorageGroupLabel(ctx, log, drive); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			return ctrl.Result{}, nil
		}

		volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
		if err != nil {
			log.Errorf("Error when getting volumes on drive %s: %s", drive.Name, err.Error())
			return ctrl.Result{Requeue: true}, err
		}
		if len(volumes) > 0 {
			log.Warnf("We can't add storage group label to drive %s with existing volumes", drive.Name)
			if err := c.removeDriveStorageGroupLabel(ctx, log, drive); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			return ctrl.Result{}, nil
		}
		log.Debugf("Also add storage-group %s label to AC %s corresponding to drive %s", driveSGLabel, ac.Name, drive.Name)
		if err = c.addACStorageGroupLabel(ctx, log, ac, driveSGLabel); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

	// remove storagegroup label from drive
	case acSGLabeled && !driveSGLabeled:
		volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
		if err != nil {
			log.Errorf("Error when getting volumes on drive %s: %s", drive.Name, err.Error())
			return ctrl.Result{Requeue: true}, err
		}
		if len(volumes) > 0 {
			log.Warnf("We can't remove storage group %s label from drive %s with existing volumes",
				acSGLabel, drive.Name)
			if err := c.addDriveStorageGroupLabel(ctx, log, drive, acSGLabel); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			return ctrl.Result{}, nil
		}

		sg := &sgcrd.StorageGroup{}
		err = c.client.ReadCR(ctx, acSGLabel, "", sg)
		switch {
		case err == nil && isDriveSelectedByValidMatchFields(&drive.Spec, &sg.Spec.DriveSelector.MatchFields):
			log.Warnf("We can't remove storage group %s label from drive %s still selected by this storage group",
				acSGLabel, drive.Name)
			if err := c.addDriveStorageGroupLabel(ctx, log, drive, acSGLabel); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			return ctrl.Result{}, nil
		case err != nil && !k8serrors.IsNotFound(err):
			log.Errorf("Failed to read StorageGroup %s with error %v", acSGLabel, err)
			return ctrl.Result{Requeue: true}, err

		// the case that the storage-group label removal is valid and we should sync the removal to AC
		default:
			log.Debugf("Also remove the storage-group %s label of AC %s corresponding to drive %s", acSGLabel,
				ac.Name, drive.Name)
			if err := c.removeACStorageGroupLabel(ctx, log, ac); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}

		// restore the update of storagegroup label of drive
	}

	return ctrl.Result{}, nil
}

func (c *Controller) handleStorageGroupDeletion(ctx context.Context, log *logrus.Entry,
	sg *sgcrd.StorageGroup) (ctrl.Result, error) {
	drivesList := &drivecrd.DriveList{}
	if err := c.client.ReadList(ctx, drivesList); err != nil {
		log.Errorf("failed to read drives list: %s", err.Error())
		return ctrl.Result{Requeue: true}, err
	}
	removalNoError := true
	for _, drive := range drivesList.Items {
		drive := drive
		if drive.Labels[apiV1.StorageGroupLabelKey] == sg.Name {
			if err := c.removeDriveAndACStorageGroupLabel(ctx, log, &drive, sg); err != nil {
				log.Errorf("error in remove storage-group label from drive %s", err.Error())
				removalNoError = false
			}
		}
	}
	if !removalNoError {
		return ctrl.Result{Requeue: true}, fmt.Errorf("error in removing storage-group label")
	}
	return c.removeFinalizer(ctx, log, sg)
}

func (c *Controller) removeFinalizer(ctx context.Context, log *logrus.Entry,
	sg *sgcrd.StorageGroup) (ctrl.Result, error) {
	if util.ContainsString(sg.Finalizers, sgFinalizer) {
		sg.Finalizers = util.RemoveString(sg.Finalizers, sgFinalizer)
		if err := c.client.UpdateCR(ctx, sg); err != nil {
			log.Errorf("Unable to remove finalizer %s from StorageGroup, error: %v.", sgFinalizer, err)
			return ctrl.Result{Requeue: true}, err
		}
	}
	return ctrl.Result{}, nil
}

func (c *Controller) handleStorageGroupCreation(ctx context.Context, log *logrus.Entry,
	sg *sgcrd.StorageGroup) (ctrl.Result, error) {
	if !c.isStorageGroupValid(log, sg) {
		return c.updateStorageGroupStatus(ctx, log, sg, apiV1.Created)
	}
	drivesList := &drivecrd.DriveList{}
	if err := c.client.ReadList(ctx, drivesList); err != nil {
		log.Errorf("failed to read drives list: %s", err.Error())
		return ctrl.Result{Requeue: true}, err
	}
	labelingNoError := true
	noDriveSelected := true
	drivesCount := map[string]int32{}
	driveSelector := sg.Spec.DriveSelector
	for _, drive := range drivesList.Items {
		drive := drive
		if isDriveSelectedByValidMatchFields(&drive.Spec, &driveSelector.MatchFields) &&
			(driveSelector.NumberDrivesPerNode == 0 || drivesCount[drive.Spec.NodeId] < driveSelector.NumberDrivesPerNode) {
			existingStorageGroup, exists := drive.Labels[apiV1.StorageGroupLabelKey]
			if !exists || (exists && existingStorageGroup == sg.Name) {
				if driveSelector.NumberDrivesPerNode > 0 {
					drivesCount[drive.Spec.NodeId]++
				}
				if exists {
					log.Infof("Drive %s has already been selected by current storage group", drive.Name)
					noDriveSelected = false
				} else {
					// TODO refactor to reduce cognitive complexity
					if lvg, err := c.crHelper.GetLVGByDrive(ctx, drive.Spec.UUID); err != nil || lvg != nil {
						if err != nil {
							log.Errorf("Error when getting LVG by drive %s: %s", drive.Name, err.Error())
							labelingNoError = false
						} else {
							log.Warnf("Drive %s has existing LVG and can't be selected by current storage group.",
								drive.Name)
						}
						continue
					}

					if volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID); err != nil || len(volumes) > 0 {
						if err != nil {
							log.Errorf("Error when getting volumes on drive %s: %s", drive.Name, err.Error())
							labelingNoError = false
						} else {
							log.Warnf("Drive %s has existing volumes and can't be selected by current storage group.",
								drive.Name)
						}
						continue
					}

					if err := c.addDriveAndACStorageGroupLabel(ctx, log, &drive, sg); err != nil {
						log.Errorf("Error in adding storage-group label to drive %s: %s", drive.Name, err.Error())
						labelingNoError = false
					}
					noDriveSelected = false
				}
			} else {
				log.Warnf("Drive %s has already been selected by storage group %s "+
					"and can't be selected by current storage group", drive.Name, existingStorageGroup)
			}
		}
	}
	if noDriveSelected {
		log.Warnf("No drive can be selected by current storage group %s", sg.Name)
	}
	if labelingNoError {
		return c.updateStorageGroupStatus(ctx, log, sg, apiV1.Created)
	}
	return ctrl.Result{Requeue: true}, fmt.Errorf("error in adding storage-group label")
}

func isDriveSelectedByValidMatchFields(drive *api.Drive, matchFields *map[string]string) bool {
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
				log.Warnf("Invalid field value %s for field %s", fieldValue, fieldName)
				return false
			}
		case "bool":
			if _, err := strconv.ParseBool(fieldValue); err != nil {
				log.Warnf("Invalid field value %s for field %s", fieldValue, fieldName)
				return false
			}
		}
	}
	return true
}

// TODO Need more check on whether storagegroup is valid
func (c *Controller) isStorageGroupValid(log *logrus.Entry, sg *sgcrd.StorageGroup) bool {
	return c.isMatchFieldsValid(log, &sg.Spec.DriveSelector.MatchFields)
}

func (c *Controller) updateStorageGroupStatus(ctx context.Context, log *logrus.Entry, sg *sgcrd.StorageGroup,
	status string) (ctrl.Result, error) {
	sg.Annotations[sgTempStatusAnnotationKey] = status
	if err := c.client.UpdateCR(ctx, sg); err != nil {
		log.Errorf("Unable to update StorageGroup status, error: %v.", err)
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (c *Controller) removeDriveAndACStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sg *sgcrd.StorageGroup) error {
	log.Debugf("Remove storagegroup label from drive %s", drive.Name)
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		return err
	}
	if len(volumes) > 0 {
		log.Warnf("Drive %s has existing volumes. Storage group label can't be removed.", drive.Name)
		return fmt.Errorf("error in removing storage-group label on drive")
	}

	ac, err := c.cachedCrHelper.GetACByLocation(drive.Spec.UUID)
	if err != nil {
		return err
	}
	if err = c.removeDriveStorageGroupLabel(ctx, log, drive); err != nil {
		return err
	}
	if ac.Labels[apiV1.StorageGroupLabelKey] == sg.Name {
		if err = c.removeACStorageGroupLabel(ctx, log, ac); err != nil {
			return err
		}
	} else {
		log.Warnf("ac %s not included in storage group %s", ac.Name, sg.Name)
	}
	return nil
}

func (c *Controller) addDriveAndACStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sg *sgcrd.StorageGroup) error {
	log.Debugf("Expect to add label of storagegroup %s to drive %s", sg.Name, drive.Name)

	ac, err := c.cachedCrHelper.GetACByLocation(drive.Spec.UUID)
	if err != nil {
		return err
	}
	// the corresponding ac exists, add storage-group label to the drive and corresponding ac
	if err = c.addDriveStorageGroupLabel(ctx, log, drive, sg.Name); err != nil {
		return err
	}
	if err = c.addACStorageGroupLabel(ctx, log, ac, sg.Name); err != nil {
		return err
	}
	log.Debugf("Successfully add label of storagegroup %s to drive %s and its corresponding AC", sg.Name, drive.Name)
	return nil
}
