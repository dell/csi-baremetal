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

package node

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/keymutex"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	ph "github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/common"
	"github.com/dell/csi-baremetal/pkg/eventing"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
	"github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
)

const volumeFinalizer = "dell.emc.csi/volume-cleanup"

// eventRecorder interface for sending events
type eventRecorder interface {
	Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{})
}

// VolumeManager is the struct to perform volume operations on node side with real storage devices
type VolumeManager struct {
	// for interacting with kubernetes objects
	k8sClient *k8s.KubeClient
	// help to read/update particular CR
	crHelper *k8s.CRHelper

	// uses for communicating with hardware manager
	driveMgrClient api.DriveServiceClient
	// holds implementations of Provisioner interface
	provisioners map[p.VolumeType]p.Provisioner

	// uses for operations with partitions
	partOps ph.WrapPartition
	// uses for FS operations such as Mount/Unmount, MkFS and so on
	fsOps utilwrappers.FSOperations
	// uses for LVM operations
	lvmOps lvm.WrapLVM
	// uses for running lsblk util
	listBlk lsblk.WrapLsblk

	// uses for searching suitable Available Capacity
	acProvider common.AvailableCapacityOperations

	// kubernetes node ID
	nodeID string
	// used for discoverLVGOnSystemDisk method to determine if we need to discover LVG in Discover method, default true
	// set false when there is no LVG on system disk or system disk is not SSD
	discoverLvgSSD bool
	// whether VolumeManager was initialized or no, uses for health probes
	initialized bool
	// general logger
	log *logrus.Entry
	// sink where we write events
	recorder eventRecorder
	// reconcile lock
	volMu keymutex.KeyMutex
	// systemDriveUUID represent system drive uuid, used to avoid unnecessary calls to Kubernetes API
	systemDriveUUID []string
}

// driveStates internal struct, holds info about drive updates
// not thread safe
type driveUpdates struct {
	Created    []*drivecrd.Drive
	NotChanged []*drivecrd.Drive
	Updated    []updatedDrive
}

func (du *driveUpdates) AddCreated(drive *drivecrd.Drive) {
	du.Created = append(du.Created, drive)
}

func (du *driveUpdates) AddNotChanged(drive *drivecrd.Drive) {
	du.NotChanged = append(du.NotChanged, drive)
}

func (du *driveUpdates) AddUpdated(previousState, currentState *drivecrd.Drive) {
	du.Updated = append(du.Updated, updatedDrive{
		PreviousState: previousState, CurrentState: currentState})
}

// updatedDrive holds previous and current state for updated drive
type updatedDrive struct {
	PreviousState *drivecrd.Drive
	CurrentState  *drivecrd.Drive
}

const (
	// DiscoverDrivesTimeout is the timeout for Discover method
	DiscoverDrivesTimeout = 300 * time.Second
	// VolumeOperationsTimeout is the timeout for local Volume creation/deletion
	VolumeOperationsTimeout = 900 * time.Second
	// amount of reconcile requests that could be processed simultaneously
	maxConcurrentReconciles = 15
)

// NewVolumeManager is the constructor for VolumeManager struct
// Receives an instance of DriveServiceClient to interact with DriveManager, CmdExecutor to execute linux commands,
// logrus logger, base.KubeClient and ID of a node where VolumeManager works
// Returns an instance of VolumeManager
func NewVolumeManager(
	client api.DriveServiceClient,
	executor command.CmdExecutor,
	logger *logrus.Logger,
	k8sclient *k8s.KubeClient,
	recorder eventRecorder, nodeID string) *VolumeManager {
	vm := &VolumeManager{
		k8sClient:      k8sclient,
		crHelper:       k8s.NewCRHelper(k8sclient, logger),
		driveMgrClient: client,
		acProvider:     common.NewACOperationsImpl(k8sclient, logger),
		provisioners: map[p.VolumeType]p.Provisioner{
			p.DriveBasedVolumeType: p.NewDriveProvisioner(executor, k8sclient, logger),
			p.LVMBasedVolumeType:   p.NewLVMProvisioner(executor, k8sclient, logger),
		},
		fsOps:           utilwrappers.NewFSOperationsImpl(executor, logger),
		lvmOps:          lvm.NewLVM(executor, logger),
		listBlk:         lsblk.NewLSBLK(logger),
		partOps:         ph.NewWrapPartitionImpl(executor, logger),
		nodeID:          nodeID,
		log:             logger.WithField("component", "VolumeManager"),
		recorder:        recorder,
		discoverLvgSSD:  true,
		volMu:           keymutex.NewHashed(0),
		systemDriveUUID: []string{base.SystemDriveAsLocation},
	}
	return vm
}

// SetProvisioners sets provisioners for current VolumeManager instance
// uses for UTs and Sanity tests purposes
func (m *VolumeManager) SetProvisioners(provs map[p.VolumeType]p.Provisioner) {
	m.provisioners = provs
}

