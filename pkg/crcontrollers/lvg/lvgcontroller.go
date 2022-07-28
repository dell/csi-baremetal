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
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	vccrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal/pkg/base/util"
	metricsC "github.com/dell/csi-baremetal/pkg/metrics/common"
)

const (
	lvgFinalizer            = "dell.emc.csi/lvg-cleanup"
	lvgDeletionRetryTimeout = 1 * time.Second
)

// Controller is the LogicalVolumeGroup custom resource Controller for serving VG operations on Node side in Reconcile loop
type Controller struct {
	k8sClient *k8s.KubeClient
	crHelper  k8s.CRHelper

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
	e := command.NewExecutor(log)
	return &Controller{
		k8sClient: k8sClient,
		crHelper:  k8s.NewCRHelperImpl(k8sClient, log),
		node:      nodeID,
		log:       log.WithField("component", "Controller"),
		e:         e,
		lvmOps:    lvm.NewLVM(e, log),
		listBlk:   lsblk.NewLSBLK(log),
	}
}

// Reconcile is the main Reconcile loop of Controller. This loop handles creation of VG matched to LogicalVolumeGroup CR on
// Controller's node if LogicalVolumeGroup.Spec.Status is Creating. Also this loop handles VG deletion on the node if
// LogicalVolumeGroup.ObjectMeta.DeletionTimestamp is not zero and VG is not placed on system drive.
// Returns reconcile result as ctrl.Result or error if something went wrong
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	defer metricsC.ReconcileDuration.EvaluateDurationForType("node_lvg_controller")()
	ll := c.log.WithFields(logrus.Fields{
		"method":  "Reconcile",
		"LVGName": req.Name,
	})

	lvg := &lvgcrd.LogicalVolumeGroup{}

	if err := c.k8sClient.ReadCR(ctx, req.Name, "", lvg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ll.Infof("Reconciling LogicalVolumeGroup: %v", lvg)

	switch {
	case !lvg.ObjectMeta.DeletionTimestamp.IsZero():
		ll.Info("Delete LogicalVolumeGroup")
		return c.handleLVGRemoving(lvg)
	case !util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer):
		return c.appendFinalizer(lvg)
	// if lvg.Spec.VolumeRefs == 0 it means that LogicalVolumeGroup just being created
	// for lvg on non-system drive finalizer should be removed during handleLVGRemoving stage
	// here controller removes finalizer for lvg on system drive, for that lvg VolumeRefs != 0
	case !util.HasNameWithPrefix(lvg.Spec.VolumeRefs) && len(lvg.Spec.VolumeRefs) != 0:
		return c.removeFinalizer(lvg)
	}

	// check for LogicalVolumeGroup state
	if lvg.Spec.Status == apiV1.Creating {
		ll.Info("Creating LogicalVolumeGroup")
		return c.handlerLVGCreation(lvg)
	}

	return ctrl.Result{}, nil
}

