package capacitycontroller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	metricsC "github.com/dell/csi-baremetal/pkg/metrics/common"
)

// RequeueDriveTime is time between drives reconciliation
const RequeueDriveTime = time.Second * 30

// Controller reconciles drive custom resource
type Controller struct {
	client   *k8s.KubeClient
	crHelper *k8s.CRHelper
	// CRHelper instance which reads from cache
	cachedCrHelper *k8s.CRHelper
	log            *logrus.Entry
}

// NewCapacityController creates new instance of Controller structure
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of Controller
func NewCapacityController(client *k8s.KubeClient, k8sCache k8s.CRReader, log *logrus.Logger) *Controller {
	return &Controller{
		client:         client,
		crHelper:       k8s.NewCRHelper(client, log),
		cachedCrHelper: k8s.NewCRHelper(client, log).SetReader(k8sCache),
		log:            log.WithField("component", "Controller"),
	}
}

// SetupWithManager registers Controller to ControllerManager
func (d *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&drivecrd.Drive{}).
		Watches(&source.Kind{Type: &lvgcrd.LogicalVolumeGroup{}}, &handler.EnqueueRequestForObject{}).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return d.filterUpdateEvent(e.ObjectOld, e.ObjectNew)
			},
		}).
		Complete(d)
}

// Reconcile reconciles Drive custom resources
func (d *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	defer metricsC.ReconcileDuration.EvaluateDurationForType("csicontroller_drive_controller")()
	resourceName := req.Name
	ctx, cancelFn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFn()

	log := d.log.WithFields(logrus.Fields{"method": "Reconcile", "name": resourceName})

	drive := &drivecrd.Drive{}
	if err := d.client.ReadCR(ctx, resourceName, "", drive); err == nil {
		return d.reconcileDrive(ctx, drive)
	}
	lvg := &lvgcrd.LogicalVolumeGroup{}
	if err := d.client.ReadCR(ctx, resourceName, "", lvg); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Warnf("Failed to read LVG and Drive CRs, resource with name %s is not found", resourceName)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return d.reconcileLVG(lvg)
}