// Reconcile is the main Reconcile loop of VolumeManager. This loop handles creation of volumes matched to Volume CR on
// VolumeManagers's node if Volume.Spec.CSIStatus is Creating. Also this loop handles volume deletion on the node if
// Volume.Spec.CSIStatus is Removing.
// Returns reconcile result as ctrl.Result or error if something went wrong
func (m *VolumeManager) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	m.volMu.LockKey(req.Name)
	ll := m.log.WithFields(logrus.Fields{
		"method":   "Reconcile",
		"volumeID": req.Name,
	})
	defer func() {
		err := m.volMu.UnlockKey(req.Name)
		if err != nil {
			ll.Warnf("Unlocking  volume with error %s", err)
		}
	}()
	ctx, cancelFn := context.WithTimeout(
		context.WithValue(context.Background(), k8s.RequestUUID, req.Name),
		VolumeOperationsTimeout)
	defer cancelFn()

	volume := &volumecrd.Volume{}

	err := m.k8sClient.ReadCR(ctx, req.Name, volume)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if volume.DeletionTimestamp.IsZero() {
		if !util.ContainsString(volume.ObjectMeta.Finalizers, volumeFinalizer) {
			ll.Debug("Appending finalizer for volume")
			volume.ObjectMeta.Finalizers = append(volume.ObjectMeta.Finalizers, volumeFinalizer)
			if err := m.k8sClient.UpdateCR(ctx, volume); err != nil {
				ll.Errorf("Unable to append finalizer %s to Volume, error: %v.", volumeFinalizer, err)
				return ctrl.Result{Requeue: true}, err
			}
		}
	} else {
		switch volume.Spec.CSIStatus {
		case apiV1.Created:
			volume.Spec.CSIStatus = apiV1.Removing
			ll.Debug("Change volume status from Created to Removing")
		case apiV1.Removing:
		case apiV1.Removed:
			if util.ContainsString(volume.ObjectMeta.Finalizers, volumeFinalizer) {
				volume.ObjectMeta.Finalizers = util.RemoveString(volume.ObjectMeta.Finalizers, volumeFinalizer)
				ll.Debug("Remove finalizer for volume")
				if err := m.k8sClient.UpdateCR(ctx, volume); err != nil {
					ll.Errorf("Unable to update Volume's finalizers")
				}
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		default:
			ll.Warnf("Volume wasn't deleted, because it has CSI status %s", volume.Spec.CSIStatus)
			return ctrl.Result{}, nil
		}
	}
	ll.Infof("Processing for status %s", volume.Spec.CSIStatus)
	switch volume.Spec.CSIStatus {
	case apiV1.Creating:
		if util.IsStorageClassLVG(volume.Spec.StorageClass) {
			return m.handleCreatingVolumeInLVG(ctx, volume)
		}
		return m.prepareVolume(ctx, volume)
	case apiV1.Removing:
		return m.handleRemovingStatus(ctx, volume)
	default:
		return ctrl.Result{}, nil
	}
}

// handleCreatingVolumeInLVG handles volume CR that has storage class related to LVG and CSIStatus creating
// check whether underlying LVG ready or not, add volume to LVG volumeRefs (if needed) and create real storage based on volume
// uses as a step for Reconcile for Volume CR
func (m *VolumeManager) handleCreatingVolumeInLVG(ctx context.Context, volume *volumecrd.Volume) (ctrl.Result, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "handleCreatingVolumeInLVG",
		"volumeID": volume.Spec.Id,
	})

	var (
		lvg = &lvgcrd.LVG{}
		err error
	)

	if err = m.k8sClient.ReadCR(ctx, volume.Spec.Location, lvg); err != nil {
		ll.Errorf("Unable to read underlying LVG %s: %v", volume.Spec.Location, err)
		if k8sError.IsNotFound(err) {
			volume.Spec.CSIStatus = apiV1.Failed
			err = m.k8sClient.UpdateCR(ctx, volume)
			if err == nil {
				return ctrl.Result{}, nil // no need to retry
			}
			ll.Errorf("Unable to update volume CR and set status to failed: %v", err)
		}
		// retry because of LVG wasn't read or Volume status wasn't updated
		return ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}, err
	}

	switch lvg.Spec.Status {
	case apiV1.Creating:
		ll.Debugf("Underlying LVG %s is still being created", lvg.Name)
		return ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}, nil
	case apiV1.Failed:
		ll.Errorf("Underlying LVG %s has reached failed status. Unable to create volume on failed lvg.", lvg.Name)
		volume.Spec.CSIStatus = apiV1.Failed
		if err = m.k8sClient.UpdateCR(ctx, volume); err != nil {
			ll.Errorf("Unable to update volume CR and set status to failed: %v", err)
			// retry because of volume status wasn't updated
			return ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}, err
		}
		return ctrl.Result{}, nil // no need to retry
	case apiV1.Created:
		// add volume ID to LVG.Spec.VolumeRefs
		if !util.ContainsString(lvg.Spec.VolumeRefs, volume.Spec.Id) {
			lvg.Spec.VolumeRefs = append(lvg.Spec.VolumeRefs, volume.Spec.Id)
			if err = m.k8sClient.UpdateCR(ctx, lvg); err != nil {
				ll.Errorf("Unable to add Volume ID to LVG %s volume refs: %v", lvg.Name, err)
				return ctrl.Result{Requeue: true}, err
			}
		}
		return m.prepareVolume(ctx, volume)
	default:
		ll.Warnf("Unable to recognize LVG status. LVG - %v", lvg)
		return ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}, nil
	}
}

