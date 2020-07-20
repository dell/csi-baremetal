package lvm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	vccrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

const lvgFinalizer = "dell.emc.csi/lvg-cleanup"

// LVGController is the LVG custom resource Controller for serving VG operations on Node side in Reconcile loop
type LVGController struct {
	k8sClient *k8s.KubeClient

	listBlk lsblk.WrapLsblk
	lvmOps  lvm.WrapLVM
	e       command.CmdExecutor

	node string
	log  *logrus.Entry
}

// NewLVGController is the constructor for LVGController struct
// Receives an instance of base.KubeClient, ID of a node where it works and logrus logger
// Returns an instance of LVGController
func NewLVGController(k8sClient *k8s.KubeClient, nodeID string, log *logrus.Logger) *LVGController {
	e := &command.Executor{}
	e.SetLogger(log)
	return &LVGController{
		k8sClient: k8sClient,
		node:      nodeID,
		log:       log.WithField("component", "LVGController"),
		e:         e,
		lvmOps:    lvm.NewLVM(e, log),
		listBlk:   lsblk.NewLSBLK(log),
	}
}

// Reconcile is the main Reconcile loop of LVGController. This loop handles creation of VG matched to LVG CR on
// LVGController's node if LVG.Spec.Status is Creating. Also this loop handles VG deletion on the node if
// LVG.ObjectMeta.DeletionTimestamp is not zero and VG is not placed on system drive.
// Returns reconcile result as ctrl.Result or error if something went wrong
func (c *LVGController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelFn()

	ll := c.log.WithFields(logrus.Fields{
		"method":  "Reconcile",
		"LVGName": req.Name,
	})

	lvg := &lvgcrd.LVG{}

	if err := c.k8sClient.ReadCR(ctx, req.Name, lvg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ll.Infof("Reconciling LVG: %v", lvg)
	if lvg.ObjectMeta.DeletionTimestamp.IsZero() {
		// append finalizer if LVG doesn't contain it
		if !util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
			lvg.ObjectMeta.Finalizers = append(lvg.ObjectMeta.Finalizers, lvgFinalizer)
			if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
				ll.Errorf("Unable to append finalizer %s to LVG, error: %v.", lvgFinalizer, err)
				return ctrl.Result{Requeue: true}, err
			}
		}
	} else {
		ll.Infof("Removing LVG")
		if util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
			volumes := &vccrd.VolumeList{}

			err := c.k8sClient.ReadList(ctx, volumes)
			if err != nil {
				ll.Errorf("Unable to read volume list: %v", err)
			}

			for _, item := range volumes.Items {
				if item.Spec.Location == lvg.Name {
					lvg.DeletionTimestamp = nil
					err := c.k8sClient.UpdateCR(ctx, lvg)
					if err != nil {
						ll.Errorf("Unable to update %s LVG: %v", lvg.Name, err)
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
			}
			// remove AC that point on that LVG
			c.removeChildAC(lvg.Name)

			if len(lvg.Spec.Locations) == 0 {
				ll.Warn("Location fields is empty")
			} else if lvg.Spec.Locations[0] != base.SystemDriveAsLocation {
				// cleanup LVM artifacts
				if err := c.removeLVGArtifacts(lvg.Name); err != nil {
					ll.Errorf("Unable to cleanup LVM artifacts: %v", err)
					return ctrl.Result{}, err
				}
			}

			lvg.ObjectMeta.Finalizers = util.RemoveString(lvg.ObjectMeta.Finalizers, lvgFinalizer)
			if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
				ll.Errorf("Unable to update LVG's finalizers: %v", err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if lvg.Spec.Status == apiV1.Creating {
		newStatus := apiV1.Created
		var err error
		var locations []string
		if locations, err = c.createSystemLVG(lvg); err != nil {
			ll.Errorf("Unable to create system LVG: %v", err)
			newStatus = apiV1.Failed
		}
		lvg.Spec.Status = newStatus
		lvg.Spec.Locations = locations
		if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
			ll.Errorf("Unable to update LVG status to %s, error: %v.", newStatus, err)
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers LVGController to ControllerManager
func (c *LVGController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lvgcrd.LVG{}).
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

func (c *LVGController) filterCRs(obj runtime.Object) bool {
	if lvg, ok := obj.(*lvgcrd.LVG); ok {
		if lvg.Spec.Node == c.node {
			return true
		}
	}
	return false
}

// createSystemLVG creates LVG in the system and put all drives from lvg.Spec.Location in that LVG
// if some drive doesn't read that drive will not pass in lvg.Location
// return list of drives in LVG that should be used as a locations for this LVG
func (c *LVGController) createSystemLVG(lvg *lvgcrd.LVG) (locations []string, err error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "createSystemLVG",
		"lvgName": lvg.Name,
	})
	ll.Info("Processing ...")

	var deviceFiles = make([]string, 0) // device files of each drive in LVG
	for _, driveUUID := range lvg.Spec.Locations {
		drive := &drivecrd.Drive{}
		if err := c.k8sClient.ReadCR(context.Background(), driveUUID, drive); err != nil {
			// that drive will not be in LVG location
			ll.Errorf("Unable to read drive %s, error: %v", driveUUID, err)
			continue
		}
		// get serial number
		sn := drive.Spec.SerialNumber
		// get device path
		dev, err := c.listBlk.SearchDrivePath(drive)
		if err != nil {
			ll.Error(err)
			continue
		}
		// create PV
		if err := c.lvmOps.PVCreate(dev); err != nil {
			ll.Errorf("Unable to create PV for device %s: %v", dev, err)
			continue
		}
		ll.Infof("PV for device %s (drive serial %s) was created.", dev, sn)
		locations = append(locations, driveUUID)
		deviceFiles = append(deviceFiles, dev)
	}
	if len(deviceFiles) == 0 {
		return locations, errors.New("no one PVs were created")
	}
	// create vg
	if err = c.lvmOps.VGCreate(lvg.Name, deviceFiles...); err != nil {
		ll.Errorf("Unable to create VG: %v", err)
		return locations, err
	}
	return locations, nil
}

// removeLVGArtifacts removes LVG and PVs that doesn't correspond to particular LVG
// when LVG is removed all PVs that were in that LVG becomes orphans
func (c *LVGController) removeLVGArtifacts(lvgName string) error {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "removeLVGArtifacts",
		"lvgName": lvgName,
	})
	ll.Info("Processing ...")

	if c.lvmOps.IsVGContainsLVs(lvgName) {
		ll.Errorf("There are LVs in LVG. Unable to remove it.")
		return fmt.Errorf("there are LVs in LVG %s", lvgName)
	}

	var err error
	if err = c.lvmOps.VGRemove(lvgName); err != nil {
		return fmt.Errorf("unable to remove LVG %s: %v", lvgName, err)
	}
	_ = c.lvmOps.RemoveOrphanPVs() // ignore error since LVG was removed successfully
	return nil
}

// removeChildAC removes AC CR that points (AC.Spec.Location) on LVG CR with name lvgName
func (c *LVGController) removeChildAC(lvgName string) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "removeChildAC",
		"lvgName": lvgName,
	})

	acList := &accrd.AvailableCapacityList{}
	if err := c.k8sClient.ReadList(context.Background(), acList); err != nil {
		ll.Errorf("Unable to list ACs: %v", err)
	}
	for _, ac := range acList.Items {
		if ac.Spec.Location == lvgName {
			acToRemove := ac
			ctxWithID := context.WithValue(context.Background(), k8s.RequestUUID, lvgName)
			if err := c.k8sClient.DeleteCR(ctxWithID, &acToRemove); err != nil {
				ll.Errorf("Unable to remove AC %v, error: %v", acToRemove, err)
			}
		}
	}
}