// reconcileLVG perform logic for LVG reconciliation
func (d *Controller) reconcileLVG(lvg *lvgcrd.LogicalVolumeGroup) (ctrl.Result, error) {
	log := d.log.WithFields(logrus.Fields{
		"method": "reconcileLVG",
	})
	var (
		status = lvg.Spec.GetStatus()
		health = lvg.Spec.GetHealth()
		name   = lvg.GetName()
	)
	// If LVG status is failed or Health is not good, try to reset its AC size to 0
	if status == apiV1.Failed || health != apiV1.HealthGood {
		return ctrl.Result{}, d.resetACSizeOfLVG(name)
	}
	// If LVG is already presented on a machine but doesn't have AC, try to create its AC using annotation with
	// VG free space
	size, err := getFreeSpaceFromLVGAnnotation(lvg.Annotations)
	if err != nil {
		log.Errorf("Failed to get free space from LVG %v, err: %v", lvg, err)
		if err == errTypes.ErrorNotFound {
			err = nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, d.createOrUpdateLVGCapacity(lvg, size)
}

// reconcileDrive preforms logic for drive reconciliation
func (d *Controller) reconcileDrive(ctx context.Context, drive *drivecrd.Drive) (ctrl.Result, error) {
	var (
		health = drive.Spec.GetHealth()
		status = drive.Spec.GetStatus()
		usage  = drive.Spec.GetUsage()
	)
	switch {
	case (health != apiV1.HealthGood && health != apiV1.HealthUnknown) ||
		status != apiV1.DriveStatusOnline ||
		usage != apiV1.DriveUsageInUse:
		return d.handleInaccessibleDrive(ctx, drive.Spec)
	default:
		return d.createOrUpdateCapacity(ctx, drive.Spec)
	}
}

// createOrUpdateCapacity tries to create AC for drive or update its size if AC already exists
func (d *Controller) createOrUpdateCapacity(ctx context.Context, drive api.Drive) (ctrl.Result, error) {
	log := d.log.WithFields(logrus.Fields{
		"method": "createOrUpdateCapacity",
	})
	driveUUID := drive.GetUUID()
	size := drive.GetSize()
	// if drive is not clean, size is 0
	if !drive.GetIsClean() {
		size = 0
	}
	ac, err := d.cachedCrHelper.GetACByLocation(driveUUID)
	switch {
	case err == nil:
		// If ac is exists, update its size to drive size
		if ac.Spec.Size != size {
			ac.Spec.Size = size
			if err := d.client.Update(context.WithValue(ctx, base.RequestUUID, ac.Name), ac); err != nil {
				log.Errorf("Error during update AvailableCapacity request to k8s: %v, error: %v", ac, err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: RequeueDriveTime}, nil
	case err == errTypes.ErrorNotFound:
		name := uuid.New().String()
		if lvg, err := d.crHelper.GetLVGByDrive(ctx, driveUUID); err != nil || lvg != nil {
			return ctrl.Result{}, err
		}
		capacity := &api.AvailableCapacity{
			Size:         size,
			Location:     driveUUID,
			StorageClass: util.ConvertDriveTypeToStorageClass(drive.GetType()),
			NodeId:       drive.GetNodeId(),
		}
		newAC := d.client.ConstructACCR(name, *capacity)
		if err := d.client.CreateCR(context.WithValue(ctx, base.RequestUUID, name), name, newAC); err != nil {
			log.Errorf("Error during create AvailableCapacity request to k8s: %v, error: %v",
				capacity, err)
			return ctrl.Result{}, err
		}
	default:
		log.Errorf("Failed to read AvailableCapacity for drive %s: %v", driveUUID, err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: RequeueDriveTime}, nil
}

// handleInaccessibleDrive deletes AC for bad Drive
func (d *Controller) handleInaccessibleDrive(ctx context.Context, drive api.Drive) (ctrl.Result, error) {
	log := d.log.WithFields(logrus.Fields{
		"method": "handleInaccessibleDrive",
	})
	ac, err := d.cachedCrHelper.GetACByLocation(drive.GetUUID())
	switch {
	case err == nil:
		log.Infof("Update AC size to 0 %s based on unhealthy location %s", ac.Name, ac.Spec.Location)
		ac.Spec.Size = 0
		if err := d.client.UpdateCR(ctx, ac); err != nil {
			log.Errorf("Failed to update unhealthy available capacity CR: %v", err)
			return ctrl.Result{}, err
		}
	case err != errTypes.ErrorNotFound:
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: RequeueDriveTime}, nil
}

// createOrUpdateLVGCapacity creates AC for LVG
func (d *Controller) createOrUpdateLVGCapacity(lvg *lvgcrd.LogicalVolumeGroup, size int64) error {
	ll := d.log.WithFields(logrus.Fields{
		"method": "createACIfFreeSpace",
	})
	if size == 0 {
		size++ // if size is 0 it field will not display for CR
	}
	var (
		location  = lvg.GetName()
		driveUUID = lvg.Spec.Locations[0]
	)
	// check whether AC exists
	ac, err := d.cachedCrHelper.GetACByLocation(location)
	switch {
	case err == nil:
		if ac.Spec.Size != size {
			ac.Spec.Size += size
			if err := d.client.UpdateCR(context.Background(), ac); err != nil {
				d.log.Errorf("Unable to update AC CR %s, error: %v.", ac.Name, err)
				return err
			}
		}
		return nil
	case err == errTypes.ErrorNotFound:
		if size > capacityplanner.AcSizeMinThresholdBytes {
			ac, err := d.cachedCrHelper.GetACByLocation(driveUUID)
			if err != nil && err != errTypes.ErrorNotFound {
				ll.Infof("Failed to get drive AC by UUID %s, err: %v", driveUUID, err)
				return err
			}
			if err == errTypes.ErrorNotFound {
				ll.Infof("Creating SYSLVG AC for lvg %s", location)
				name := uuid.New().String()
				capacity := &api.AvailableCapacity{
					Size:         size,
					Location:     lvg.Name,
					StorageClass: apiV1.StorageClassSystemLVG,
					NodeId:       lvg.Spec.Node,
				}
				ac = d.client.ConstructACCR(name, *capacity)
				if err := d.client.CreateCR(context.Background(), name, ac); err != nil {
					return fmt.Errorf("unable to create AC based on system LogicalVolumeGroup, error: %v", err)
				}
			} else {
				ll.Infof("Replacing AC %s location from drive %s with lvg %s", ac.Name, ac.Spec.Location, location)
				ac.Spec.Size = size
				ac.Spec.Location = location
				ac.Spec.StorageClass = apiV1.StorageClassSystemLVG
				if err := d.client.UpdateCR(context.Background(), ac); err != nil {
					return fmt.Errorf("unable to create AC based on system LogicalVolumeGroup, error: %v", err)
				}
			}
			ll.Infof("Created AC %+v for lvg %s", ac, location)
			return nil
		}
		ll.Infof("There is no available space on %s", location)
		return nil
	default:
		return err
	}
}

// resetACSize sets size of corresponding AC to 0 to avoid further allocations
func (d *Controller) resetACSizeOfLVG(lvgName string) error {
	// read AC
	ac, err := d.cachedCrHelper.GetACByLocation(lvgName)
	if err != nil {
		if err == errTypes.ErrorNotFound {
			// non re-triable error
			d.log.Errorf("AC CR for LogicalVolumeGroup %s not found", lvgName)
			return nil
		}
		return err
	}
	if ac.Spec.Size != 0 {
		ac.Spec.Size = 0
		if err := d.client.UpdateCR(context.Background(), ac); err != nil {
			d.log.Errorf("Unable to update AC CR %s, error: %v.", ac.Name, err)
			return err
		}
	}
	return nil
}

func (d *Controller) filterUpdateEvent(old runtime.Object, new runtime.Object) bool {
	var (
		oldDrive *drivecrd.Drive
		newDrive *drivecrd.Drive
		ok       bool
	)
	if oldDrive, ok = old.(*drivecrd.Drive); !ok {
		return handleLVGObjects(old, new)
	}
	if newDrive, ok = new.(*drivecrd.Drive); ok {
		return filter(oldDrive.Spec, newDrive.Spec)
	}
	return true
}

func handleLVGObjects(old runtime.Object, new runtime.Object) bool {
	var (
		oldLVG *lvgcrd.LogicalVolumeGroup
		newLVG *lvgcrd.LogicalVolumeGroup
		ok     bool
	)
	if oldLVG, ok = old.(*lvgcrd.LogicalVolumeGroup); !ok {
		return false
	}
	if newLVG, ok = new.(*lvgcrd.LogicalVolumeGroup); ok {
		return filterLVG(oldLVG, newLVG)
	}
	return false
}

func filter(old api.Drive, new api.Drive) bool {
	// controller perform reconcile for drives, which have different statuses, health or isClean field.
	// Another drives are skipped
	return old.GetIsClean() != new.GetIsClean() ||
		old.GetStatus() != new.GetStatus() ||
		old.GetHealth() != new.GetHealth()
}

func filterLVG(old *lvgcrd.LogicalVolumeGroup, new *lvgcrd.LogicalVolumeGroup) bool {
	// controller perform reconcile for lvg, which have different statuses, health or annotation field.
	// Another LVGs are skipped
	return (new.Spec.GetHealth() != apiV1.HealthGood && old.Spec.GetHealth() != new.Spec.GetHealth()) ||
		(new.Spec.GetStatus() == apiV1.Failed && old.Spec.GetStatus() != new.Spec.GetStatus()) ||
		checkLVGAnnotation(old.Annotations, new.Annotations)
}

func checkLVGAnnotation(oldAnnotation, newAnnotation map[string]string) bool {
	newSize, errNew := getFreeSpaceFromLVGAnnotation(newAnnotation)
	oldSize, errOld := getFreeSpaceFromLVGAnnotation(oldAnnotation)

	// Controller doesn't perform reconcile if new lvg doesn't have annotation
	if errNew != nil {
		return false
	}

	// Controller performs reconcile if both lvg have different VG space, for example old lvg doesn't have annotation
	// new lvg have annotation. In this case oldSize = 0 and newSize equals size from annotation. This logic is used
	// when lvg is already presented on a machine, but doesn't have AC
	if errOld != nil && errOld != errTypes.ErrorNotFound {
		return false
	}

	if newSize != oldSize {
		return true
	}
	return false
}

func getFreeSpaceFromLVGAnnotation(annotation map[string]string) (int64, error) {
	if annotation != nil {
		if sizeString, ok := annotation[apiV1.LVGFreeSpaceAnnotation]; ok {
			size, err := strconv.ParseInt(sizeString, 10, 64)
			if err != nil {
				return 0, err
			}
			return size, nil
		}
	}

	return 0, errTypes.ErrorNotFound
}