// prepareVolume prepares real storage based on provided volume and update corresponding volume CR's CSIStatus
// uses as a step for Reconcile for Volume CR
func (m *VolumeManager) prepareVolume(ctx context.Context, volume *volumecrd.Volume) (ctrl.Result, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "prepareVolume",
		"volumeID": volume.Spec.Id,
	})

	newStatus := apiV1.Created

	err := m.getProvisionerForVolume(&volume.Spec).PrepareVolume(volume.Spec)
	if err != nil {
		ll.Errorf("Unable to create volume size of %d bytes: %v. Set volume status to Failed", volume.Spec.Size, err)
		newStatus = apiV1.Failed
	}

	volume.Spec.CSIStatus = newStatus
	if updateErr := m.k8sClient.UpdateCRWithAttempts(ctx, volume, 5); updateErr != nil {
		ll.Errorf("Unable to update volume status to %s: %v", newStatus, updateErr)
		return ctrl.Result{Requeue: true}, updateErr
	}

	return ctrl.Result{}, err
}

// handleRemovingStatus handles volume CR with removing CSIStatus - removed real storage (partition/lv) and
// update corresponding volume CR's CSIStatus
// uses as a step for Reconcile for Volume CR
func (m *VolumeManager) handleRemovingStatus(ctx context.Context, volume *volumecrd.Volume) (ctrl.Result, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "handleRemovingStatus",
		"volumeID": volume.Name,
	})

	var (
		err       error
		newStatus string
	)
	if err = m.getProvisionerForVolume(&volume.Spec).ReleaseVolume(volume.Spec); err != nil {
		ll.Errorf("Failed to remove volume - %s. Error: %v. Set status to Failed", volume.Spec.Id, err)
		newStatus = apiV1.Failed
	} else {
		ll.Infof("Volume - %s was successfully removed. Set status to Removed", volume.Spec.Id)
		newStatus = apiV1.Removed
	}
	volume.Spec.CSIStatus = newStatus
	if updateErr := m.k8sClient.UpdateCRWithAttempts(ctx, volume, 10); updateErr != nil {
		ll.Error("Unable to set new status for volume")
		return ctrl.Result{Requeue: true}, updateErr
	}
	return ctrl.Result{}, err
}

// SetupWithManager registers VolumeManager to ControllerManager
func (m *VolumeManager) SetupWithManager(mgr ctrl.Manager) error {
	m.log.WithField("method", "SetupWithManager").
		Infof("MaxConcurrentReconciles - %d", maxConcurrentReconciles)
	return ctrl.NewControllerManagedBy(mgr).
		For(&volumecrd.Volume{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return m.isCorrespondedToNodePredicate(e.Object)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return m.isCorrespondedToNodePredicate(e.Object)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return m.isCorrespondedToNodePredicate(e.ObjectOld)
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return m.isCorrespondedToNodePredicate(e.Object)
			},
		}).
		Complete(m)
}

// isCorrespondedToNodePredicate checks is a provided obj is aVolume CR object
// and that volume's node is and current manager node
func (m *VolumeManager) isCorrespondedToNodePredicate(obj runtime.Object) bool {
	if vol, ok := obj.(*volumecrd.Volume); ok {
		if vol.Spec.NodeId == m.nodeID {
			return true
		}
	}

	return false
}

// Discover inspects actual drives structs from DriveManager and create volume object if partition exist on some of them
// (in case of VolumeManager restart). Updates Drives CRs based on gathered from DriveManager information.
// Also this method creates AC CRs. Performs at some intervals in a goroutine
// Returns error if something went wrong during discovering
func (m *VolumeManager) Discover() error {
	ctx, cancelFn := context.WithTimeout(context.Background(), DiscoverDrivesTimeout)
	defer cancelFn()
	drivesResponse, err := m.driveMgrClient.GetDrivesList(ctx, &api.DrivesRequest{NodeId: m.nodeID})
	if err != nil {
		return err
	}

	updates, err := m.updateDrivesCRs(ctx, drivesResponse.Disks)
	if err != nil {
		return fmt.Errorf("updateDrivesCRs return error: %v", err)
	}
	m.handleDriveUpdates(ctx, updates)

	freeDrives, err := m.drivesAreNotUsed()

	if err != nil {
		return fmt.Errorf("drivesAreNotUsed return error: %v", err)
	}

	if m.discoverLvgSSD {
		if err = m.discoverLVGOnSystemDrive(); err != nil {
			m.log.WithField("method", "Discover").
				Errorf("unable to inspect system LVG: %v", err)
		}
	}
	if err = m.discoverVolumeCRs(freeDrives); err != nil {
		return fmt.Errorf("discoverVolumeCRs return error: %v", err)
	}

	if err = m.discoverAvailableCapacity(ctx, freeDrives); err != nil {
		return fmt.Errorf("discoverAvailableCapacity return error: %v", err)
	}

	m.initialized = true
	return nil
}

