package storagegroup

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
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
			DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
				return false
			},
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
		return false
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

func (c *Controller) syncDriveStorageGroupLabel(ctx context.Context, drive *drivecrd.Drive) (ctrl.Result, error) {
	log := c.log.WithFields(logrus.Fields{"method": "syncDriveStorageGroupLabel", "name": drive.Name})
	log.Debugf("Sync storage group label of drive: %v", drive)

	ac, err := c.cachedCrHelper.GetACByLocation(drive.Spec.UUID)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	// TODO *handle the case that drive.Labels[apiV1.StorageGroupLabelKey] exists with value ""
	if ac.Labels[apiV1.StorageGroupLabelKey] != drive.Labels[apiV1.StorageGroupLabelKey] {
		ac.Labels[apiV1.StorageGroupLabelKey] = drive.Labels[apiV1.StorageGroupLabelKey]
		if ac.Labels[apiV1.StorageGroupLabelKey] == "" {
			delete(ac.Labels, apiV1.StorageGroupLabelKey)
		}
		if err1 := c.client.UpdateCR(ctx, ac); err1 != nil {
			log.Errorf("failed to sync storage-group label to ac %s with error %s", ac.Name, err.Error())
			return ctrl.Result{Requeue: true}, err1
		}
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
			if err := c.removeStorageGroupLabel(ctx, log, &drive, sg); err != nil {
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

					if err := c.addStorageGroupLabel(ctx, log, &drive, sg); err != nil {
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

func (c *Controller) removeStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
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
	delete(drive.Labels, apiV1.StorageGroupLabelKey)
	if err1 := c.client.UpdateCR(ctx, drive); err1 != nil {
		log.Errorf("failed to remove storage-group label from drive %s with error %s", drive.Name, err.Error())
		return err1
	}
	if ac.Labels[apiV1.StorageGroupLabelKey] == sg.Name {
		delete(ac.Labels, apiV1.StorageGroupLabelKey)
		if err1 := c.client.UpdateCR(ctx, ac); err1 != nil {
			log.Errorf("failed to remove storage-group label from ac %s with error %s", ac.Name, err.Error())
			return err1
		}
	} else {
		log.Warnf("ac %s not included in storage group %s", ac.Name, sg.Name)
	}
	return nil
}

func (c *Controller) addStorageGroupLabel(ctx context.Context, log *logrus.Entry, drive *drivecrd.Drive,
	sg *sgcrd.StorageGroup) error {
	log.Debugf("Expect to add label of storagegroup %s to drive %s", sg.Name, drive.Name)

	ac, err := c.cachedCrHelper.GetACByLocation(drive.Spec.UUID)
	if err != nil {
		return err
	}
	// the corresponding ac exists, add storage-group label to the drive and corresponding ac
	drive.Labels[apiV1.StorageGroupLabelKey] = sg.Name
	if err1 := c.client.UpdateCR(ctx, drive); err1 != nil {
		log.Errorf("failed to add storage-group label to drive %s with error %s", drive.Name, err.Error())
		return err1
	}
	ac.Labels[apiV1.StorageGroupLabelKey] = sg.Name
	if err1 := c.client.UpdateCR(ctx, ac); err1 != nil {
		log.Errorf("failed to add storage-group label to ac %s with error %s", ac.Name, err.Error())
		return err1
	}
	log.Debugf("Successfully add label of storagegroup %s to drive %s", sg.Name, drive.Name)
	return nil
}
