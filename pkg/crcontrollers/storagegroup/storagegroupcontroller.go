package storagegroup

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

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
		Complete(c)
}

// Reconcile reconciles StorageGroup custom resources
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(ctx, contextTimeoutSeconds*time.Second)
	defer cancelFn()

	// read name
	name := req.Name
	// customize logging
	log := c.log.WithFields(logrus.Fields{"method": "Reconcile", "name": name})

	// obtain corresponding storage group
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
		// TODO put all of this part in func handleStorageGroupCreation
		newStatus, err := c.handleStorageGroupCreation(ctx, log, storageGroup)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		storageGroup.Annotations[sgTempStatusAnnotationKey] = newStatus
		if err := c.client.UpdateCR(ctx, storageGroup); err != nil {
			log.Errorf("Unable to update StorageGroup status, error: %v.", sgFinalizer, err)
			return ctrl.Result{Requeue: true}, err
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
		if drive.Labels[apiV1.StorageGroupLabelKey] == sg.Name {
			if err := c.removeStorageGroupLabel(ctx, log, &drive, sg); err != nil {
				log.Errorf("Error in remove storage-group label from drive %s", err.Error())
				removalNoError = false
			}
		}
	}
	if !removalNoError {
		return ctrl.Result{Requeue: true}, fmt.Errorf("Error in removing storage-group label")
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
	sg *sgcrd.StorageGroup) (string, error) {
	drivesList := &drivecrd.DriveList{}
	if err := c.client.ReadList(ctx, drivesList); err != nil {
		log.Errorf("failed to read drives list: %s", err.Error())
		return apiV1.Creating, err
	}
	labelingNoError := true
	invalidField := false
	noDriveSelected := true
	drivesCount := map[string]int32{}
	driveSelector := sg.Spec.DriveSelector
	for _, drive := range drivesList.Items {
		driveSelected := true
		for fieldName, fieldValue := range driveSelector.MatchFields {
			driveField := reflect.ValueOf(&(drive.Spec)).Elem().FieldByName(fieldName)
			invalidField = !driveField.IsValid()
			if invalidField {
				driveSelected = false
				break
			}
			switch driveField.Type().String() {
			case "string":
				if driveField.String() != fieldValue {
					driveSelected = false
				}
			case "int64":
				fieldValueInt64, err := strconv.ParseInt(fieldValue, 10, 64)
				invalidField = err != nil
				if invalidField || driveField.Int() != fieldValueInt64 {
					driveSelected = false
				}
			case "bool":
				fieldValueBool, err := strconv.ParseBool(fieldValue)
				invalidField = err != nil
				if invalidField && driveField.Bool() != fieldValueBool {
					driveSelected = false
				}
			}
			if invalidField || !driveSelected {
				break
			}
		}
		if invalidField {
			log.Errorf("Invalid field term in driveSelector of storage group %s", sg.Name)
			break
		}
		if driveSelected && (driveSelector.NumberDrivesPerNode == 0 || drivesCount[drive.Spec.NodeId] < driveSelector.NumberDrivesPerNode) {
			noDriveSelected = false
			if driveSelector.NumberDrivesPerNode > 0 {
				drivesCount[drive.Spec.NodeId]++
			}
			if err := c.addStorageGroupLabel(ctx, log, &drive, sg); err != nil {
				log.Errorf("Error in adding storage-group label to drive %s", err.Error())
				labelingNoError = false
			}
		}
	}
	if noDriveSelected {
		log.Warnf("No drive selected by driveSelector of storage group %s", sg.Name)
	}
	if labelingNoError {
		return apiV1.Created, nil
	}
	return apiV1.Creating, fmt.Errorf("Error in adding storage-group label")
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
		return fmt.Errorf("Error in removing storage-group label on drive")
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
	if existingStorageGroup, ok := drive.Labels[apiV1.StorageGroupLabelKey]; ok {
		log.Warnf("Drive %s already has already been selected by storage group %s", drive.Name, existingStorageGroup)
		return nil
	}
	volumes, err := c.crHelper.GetVolumesByLocation(ctx, drive.Spec.UUID)
	if err != nil {
		return err
	}
	if len(volumes) > 0 {
		log.Warnf("Drive %s already has existing volumes. Storage group label won't be added.", drive.Name)
		return nil
	}

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
	return nil
}