// updateDrivesCRs updates Drives CRs based on provided list of Drives.
// Receives golang context and slice of discovered api.Drive structs usually got from DriveManager
// returns struct with information about drives updates
func (m *VolumeManager) updateDrivesCRs(ctx context.Context, drivesFromMgr []*api.Drive) (*driveUpdates, error) {
	ll := m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "updateDrivesCRs",
	})
	ll.Debugf("Processing")

	var (
		driveCRs       []drivecrd.Drive
		firstIteration bool
		err            error
	)

	if driveCRs, err = m.crHelper.GetDriveCRs(m.nodeID); err != nil {
		return nil, err
	}
	firstIteration = len(driveCRs) == 0

	var updates = new(driveUpdates)
	// Try to find not existing CR for discovered drives
	for _, drivePtr := range drivesFromMgr {
		exist := false
		for index, driveCR := range driveCRs {
			driveCR := driveCR
			// If drive CR already exist, try to update, if drive was changed
			if m.drivesAreTheSame(drivePtr, &driveCR.Spec) {
				exist = true
				if driveCR.Equals(drivePtr) {
					updates.AddNotChanged(&driveCR)
				} else {
					previousState := driveCR.DeepCopy()
					drivePtr.UUID = driveCR.Spec.UUID
					toUpdate := driveCR
					toUpdate.Spec = *drivePtr
					if err := m.k8sClient.UpdateCR(ctx, &toUpdate); err != nil {
						ll.Errorf("Failed to update drive CR (health/status) %v, error %v", toUpdate, err)
						updates.AddNotChanged(previousState)
					} else {
						driveCRs[index] = toUpdate
						updates.AddUpdated(previousState, &toUpdate)
					}
				}
				break
			}
		}
		if !exist {
			// Drive CR is not exist, try to create it
			toCreateSpec := *drivePtr
			toCreateSpec.NodeId = m.nodeID
			toCreateSpec.UUID = uuid.New().String()
			isSystem, err := m.isDriveSystem(drivePtr.Path)
			if err != nil {
				ll.Errorf("Failed to determine if drive %v is system, error: %v", drivePtr, err)
			}
			if isSystem {
				m.systemDriveUUID = append(m.systemDriveUUID, toCreateSpec.UUID)
			}
			toCreateSpec.IsSystem = isSystem
			driveCR := m.k8sClient.ConstructDriveCR(toCreateSpec.UUID, toCreateSpec)
			if err := m.k8sClient.CreateCR(ctx, driveCR.Name, driveCR); err != nil {
				ll.Errorf("Failed to create drive CR %v, error: %v", driveCR, err)
			}
			updates.AddCreated(driveCR)
			driveCRs = append(driveCRs, *driveCR)
		}
	}

	// that means that it is a first round and drives are discovered first time
	if firstIteration {
		return updates, nil
	}

	for _, d := range driveCRs {
		wasDiscovered := false
		for _, drive := range drivesFromMgr {
			if m.drivesAreTheSame(&d.Spec, drive) {
				wasDiscovered = true
				break
			}
		}

		if !wasDiscovered {
			if m.isDriveInLVG(d.Spec) {
				continue
			}

			ll.Warnf("Set status OFFLINE for drive %v", d.Spec)
			previousState := d.DeepCopy()
			toUpdate := d
			toUpdate.Spec.Status = apiV1.DriveStatusOffline
			toUpdate.Spec.Health = apiV1.HealthUnknown
			if err := m.k8sClient.UpdateCR(ctx, &toUpdate); err != nil {
				ll.Errorf("Failed to update drive CR %v, error %v", toUpdate, err)
				updates.AddNotChanged(previousState)
			} else {
				updates.AddUpdated(previousState, &toUpdate)
			}
		}
	}
	return updates, nil
}

func (m *VolumeManager) handleDriveUpdates(ctx context.Context, updates *driveUpdates) {
	for _, updDrive := range updates.Updated {
		m.handleDriveStatusChange(ctx, &updDrive.CurrentState.Spec)
	}
	m.createEventsForDriveUpdates(updates)
}

// isDriveInLVG check whether drive is a part of some LVG or no
func (m *VolumeManager) isDriveInLVG(d api.Drive) bool {
	lvgs := m.crHelper.GetLVGCRs(m.nodeID)
	for _, lvg := range lvgs {
		if util.ContainsString(lvg.Spec.Locations, d.UUID) {
			return true
		}
	}
	return false
}

