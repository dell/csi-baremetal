/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lvg

import (
	"context"
	"errors"
	"fmt"

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
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

const lvgFinalizer = "dell.emc.csi/lvg-cleanup"

// Controller is the LVG custom resource Controller for serving VG operations on Node side in Reconcile loop
type Controller struct {
	k8sClient *k8s.KubeClient
	crHelper  *k8s.CRHelper

	listBlk lsblk.WrapLsblk
	lvmOps  lvm.WrapLVM
	e       command.CmdExecutor

	node string
	log  *logrus.Entry
}

// NewController is the constructor for Controller struct
// Receives an instance of base.KubeClient, ID of a node where it works and logrus logger
// Returns an instance of Controller
func NewController(k8sClient *k8s.KubeClient, nodeID string, log *logrus.Logger) *Controller {
	e := &command.Executor{}
	e.SetLogger(log)
	return &Controller{
		k8sClient: k8sClient,
		crHelper:  k8s.NewCRHelper(k8sClient, log),
		node:      nodeID,
		log:       log.WithField("component", "Controller"),
		e:         e,
		lvmOps:    lvm.NewLVM(e, log),
		listBlk:   lsblk.NewLSBLK(log),
	}
}

// Reconcile is the main Reconcile loop of Controller. This loop handles creation of VG matched to LVG CR on
// Controller's node if LVG.Spec.Status is Creating. Also this loop handles VG deletion on the node if
// LVG.ObjectMeta.DeletionTimestamp is not zero and VG is not placed on system drive.
// Returns reconcile result as ctrl.Result or error if something went wrong
func (c *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "Reconcile",
		"LVGName": req.Name,
	})

	lvg := &lvgcrd.LVG{}

	if err := c.k8sClient.ReadCR(context.Background(), req.Name, lvg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ll.Infof("Reconciling LVG: %v", lvg)

	switch {
	case !lvg.ObjectMeta.DeletionTimestamp.IsZero():
		ll.Info("Delete LVG")
		return c.handleLVGRemoving(lvg)
	case !util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer):
		return c.appendFinalizer(lvg)
	// if lvg.Spec.VolumeRefs == 0 it means that LVG just being created
	// for lvg on non-system drive finalizer should be removed during handleLVGRemoving stage
	// here controller removes finalizer for lvg on system drive, for that lvg VolumeRefs != 0
	case !util.HasNameWithPrefix(lvg.Spec.VolumeRefs) && len(lvg.Spec.VolumeRefs) != 0:
		return c.removeFinalizer(lvg)
	}

	// check for LVG state
	switch lvg.Spec.Status {
	case apiV1.Creating:
		ll.Info("Creating LVG")
		return c.handlerLVGCreation(lvg)
	case apiV1.Failed:
		return ctrl.Result{}, c.resetACSizeOfLVG(lvg.Name)
	}

	if lvg.Spec.Health != apiV1.HealthGood {
		return ctrl.Result{}, c.resetACSizeOfLVG(lvg.Name)
	}

	return ctrl.Result{}, nil
}

