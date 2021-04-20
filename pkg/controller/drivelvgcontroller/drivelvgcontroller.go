package drive

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	metricsC "github.com/dell/csi-baremetal/pkg/metrics/common"
)

// Controller reconciles drive custom resource
type Controller struct {
	client   *k8s.KubeClient
	crHelper *k8s.CRHelper
	// CRHelper instance which reads from cache
	cachedCrHelper *k8s.CRHelper
	log            *logrus.Entry
}

// NewDriveController creates new instance of Controller structure
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of Controller
func NewDriveController(client *k8s.KubeClient, k8sCache k8s.CRReader, log *logrus.Logger) *Controller {
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
	log.Warnf("Failed to read Drive %s CR, try to read LVG CR", resourceName)
	lvg := &lvgcrd.LogicalVolumeGroup{}
	if err := d.client.ReadCR(ctx, resourceName, "", lvg); err != nil {
		log.Errorf("Failed to read LVG %s CR", resourceName)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return d.reconcileLVG(lvg)
}

func (d *Controller) reconcileLVG(lvg *lvgcrd.LogicalVolumeGroup) (ctrl.Result, error) {
	log := d.log.WithFields(logrus.Fields{
		"method": "reconcileLVG",
	})
	var (
		status = lvg.Spec.GetStatus()
		health = lvg.Spec.GetHealth()
		name   = lvg.GetName()
		node   = lvg.Spec.GetNode()
	)
	if status == apiV1.Failed || health != apiV1.HealthGood {
		return ctrl.Result{}, d.resetACSizeOfLVG(name)
	}
	if len(lvg.Annotations) != 0 {
		if sizeString, ok := lvg.Annotations[apiV1.LVGFreeSpaceAnnotation]; ok {
			byteSize, err := strconv.ParseInt(sizeString, 10, 64)
			if err != nil {
				log.Errorf("Failed to convert string size %s to int64, err: %v", sizeString, err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, d.createACIfNotExists(name, node, byteSize)
		}
		log.Warnf("LVG doesn't contains annotation: %s", apiV1.LVGFreeSpaceAnnotation)
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, d.createACIfNotExists(name, node, lvg.Spec.GetSize())
}

func (d *Controller) reconcileDrive(ctx context.Context, drive *drivecrd.Drive) (ctrl.Result, error) {
	var (
		health  = drive.Spec.GetHealth()
		status  = drive.Spec.GetStatus()
		isClean = drive.Spec.GetIsClean()
	)

	switch {
	case isClean && (health == apiV1.HealthGood && status == apiV1.DriveStatusOnline):
		return d.handleDrive(ctx, drive.Spec)
	case health != apiV1.HealthGood || status != apiV1.DriveStatusOnline:
		return d.handleDriveIsNotGood(ctx, drive.Spec)
	case !isClean:
		return d.handleDrive(ctx, drive.Spec)
	}
	return ctrl.Result{}, nil
}

func (d *Controller) handleDrive(ctx context.Context, drive api.Drive) (ctrl.Result, error) {
	return d.createACIfNotExistOrUpdate(ctx, drive)
}

func (d *Controller) createACIfNotExistOrUpdate(ctx context.Context, drive api.Drive) (ctrl.Result, error) {
	log := d.log.WithFields(logrus.Fields{
		"method": "createACIfNotExistOrUpdate",
	})
	driveUUID := drive.GetUUID()
	size := drive.GetSize()
	if !drive.GetIsClean() {
		size = 0
	}
	ac, err := d.cachedCrHelper.GetACByLocation(driveUUID)
	switch {
	case err == nil:
		ac.Spec.Size = size
		if err := d.client.Update(context.WithValue(ctx, base.RequestUUID, ac.Name), ac); err != nil {
			log.Errorf("Error during update AvailableCapacity request to k8s: %v, error: %v", ac, err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	case err == errTypes.ErrorNotFound:
		name := uuid.New().String()
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
		log.Infof("Failed to read AvailableCapacity for drive %s: %v", driveUUID, err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (d *Controller) handleDriveIsNotGood(ctx context.Context, drive api.Drive) (ctrl.Result, error) {
	log := d.log.WithFields(logrus.Fields{
		"method": "handleDriveIsNotGood",
	})
	ac, err := d.cachedCrHelper.GetACByLocation(drive.GetUUID())
	switch {
	case err == nil:
		log.Infof("Removing AC %s based on unhealthy location %s", ac.Name, ac.Spec.Location)
		if err := d.client.DeleteCR(ctx, ac); err != nil {
			log.Errorf("Failed to delete unhealthy available capacity CR: %v", err)
			return ctrl.Result{}, err
		}
	case err != errTypes.ErrorNotFound:
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (d *Controller) createACIfNotExists(location, nodeID string, size int64) error {
	ll := d.log.WithFields(logrus.Fields{
		"method": "createACIfFreeSpace",
	})
	if size == 0 {
		size++ // if size is 0 it field will not display for CR
	}
	// check whether AC exists
	if ac, _ := d.cachedCrHelper.GetACByLocation(location); ac != nil {
		if ac.Spec.Size != size {
			ac.Spec.Size = size
			if err := d.client.UpdateCR(context.Background(), ac); err != nil {
				d.log.Errorf("Unable to update AC CR %s, error: %v.", ac.Name, err)
				return err
			}
		}
		return nil
	}

	if size > capacityplanner.AcSizeMinThresholdBytes {
		acName := uuid.New().String()
		acCR := d.client.ConstructACCR(acName, api.AvailableCapacity{
			Location:     location,
			NodeId:       nodeID,
			StorageClass: apiV1.StorageClassSystemLVG,
			Size:         size,
		})
		if err := d.client.CreateCR(context.Background(), acName, acCR); err != nil {
			return fmt.Errorf("unable to create AC based on system LogicalVolumeGroup, error: %v", err)
		}
		ll.Infof("Created AC %v for lvg %s", acCR, location)
		return nil
	}
	ll.Infof("There is no available space on %s", location)
	return nil
}

// resetACSize sets size of corresponding AC to 0 to avoid further allocations
func (d *Controller) resetACSizeOfLVG(lvgName string) error {
	var (
		err error
		ac  *accrd.AvailableCapacity
	)
	// read AC
	if ac, err = d.cachedCrHelper.GetACByLocation(lvgName); err == nil {
		// update if not null already
		if ac.Spec.Size != 0 {
			ac.Spec.Size = 0
			if err := d.client.UpdateCR(context.Background(), ac); err != nil {
				d.log.Errorf("Unable to set AC CR %s size to 0, error: %v.", ac.Name, err)
				return err
			}
		}
		return nil
	}
	if err == errTypes.ErrorNotFound {
		// non re-triable error
		d.log.Errorf("AC CR for LogicalVolumeGroup %s not found", lvgName)
		return nil
	}
	return err
}

func (d *Controller) filterUpdateEvent(old runtime.Object, new runtime.Object) bool {
	var (
		oldDrive *drivecrd.Drive
		newDrive *drivecrd.Drive
		ok       bool
	)
	if oldDrive, ok = old.(*drivecrd.Drive); !ok {
		return true
	}
	if newDrive, ok = new.(*drivecrd.Drive); ok {
		return filter(oldDrive.Spec, newDrive.Spec)
	}
	return true
}
func filter(old api.Drive, new api.Drive) bool {
	return old.GetIsClean() != new.GetIsClean() ||
		old.GetStatus() != new.GetStatus() ||
		old.GetHealth() != new.GetHealth()
}