// drivesAreNotUsed search drives in Drives CRs that isn't have any Volume CR or LVG CR
// Returns slice of pointers on drivecrd.Drive structs
func (m *VolumeManager) drivesAreNotUsed() ([]*drivecrd.Drive, error) {
	var (
		drives    = make([]*drivecrd.Drive, 0)
		volumeCRs []volumecrd.Volume
		driveCRs  []drivecrd.Drive
		lvgCRs    []lvgcrd.LVG
		err       error
	)

	if volumeCRs, err = m.crHelper.GetVolumeCRs(m.nodeID); err != nil {
		return nil, err
	}
	if driveCRs, err = m.crHelper.GetDriveCRs(m.nodeID); err != nil {
		return nil, err
	}
	lvgCRs = m.crHelper.GetLVGCRs(m.nodeID)

	var locations = make(map[string]struct{}, len(volumeCRs))
	for _, v := range volumeCRs {
		locations[v.Spec.Location] = struct{}{}
	}
	for _, lvg := range lvgCRs {
		if len(lvg.Spec.Locations) > 0 && util.IsDriveSystem(lvg.Spec.Locations[0], m.systemDriveUUID) {
			continue
		}
		for _, location := range lvg.Spec.Locations {
			locations[location] = struct{}{}
		}
	}

	for _, d := range driveCRs {
		if _, isUsed := locations[d.Spec.UUID]; !isUsed {
			dInst := d
			drives = append(drives, &dInst)
		}
	}
	return drives, nil
}

// discoverVolumeCRs matches system block devices with freeDrives
// searches drives in freeDrives that are not have volume and if there are some partitions on them - try to read
// partition uuid and create volume CR object
func (m *VolumeManager) discoverVolumeCRs(freeDrives []*drivecrd.Drive) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "discoverVolumeCRs",
	})

	// explore each drive from freeDrives
	blockDevices, err := m.listBlk.GetBlockDevices("")
	if err != nil {
		return fmt.Errorf("unable to inspect system block devices via lsblk, error: %v", err)
	}

	bdevMap := make(map[string]lsblk.BlockDevice, len(blockDevices))
	for _, bdev := range blockDevices {
		bdevMap[bdev.Serial] = bdev
	}

	for _, d := range freeDrives {
		if d.Spec.IsSystem {
			if m.isDriveInLVG(d.Spec) {
				continue
			}
		}
		bdev, ok := bdevMap[d.Spec.SerialNumber]
		if !ok {
			ll.Errorf("For drive %v there is no corresponding block device.", *d)
			continue
		}
		if len(bdev.Children) > 0 {
			var (
				partUUID string
				size     int64
			)

			if bdev.Children[0].Size != "" {
				size, err = strconv.ParseInt(bdev.Size, 10, 64)
				if err != nil {
					ll.Warnf("Unable parse string %s to int, for device %s: %v. Volume CR will be created with 0 size",
						bdev.Size, bdev.Name, err)
				}
			}

			partUUID = bdev.Children[0].PartUUID
			if partUUID == "" {
				partUUID = uuid.New().String() // just generate random and exclude drive
				ll.Warnf("There is no part UUID for partition from device %v, UUID has been generated %s", bdev, partUUID)
			}

			volumeCR := m.k8sClient.ConstructVolumeCR(partUUID, api.Volume{
				NodeId:       m.nodeID,
				Id:           partUUID,
				Size:         size,
				Location:     d.Spec.UUID,
				LocationType: apiV1.LocationTypeDrive,
				Mode:         apiV1.ModeFS,
				Type:         bdev.FSType,
				Health:       d.Spec.Health,
				CSIStatus:    "",
			})

			ctxWithID := context.WithValue(context.Background(), k8s.RequestUUID, volumeCR.Name)
			if err = m.k8sClient.CreateCR(ctxWithID, partUUID, volumeCR); err != nil {
				ll.Errorf("Unable to create volume CR %s: %v", partUUID, err)
			}
		}
	}
	return nil
}