// appendFinalizer appends finalizer to the LogicalVolumeGroup CR (update CR)
func (c *Controller) appendFinalizer(lvg *lvgcrd.LogicalVolumeGroup) (ctrl.Result, error) {
	if len(lvg.Spec.VolumeRefs) == 0 || util.HasNameWithPrefix(lvg.Spec.VolumeRefs) {
		lvg.ObjectMeta.Finalizers = append(lvg.ObjectMeta.Finalizers, lvgFinalizer)
		if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
			c.log.WithField("LVGName", lvg.Name).
				Errorf("Unable to append finalizer %s to LogicalVolumeGroup: %v.", lvgFinalizer, err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

// removeFinalizer removes finalizer for LogicalVolumeGroup CR (update CR, that is trigger reconcile again)
func (c *Controller) removeFinalizer(lvg *lvgcrd.LogicalVolumeGroup) (ctrl.Result, error) {
	if !util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
		return ctrl.Result{Requeue: true}, nil
	}

	lvg.ObjectMeta.Finalizers = util.RemoveString(lvg.ObjectMeta.Finalizers, lvgFinalizer)
	if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
		c.log.WithField("LVGName", lvg.Name).Errorf("Unable to update LogicalVolumeGroup's finalizers: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// handlerLVGCreation handles LogicalVolumeGroup CR with creating status, create LogicalVolumeGroup on the system drive
// updates corresponding LogicalVolumeGroup CR (set status)
func (c *Controller) handlerLVGCreation(lvg *lvgcrd.LogicalVolumeGroup) (ctrl.Result, error) {
	ll := logrus.WithField("LVGName", lvg.Name)

	newStatus := apiV1.Created
	var err error
	var locations []string
	if locations, err = c.createSystemLVG(lvg); err != nil {
		ll.Errorf("Unable to create system LogicalVolumeGroup: %v", err)
		newStatus = apiV1.Failed
	}
	lvg.Spec.Status = newStatus
	lvg.Spec.Locations = locations
	if err := c.k8sClient.UpdateCR(context.Background(), lvg); err != nil {
		ll.Errorf("Unable to update LogicalVolumeGroup status to %s, error: %v.", newStatus, err)
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// handleLVGRemoving handles removing of LogicalVolumeGroup CR, removes LogicalVolumeGroup from the system and removes finalizers
func (c *Controller) handleLVGRemoving(lvg *lvgcrd.LogicalVolumeGroup) (ctrl.Result, error) {
	ll := logrus.WithField("LVGName", lvg.Name)

	if !util.ContainsString(lvg.ObjectMeta.Finalizers, lvgFinalizer) {
		return ctrl.Result{}, nil
	}

	volumes := &vccrd.VolumeList{}

	// TODO - Remove context.Background() usage - https://github.com/dell/csi-baremetal/issues/703
	err := c.k8sClient.ReadList(context.Background(), volumes)
	if err != nil {
		ll.Errorf("Unable to read volume list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	// If Kubernetes has volumes with location of LogicalVolumeGroup, which is needed to be deleted,
	// we prevent removing, because this LogicalVolumeGroup is still used.
	for _, item := range volumes.Items {
		if item.Spec.Location == lvg.Name && item.DeletionTimestamp.IsZero() {
			ll.Debugf("There are volume %v with LogicalVolumeGroup location, stop LogicalVolumeGroup deletion", item)
			return ctrl.Result{RequeueAfter: lvgDeletionRetryTimeout}, nil
		}
	}

	drivesUUIDs := c.k8sClient.GetSystemDriveUUIDs()
	if !util.ContainsString(drivesUUIDs, lvg.Spec.Locations[0]) {
		// cleanup LVM artifacts
		if err := c.removeLVGArtifacts(lvg.Name); err != nil {
			ll.Errorf("Unable to cleanup LVM artifacts: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	return c.removeFinalizer(lvg)
}

// SetupWithManager registers Controller to ControllerManager
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lvgcrd.LogicalVolumeGroup{}).
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
	if lvg, ok := obj.(*lvgcrd.LogicalVolumeGroup); ok {
		if lvg.Spec.Node == c.node {
			return true
		}
	}
	return false
}

// createSystemLVG creates LogicalVolumeGroup in the system and put all drives from lvg.Spec.Location in that LogicalVolumeGroup
// if some drive doesn't read that drive will not pass in lvg.Location
// return list of drives in LogicalVolumeGroup that should be used as a locations for this LogicalVolumeGroup
func (c *Controller) createSystemLVG(lvg *lvgcrd.LogicalVolumeGroup) (locations []string, err error) {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "createSystemLVG",
		"lvgName": lvg.Name,
	})
	ll.Info("Processing ...")

	var deviceFiles = make([]string, 0) // device files of each drive in LogicalVolumeGroup
	for _, driveUUID := range lvg.Spec.Locations {
		drive := &drivecrd.Drive{}
		// TODO - Remove context.Background() usage - https://github.com/dell/csi-baremetal/issues/703
		if err := c.k8sClient.ReadCR(context.Background(), driveUUID, "", drive); err != nil {
			// that drive will not be in LogicalVolumeGroup location
			ll.Errorf("Unable to read drive %s, error: %v", driveUUID, err)
			continue
		}
		// get serial number
		sn := drive.Spec.SerialNumber
		// get device path
		dev, err := c.listBlk.SearchDrivePath(&drive.Spec)
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

// removeLVGArtifacts removes LogicalVolumeGroup and PVs that doesn't correspond to particular LogicalVolumeGroup
// when LogicalVolumeGroup is removed all PVs that were in that LogicalVolumeGroup becomes orphans
func (c *Controller) removeLVGArtifacts(lvgName string) error {
	ll := c.log.WithFields(logrus.Fields{
		"method":  "removeLVGArtifacts",
		"lvgName": lvgName,
	})
	ll.Info("Processing ...")

	if c.lvmOps.IsVGContainsLVs(lvgName) {
		ll.Errorf("There are LVs in LogicalVolumeGroup. Unable to remove it.")
		return fmt.Errorf("there are LVs in LogicalVolumeGroup %s", lvgName)
	}

	var err error
	if err = c.lvmOps.VGRemove(lvgName); err != nil {
		return fmt.Errorf("unable to remove LogicalVolumeGroup %s: %v", lvgName, err)
	}
	_ = c.lvmOps.RemoveOrphanPVs() // ignore error since LogicalVolumeGroup was removed successfully
	return nil
}

func (c *Controller) setNewVGSize(lvg *lvgcrd.LogicalVolumeGroup, size int64) error {
	if lvg.Annotations == nil {
		lvg.Annotations = make(map[string]string, 1)
	}
	lvg.Annotations[apiV1.LVGFreeSpaceAnnotation] = strconv.FormatInt(size, 10)
	// TODO - Remove context.Background() usage - https://github.com/dell/csi-baremetal/issues/703
	ctx := context.WithValue(context.Background(), base.RequestUUID, lvg.Name)
	if err := c.k8sClient.UpdateCR(ctx, lvg); err != nil {
		return err
	}
	return nil
}