// appendFinalizer appends finalizer to the LVG CR (update CR)
func (c *Controller) appendFinalizer(lvg *lvgcrd.LVG) (ctrl.Result, error) {
	if len(lvg.Spec.VolumeRefs) == 0 || util.HasNameWithPrefix(lvg.Spec.VolumeRefs) {
		lvg.ObjectMeta.Finalizers = append(lvg.ObjectMeta.Finalizers, lvgFinalizer)
		if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
			c.log.WithField("LVGName", lvg.Name).
				Errorf("Unable to append finalizer %s to LVG: %v.", lvgFinalizer, err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

// removeFinalizer removes finalizer for LVG CR (update CR, that is trigger reconcile again)
func (c *Controller) removeFinalizer(lvg *lvgcrd.LVG) (ctrl.Result, error) {
	if !util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
		return ctrl.Result{Requeue: true}, nil
	}

	lvg.ObjectMeta.Finalizers = util.RemoveString(lvg.ObjectMeta.Finalizers, lvgFinalizer)
	if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
		c.log.WithField("LVGName", lvg.Name).Errorf("Unable to update LVG's finalizers: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// handlerLVGCreation handles LVG CR with creating status, create LVG on the system drive
// updates corresponding LVG CR (set status)
func (c *Controller) handlerLVGCreation(lvg *lvgcrd.LVG) (ctrl.Result, error) {
	ll := logrus.WithField("LVGName", lvg.Name)

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
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// handleLVGRemoving handles removing of LVG CR, removes LVG from the system and removes finalizers
func (c *Controller) handleLVGRemoving(lvg *lvgcrd.LVG) (ctrl.Result, error) {
	ll := logrus.WithField("LVGName", lvg.Name)

	if !util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
		return ctrl.Result{}, nil
	}

	volumes := &vccrd.VolumeList{}

	err := c.k8sClient.ReadList(context.Background(), volumes)
	if err != nil {
		ll.Errorf("Unable to read volume list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}
	// If Kubernetes has volumes with location of LVG, which is needed to be deleted,
	// we prevent removing, because this LVG is still used.
	for _, item := range volumes.Items {
		if item.Spec.Location == lvg.Name && item.DeletionTimestamp.IsZero() {
			ll.Debugf("There are volume %v with LVG location, stop LVG deletion", item)
			return ctrl.Result{}, nil
		}
	}
	// update AC size that point on that LVG
	c.increaseACSize(lvg.Spec.Locations[0], lvg.Spec.Size)

	drivesUUIDs := append(c.k8sClient.GetSystemDriveUUIDs(), base.SystemDriveAsLocation)
	if !util.ContainsString(drivesUUIDs, lvg.Spec.Locations[0]) {
		// cleanup LVM artifacts
		if err := c.removeLVGArtifacts(lvg.Name); err != nil {
			ll.Errorf("Unable to cleanup LVM artifacts: %v", err)
			return ctrl.Result{}, err
		}
	}

	return c.removeFinalizer(lvg)
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
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

func (c *Controller) filterCRs(obj runtime.Object) bool {
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
func (c *Controller) createSystemLVG(lvg *lvgcrd.LVG) (locations []string, err error) {
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
func (c *Controller) removeLVGArtifacts(lvgName string) error {
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

// increaseACSize updates size of AC related to drive
func (c *Controller) increaseACSize(driveID string, size int64) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "increaseACSize",
		"driveID": driveID,
	})

	// read all ACs
	acList := &accrd.AvailableCapacityList{}
	if err := c.k8sClient.ReadList(context.Background(), acList); err != nil {
		ll.Errorf("Unable to list ACs: %v", err)
		return
	}

	// search for AC and update size
	for _, ac := range acList.Items {
		if ac.Spec.Location == driveID {
			ac.Spec.Size += size
			ctxWithID := context.WithValue(context.Background(), base.RequestUUID, driveID)
			// nolint: scopelint
			if err := c.k8sClient.UpdateCR(ctxWithID, &ac); err != nil {
				ll.Errorf("Unable to update size of AC %v, error: %v", ac, err)
			}
			return
		}
	}

	ll.Errorf("Corresponding AC for drive ID %s not found", driveID)
}

// resetACSize sets size of corresponding AC to 0 to avoid further allocations
func (c *Controller) resetACSizeOfLVG(lvgName string) error {
	var (
		err error
		ac  *accrd.AvailableCapacity
	)
	// read AC
	if ac, err = c.crHelper.GetACByLocation(lvgName); err == nil {
		// update if not null already
		if ac.Spec.Size != 0 {
			ac.Spec.Size = 0
			if err := c.k8sClient.UpdateCR(context.Background(), ac); err != nil {
				c.log.Errorf("Unable to set AC CR %s size to 0, error: %v.", ac.Name, err)
				return err
			}
		}
		return nil
	}

	if err == errTypes.ErrorNotFound {
		// non re-triable error
		c.log.Errorf("AC CR for LVG %s not found", lvgName)
		return nil
	}

	return err
}