// DiscoverAvailableCapacity inspect current available capacity on nodes and fill AC CRs. This method manages only
// hardware available capacity such as HDD or SSD. If drive is healthy and online and also it is not used in LVGs
// and it doesn't contain volume then this drive is in AvailableCapacity CRs.
// Returns error if at least one drive from cache was handled badly
func (m *VolumeManager) discoverAvailableCapacity(ctx context.Context, freeDrives []*drivecrd.Drive) error {
	ll := m.log.WithField("method", "discoverAvailableCapacity")

	var (
		err        error
		wasError   = false
		acList     = &accrd.AvailableCapacityList{}
		volumeList = &volumecrd.VolumeList{}
	)

	if err = m.k8sClient.ReadList(ctx, acList); err != nil {
		return fmt.Errorf("unable to read AC list: %v", err)
	}
	if err = m.k8sClient.ReadList(ctx, volumeList); err != nil {
		return fmt.Errorf("unable to read Volume list: %v", err)
	}
	var (
		// key - ac.Spec.Location that is Drive.Spec.UUID
		acsLocations = make(map[string]*accrd.AvailableCapacity, len(acList.Items))
		// key - volume.Spec.Location that is Drive.Spec.UUID or LVG.Spec.Name (don't need to use info about LVG here)
		volumeLocations = make(map[string]struct{})
	)
	for _, ac := range acList.Items {
		ac := ac
		acsLocations[ac.Spec.Location] = &ac
	}
	for _, v := range volumeList.Items {
		volumeLocations[v.Spec.Location] = struct{}{}
	}

	for _, drive := range freeDrives {
		if drive.Spec.Health != apiV1.HealthGood || drive.Spec.Status != apiV1.DriveStatusOnline {
			// AC that points on such drive was removed before (if they had existed)
			continue
		}
		// check whether appropriate AC exists or not
		if _, acExist := acsLocations[drive.Spec.UUID]; acExist {
			// check whether there is Volume CR that points on same drive
			if _, volumeExist := volumeLocations[drive.Spec.UUID]; volumeExist {
				ll.Warnf("There is Volume CR that points on same drive %s as AC %s",
					drive.Name, acsLocations[drive.Spec.UUID].Name)
				if err = m.k8sClient.DeleteCR(ctx, acsLocations[drive.Spec.UUID]); err != nil {
					ll.Errorf("Unable to delete AC CR %s: %v. Inconsistent ACs", acsLocations[drive.Spec.UUID].Name, err)
				}
			}
			continue
		}
		// create AC based on drive
		capacity := &api.AvailableCapacity{
			Size:         drive.Spec.Size,
			Location:     drive.Spec.UUID,
			StorageClass: util.ConvertDriveTypeToStorageClass(drive.Spec.Type),
			NodeId:       m.nodeID,
		}

		if drive.Spec.IsSystem {
			if m.isDriveInLVG(drive.Spec) {
				capacity.Size = 0
			}
		}
		name := uuid.New().String()

		newAC := m.k8sClient.ConstructACCR(name, *capacity)
		if err := m.k8sClient.CreateCR(context.WithValue(ctx, k8s.RequestUUID, name),
			name, newAC); err != nil {
			ll.Errorf("Error during CreateAvailableCapacity request to k8s: %v, error: %v",
				capacity, err)
			wasError = true
		}
	}

	if wasError {
		return errors.New("not all available capacity were created")
	}

	return nil
}

// discoverLVGOnSystemDrive discovers LVG configuration on system SSD drive and creates LVG CR and AC CR,
// return nil in case of success. If system drive is not SSD or LVG CR that points in system VG is exists - return nil.
// If system VG free space is less then threshold - AC CR will not be created but LVG will.
// Returns error in case of error on any step
func (m *VolumeManager) discoverLVGOnSystemDrive() error {
	ll := m.log.WithField("method", "discoverLVGOnSystemDrive")

	var (
		lvgList = lvgcrd.LVGList{}
		errTmpl = "unable to inspect system LVM, error: %v"
		err     error
	)

	// at first check whether LVG on system drive exists or no
	if err = m.k8sClient.ReadList(context.Background(), &lvgList); err != nil {
		return fmt.Errorf(errTmpl, err)
	}
	for _, lvg := range lvgList.Items {
		if lvg.Spec.Node == m.nodeID && len(lvg.Spec.Locations) > 0 && util.IsDriveSystem(lvg.Spec.Locations[0], m.systemDriveUUID) {
			var vgFreeSpace int64
			if vgFreeSpace, err = m.lvmOps.GetVgFreeSpace(lvg.Spec.Name); err != nil {
				return err
			}
			ll.Infof("LVG CR that points on system VG is exists: %v", lvg)
			return m.createACIfFreeSpace(lvg.Name, apiV1.StorageClassSystemLVG, vgFreeSpace)
		}
	}

	var (
		rootMountPoint, vgName string
		vgFreeSpace            int64
	)

	if rootMountPoint, err = m.fsOps.FindMountPoint(base.KubeletRootPath); err != nil {
		return fmt.Errorf(errTmpl, err)
	}

	// from container we expect here name like "VG_NAME[/var/lib/kubelet/pods]"
	rootMountPoint = strings.Split(rootMountPoint, "[")[0]

	devices, err := m.listBlk.GetBlockDevices(rootMountPoint)
	if err != nil {
		return fmt.Errorf(errTmpl, err)
	}

	if devices[0].Rota != base.NonRotationalNum {
		m.discoverLvgSSD = false
		ll.Infof("System disk is not SSD. LVG will not be created base on it.")
		return nil
	}

	lvgExists, err := m.lvmOps.IsLVGExists(rootMountPoint)

	if err != nil {
		return fmt.Errorf(errTmpl, err)
	}

	if !lvgExists {
		m.discoverLvgSSD = false
		ll.Infof("System disk is SSD. but it doesn't have LVG.")
		return nil
	}

	if vgName, err = m.lvmOps.FindVgNameByLvName(rootMountPoint); err != nil {
		return fmt.Errorf(errTmpl, err)
	}
	if vgFreeSpace, err = m.lvmOps.GetVgFreeSpace(vgName); err != nil {
		return fmt.Errorf(errTmpl, err)
	}
	lvs, err := m.lvmOps.GetLVsInVG(vgName)
	if err != nil {
		return fmt.Errorf("unable to determine LVs in system VG %s: %v", vgName, err)
	}

	systemDriveUUID := base.SystemDriveAsLocation
	for _, driveUUID := range m.systemDriveUUID {
		drive := m.crHelper.GetDriveCRByUUID(driveUUID)
		if drive != nil && drive.Spec.SerialNumber == devices[0].Serial {
			systemDriveUUID = drive.Spec.UUID
		}
	}

	var (
		vgCRName = uuid.New().String()
		vg       = api.LogicalVolumeGroup{
			Name:       vgName,
			Node:       m.nodeID,
			Locations:  []string{systemDriveUUID},
			Size:       vgFreeSpace,
			Status:     apiV1.Created,
			VolumeRefs: lvs,
		}
		vgCR = m.k8sClient.ConstructLVGCR(vgCRName, vg)
		ctx  = context.WithValue(context.Background(), k8s.RequestUUID, vg.Name)
	)
	if err = m.k8sClient.CreateCR(ctx, vg.Name, vgCR); err != nil {
		return fmt.Errorf("unable to create LVG CR %v, error: %v", vgCR, err)
	}
	return m.createACIfFreeSpace(vgCRName, apiV1.StorageClassSystemLVG, vgFreeSpace)
}

// getProvisionerForVolume returns appropriate Provisioner implementation for volume
func (m *VolumeManager) getProvisionerForVolume(vol *api.Volume) p.Provisioner {
	if util.IsStorageClassLVG(vol.StorageClass) {
		return m.provisioners[p.LVMBasedVolumeType]
	}

	return m.provisioners[p.DriveBasedVolumeType]
}

// handleDriveStatusChange removes AC that is based on unhealthy drive, returns AC if drive returned to healthy state,
// mark volumes of the unhealthy drive as unhealthy.
// Receives golang context and api.Drive that should be handled
func (m *VolumeManager) handleDriveStatusChange(ctx context.Context, drive *api.Drive) {
	ll := m.log.WithFields(logrus.Fields{
		"method":  "handleDriveStatusChange",
		"driveID": drive.UUID,
	})

	ll.Infof("The new drive status from DriveMgr is %s", drive.Health)

	// Handle resources without LVG
	// Remove AC based on disk with health BAD, SUSPECT, UNKNOWN
	if drive.Health != apiV1.HealthGood || drive.Status == apiV1.DriveStatusOffline {
		ac := m.crHelper.GetACByLocation(drive.UUID)
		if ac != nil {
			ll.Infof("Removing AC %s based on unhealthy location %s", ac.Name, ac.Spec.Location)
			if err := m.k8sClient.DeleteCR(ctx, ac); err != nil {
				ll.Errorf("Failed to delete unhealthy available capacity CR: %v", err)
			}
		}
	}

	// Set disk's health status to volume CR
	vol := m.crHelper.GetVolumeByLocation(drive.UUID)
	if vol != nil {
		ll.Infof("Setting updated status %s to volume %s", drive.Health, vol.Name)
		// save previous health state
		prevHealthState := vol.Spec.Health
		vol.Spec.Health = drive.Health
		if err := m.k8sClient.UpdateCR(ctx, vol); err != nil {
			ll.Errorf("Failed to update volume CR's %s health status: %v", vol.Name, err)
		}
		if vol.Spec.Health == apiV1.HealthBad {
			m.recorder.Eventf(vol, eventing.WarningType, eventing.VolumeBadHealth,
				"Volume health transitioned from %s to %s. Inherited from %s drive on %s)",
				prevHealthState, vol.Spec.Health, drive.Health, drive.NodeId)
		}
	}

	// Handle resources with LVG
	// This is not work for the current moment because HAL doesn't monitor disks with LVM
	// TODO: Handle disk health which are used by LVGs - https://github.com/dell/csi-baremetal/issues/88
}

// drivesAreTheSame check whether two drive represent same node drive or no
// method is rely on that each drive could be uniquely identified by it VID/PID/Serial Number
func (m *VolumeManager) drivesAreTheSame(drive1, drive2 *api.Drive) bool {
	return drive1.SerialNumber == drive2.SerialNumber &&
		drive1.VID == drive2.VID &&
		drive1.PID == drive2.PID
}

// createACIfFreeSpace create AC CR if there are free spcae on drive
// Receive context, drive location, storage class, size of available capacity
// Return error
func (m *VolumeManager) createACIfFreeSpace(location string, sc string, size int64) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "createACIfFreeSpace",
	})
	if size == 0 {
		size++ // if size is 0 it field will not display for CR
	}
	acCR := m.crHelper.GetACByLocation(location)
	if acCR != nil {
		return nil
	}
	if size > common.AcSizeMinThresholdBytes {
		acName := uuid.New().String()
		acCR = m.k8sClient.ConstructACCR(acName, api.AvailableCapacity{
			Location:     location,
			NodeId:       m.nodeID,
			StorageClass: sc,
			Size:         size,
		})
		if err := m.k8sClient.CreateCR(context.Background(), acName, acCR); err != nil {
			return fmt.Errorf("unable to create AC based on system LVG, error: %v", err)
		}
		ll.Infof("Created AC %v for lvg %s", acCR, location)
		return nil
	}
	ll.Infof("There is no available space on %s", location)
	return nil
}

// createEventsForDriveUpdates create required events for drive state change
func (m *VolumeManager) createEventsForDriveUpdates(updates *driveUpdates) {
	for _, createdDrive := range updates.Created {
		m.sendEventForDrive(createdDrive, eventing.InfoType, eventing.DriveDiscovered,
			"New drive discovered SN: %s, Node: %s.",
			createdDrive.Spec.SerialNumber, createdDrive.Spec.NodeId)
		m.createEventForDriveHealthChange(
			createdDrive, apiV1.HealthUnknown, createdDrive.Spec.Health)
	}
	for _, updDrive := range updates.Updated {
		if updDrive.CurrentState.Spec.Health != updDrive.PreviousState.Spec.Health {
			m.createEventForDriveHealthChange(
				updDrive.CurrentState, updDrive.PreviousState.Spec.Health, updDrive.CurrentState.Spec.Health)
		}
		if updDrive.CurrentState.Spec.Status != updDrive.PreviousState.Spec.Status {
			m.createEventForDriveStatusChange(
				updDrive.CurrentState, updDrive.PreviousState.Spec.Status, updDrive.CurrentState.Spec.Status)
		}
	}
}

func (m *VolumeManager) createEventForDriveHealthChange(
	drive *drivecrd.Drive, prevHealth, currentHealth string) {
	healthMsgTemplate := "Drive health is: %s, previous state: %s."
	eventType := eventing.WarningType
	var reason string
	switch currentHealth {
	case apiV1.HealthGood:
		eventType = eventing.InfoType
		reason = eventing.DriveHealthGood
	case apiV1.HealthBad:
		eventType = eventing.ErrorType
		reason = eventing.DriveHealthFailure
	case apiV1.HealthSuspect:
		reason = eventing.DriveHealthSuspect
	case apiV1.HealthUnknown:
		reason = eventing.DriveHealthUnknown
	default:
		return
	}
	m.sendEventForDrive(drive, eventType, reason,
		healthMsgTemplate, currentHealth, prevHealth)
}

func (m *VolumeManager) createEventForDriveStatusChange(
	drive *drivecrd.Drive, prevStatus, currentStatus string) {
	statusMsgTemplate := "Drive status is: %s, previous status: %s."
	eventType := eventing.InfoType
	var reason string
	switch currentStatus {
	case apiV1.DriveStatusOnline:
		reason = eventing.DriveStatusOnline
	case apiV1.DriveStatusOffline:
		eventType = eventing.ErrorType
		reason = eventing.DriveStatusOffline
	default:
		return
	}
	m.sendEventForDrive(drive, eventType, reason,
		statusMsgTemplate, currentStatus, prevStatus)
}

func (m *VolumeManager) sendEventForDrive(drive *drivecrd.Drive, eventtype, reason, messageFmt string,
	args ...interface{}) {
	messageFmt += prepareDriveDescription(drive)
	m.recorder.Eventf(drive, eventtype, reason, messageFmt, args...)
}

func prepareDriveDescription(drive *drivecrd.Drive) string {
	return fmt.Sprintf(" Drive Details: SN='%s', Node='%s',"+
		" Type='%s', Model='%s %s',"+
		" Size='%d', Firmware='%s'",
		drive.Spec.SerialNumber, drive.Spec.NodeId, drive.Spec.Type,
		drive.Spec.VID, drive.Spec.PID, drive.Spec.Size, drive.Spec.Firmware)
}

// isDriveSystem check whether drive is system
// Parameters: path string - drive path
// Returns true if drive is system, false in opposite; error
func (m *VolumeManager) isDriveSystem(path string) (bool, error) {
	devices, err := m.listBlk.GetBlockDevices(path)
	if err != nil {
		return false, err
	}
	return m.isRootMountpoint(devices), nil
}

// isRootMountpoint check whether devices has root mountpoint
// Parameters: BlockDevice from lsblk output
// Returns true if device has root mountpoint, false in opposite
func (m *VolumeManager) isRootMountpoint(dev []lsblk.BlockDevice) bool {
	for _, device := range dev {
		if strings.TrimSpace(device.MountPoint) == base.KubeletRootPath {
			return true
		}
		if m.isRootMountpoint(device.Children) {
			return true
		}
	}
	return false
}
