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
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/keymutex"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	crevent "sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/datadiscover"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/datadiscover/types"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lsblk"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/lvm"
	ph "github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/common"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/metrics"
	metricsC "github.com/dell/csi-baremetal/pkg/metrics/common"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
	"github.com/dell/csi-baremetal/pkg/node/provisioners/utilwrappers"
	wbtconf "github.com/dell/csi-baremetal/pkg/node/wbt/common"
	wbtops "github.com/dell/csi-baremetal/pkg/node/wbt/operations"
)

const (
	volumeFinalizer = "dell.emc.csi/volume-cleanup"

	deleteVolumeFailedMsg = "Failed to remove volume %s with error: %s"

	fakeAttachAnnotation = "pv.attach.kubernetes.io/ignore-if-inaccessible"
	fakeAttachAllowKey   = "yes"

	// Annotation key for health overriding
	// Discover function replaces drive health with passed value if the annotation is set
	driveHealthOverrideAnnotation = "health"
)

// eventRecorder interface for sending events
type eventRecorder interface {
	Eventf(object runtime.Object, event *eventing.EventDescription, messageFmt string, args ...interface{})
}

// VolumeManager is the struct to perform volume operations on node side with real storage devices
type VolumeManager struct {
	// for interacting with kubernetes objects
	k8sClient *k8s.KubeClient
	// cache for kubernetes resources
	k8sCache k8s.CRReader
	// help to read/update particular CR
	crHelper *k8s.CRHelper
	// CRHelper instance which reads from cache
	cachedCrHelper *k8s.CRHelper

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
	// uses for disable/enable WBT
	wbtOps    wbtops.WrapWbt
	wbtConfig *wbtconf.WbtConfig

	// uses for searching suitable Available Capacity
	acProvider common.AvailableCapacityOperations

	// kubernetes node ID
	nodeID string
	// kubernetes node name
	nodeName string
	// used for discoverLVGOnSystemDisk method to determine if we need to discover LogicalVolumeGroup in Discover method, default true
	// set false when there is no LogicalVolumeGroup on system disk or system disk is not SSD
	discoverSystemLVG bool
	// whether VolumeManager was initialized or no, uses for health probes
	initialized bool
	// general logger
	log *logrus.Entry
	// sink where we write events
	recorder eventRecorder
	// reconcile lock
	volMu keymutex.KeyMutex
	// systemDrivesUUIDs represent system drive uuids, used to avoid unnecessary calls to Kubernetes API.
	// We use slice in case of RAID and multiple system disks
	systemDrivesUUIDs []string

	// metrics
	metricDriveMgrDuration metrics.Statistic
	metricDriveMgrCount    prometheus.Gauge

	// discover data on drive
	dataDiscover types.WrapDataDiscover
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
	k8sClient *k8s.KubeClient,
	k8sCache k8s.CRReader,
	recorder eventRecorder,
	nodeID string,
	nodeName string) *VolumeManager {
	driveMgrDuration := metrics.NewMetrics(prometheus.HistogramOpts{
		Name:    "discovery_duration_seconds",
		Help:    "duration of the discovery method for the drive manager",
		Buckets: metrics.ExtendedDefBuckets,
	})
	driveMgrCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "discovery_drive_count",
		Help: "last drive count discovered",
	})
	for _, c := range []prometheus.Collector{driveMgrDuration.Collect(), driveMgrCount} {
		if err := prometheus.Register(c); err != nil {
			logger.WithField("component", "NewVolumeManager").
				Errorf("Failed to register metric: %v", err)
		}
	}

	partImpl := ph.NewWrapPartitionImpl(executor, logger)
	lvmOps := lvm.NewLVM(executor, logger)
	fsOps := utilwrappers.NewFSOperationsImpl(executor, logger)
	wbtOps := wbtops.NewWbt(executor)

	vm := &VolumeManager{
		k8sClient:      k8sClient,
		k8sCache:       k8sCache,
		crHelper:       k8s.NewCRHelper(k8sClient, logger),
		cachedCrHelper: k8s.NewCRHelper(k8sClient, logger).SetReader(k8sCache),
		driveMgrClient: client,
		acProvider:     common.NewACOperationsImpl(k8sClient, logger),
		provisioners: map[p.VolumeType]p.Provisioner{
			p.DriveBasedVolumeType: p.NewDriveProvisioner(executor, k8sClient, logger),
			p.LVMBasedVolumeType:   p.NewLVMProvisioner(executor, k8sClient, logger),
		},
		fsOps:                  fsOps,
		lvmOps:                 lvmOps,
		listBlk:                lsblk.NewLSBLK(logger),
		partOps:                partImpl,
		wbtOps:                 wbtOps,
		nodeID:                 nodeID,
		nodeName:               nodeName,
		log:                    logger.WithField("component", "VolumeManager"),
		recorder:               recorder,
		discoverSystemLVG:      true,
		volMu:                  keymutex.NewHashed(0),
		systemDrivesUUIDs:      make([]string, 0),
		metricDriveMgrDuration: driveMgrDuration,
		metricDriveMgrCount:    driveMgrCount,
		dataDiscover:           datadiscover.NewDataDiscover(fsOps, partImpl, lvmOps),
	}
	return vm
}

// SetProvisioners sets provisioners for current VolumeManager instance
// uses for UTs and Sanity tests purposes
func (m *VolumeManager) SetProvisioners(provs map[p.VolumeType]p.Provisioner) {
	m.provisioners = provs
}

// SetListBlk sets listBlk for current VolumeManager instance
// uses in Sanity testing
func (m *VolumeManager) SetListBlk(listBlk lsblk.WrapLsblk) {
	m.listBlk = listBlk
}

// Reconcile is the main Reconcile loop of VolumeManager. This loop handles creation of volumes matched to Volume CR on
// VolumeManagers's node if Volume.Spec.CSIStatus is Creating. Also this loop handles volume deletion on the node if
// Volume.Spec.CSIStatus is Removing.
// Returns reconcile result as ctrl.DiscoverResult or error if something went wrong
func (m *VolumeManager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	defer metricsC.ReconcileDuration.EvaluateDurationForType("node_volume_controller")()
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
		context.WithValue(ctx, base.RequestUUID, req.Name),
		VolumeOperationsTimeout)
	defer cancelFn()

	volume := &volumecrd.Volume{}

	err := m.k8sClient.ReadCR(ctx, req.Name, req.Namespace, volume)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if volume.DeletionTimestamp.IsZero() {
		if !util.ContainsString(volume.ObjectMeta.Finalizers, volumeFinalizer) && volume.Spec.CSIStatus != apiV1.Empty {
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
			// we need to update annotation on related drive CRD
			// todo can we do polling instead?
			ll.Infof("Volume %s is removed. Updating related", volume.Name)
			// drive must be present in the system
			drive, _ := m.crHelper.GetDriveCRByVolume(volume)
			if drive != nil {
				annotations := drive.GetAnnotations()
				delete(annotations, fmt.Sprintf("%s/%s", apiV1.DriveAnnotationVolumeStatusPrefix, volume.Name))
				drive.SetAnnotations(annotations)
				if err := m.k8sClient.UpdateCR(ctx, drive); err != nil {
					ll.Errorf("Unable to update Drive annotations")
				}
			} else {
				ll.Errorf("Unable to obtain drive for volume %s", volume.Name)
			}

			// remove finalizer
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
	case apiV1.Resizing:
		return m.handleExpandingStatus(ctx, volume)
	}

	if volume.Spec.Usage == apiV1.VolumeUsageReleasing {
		// check for release annotation
		releaseStatus := volume.Annotations[apiV1.VolumeAnnotationRelease]
		// when done move to RELEASED state
		switch releaseStatus {
		case apiV1.VolumeAnnotationReleaseDone:
			ll.Infof("Volume %s is released", volume.Name)
			return m.updateVolumeAndDriveUsageStatus(ctx, volume, apiV1.VolumeUsageReleased, apiV1.DriveUsageReleasing)
		case apiV1.VolumeAnnotationReleaseFailed:
			errMsg := "Volume releasing is failed"
			errorDesc, ok := volume.Annotations[apiV1.VolumeAnnotationReleaseStatus]
			if ok && errorDesc != "" {
				errMsg += fmt.Sprintf(" err: %s", errorDesc)
			}
			ll.Errorf(errMsg)
			return m.updateVolumeAndDriveUsageStatus(ctx, volume, apiV1.VolumeUsageFailed, apiV1.DriveUsageFailed)
		}
	}
	return ctrl.Result{}, nil
}

func (m *VolumeManager) updateVolumeAndDriveUsageStatus(ctx context.Context, volume *volumecrd.Volume,
	volumeStatus, driveStatus string) (ctrl.Result, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method":       "updateVolumeAndDriveUsageStatus",
		"volumeID":     volume.Name,
		"volumeStatus": volumeStatus,
		"driveStatus":  driveStatus,
	})
	volume.Spec.Usage = volumeStatus
	if err := m.k8sClient.UpdateCR(ctx, volume); err != nil {
		ll.Errorf("Unable to change volume %s usage status to %s, error: %v.",
			volume.Name, volume.Spec.Usage, err)
		return ctrl.Result{Requeue: true}, err
	}
	drive, err := m.crHelper.GetDriveCRByVolume(volume)
	if err != nil {
		ll.Errorf("Unable to read drive CR, error: %v", err)
		return ctrl.Result{Requeue: true}, err
	}
	// TODO add annotations for additional statuses?
	if volumeStatus == apiV1.VolumeUsageReleased {
		m.addVolumeStatusAnnotation(drive, volume.Name, apiV1.VolumeUsageReleased)
	}
	if drive != nil {
		if driveStatus == apiV1.DriveUsageFailed {
			eventMsg := fmt.Sprintf("Failed to release volume(s), %s", drive.GetDriveDescription())
			m.recorder.Eventf(drive, eventing.DriveRemovalFailed, eventMsg)
		}
		drive.Spec.Usage = driveStatus
		if err := m.k8sClient.UpdateCR(ctx, drive); err != nil {
			ll.Errorf("Unable to change drive %s usage status to %s, error: %v.",
				drive.Name, drive.Spec.Usage, err)
			return ctrl.Result{Requeue: true}, err
		}
	}
	return ctrl.Result{}, nil
}

// handleCreatingVolumeInLVG handles volume CR that has storage class related to LogicalVolumeGroup and CSIStatus creating
// check whether underlying LogicalVolumeGroup ready or not, add volume to LogicalVolumeGroup volumeRefs (if needed) and create real storage based on volume
// uses as a step for Reconcile for Volume CR
func (m *VolumeManager) handleCreatingVolumeInLVG(ctx context.Context, volume *volumecrd.Volume) (ctrl.Result, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "handleCreatingVolumeInLVG",
		"volumeID": volume.Spec.Id,
	})

	var (
		lvg = &lvgcrd.LogicalVolumeGroup{}
		err error
	)

	if err = m.k8sClient.ReadCR(ctx, volume.Spec.Location, "", lvg); err != nil {
		ll.Errorf("Unable to read underlying LogicalVolumeGroup %s: %v", volume.Spec.Location, err)
		if k8sError.IsNotFound(err) {
			volume.Spec.CSIStatus = apiV1.Failed
			err = m.k8sClient.UpdateCR(ctx, volume)
			if err == nil {
				return ctrl.Result{}, nil // no need to retry
			}
			ll.Errorf("Unable to update volume CR and set status to failed: %v", err)
		}
		// retry because of LogicalVolumeGroup wasn't read or Volume status wasn't updated
		return ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}, err
	}

	switch lvg.Spec.Status {
	case apiV1.Creating:
		ll.Debugf("Underlying LogicalVolumeGroup %s is still being created", lvg.Name)
		return ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}, nil
	case apiV1.Failed:
		ll.Errorf("Underlying LogicalVolumeGroup %s has reached failed status. Unable to create volume on failed lvg.", lvg.Name)
		volume.Spec.CSIStatus = apiV1.Failed
		if err = m.k8sClient.UpdateCR(ctx, volume); err != nil {
			ll.Errorf("Unable to update volume CR and set status to failed: %v", err)
			// retry because of volume status wasn't updated
			return ctrl.Result{Requeue: true, RequeueAfter: base.DefaultRequeueForVolume}, err
		}
		return ctrl.Result{}, nil // no need to retry
	case apiV1.Created:
		// add volume ID to LogicalVolumeGroup.Spec.VolumeRefs
		if !util.ContainsString(lvg.Spec.VolumeRefs, volume.Spec.Id) {
			lvg.Spec.VolumeRefs = append(lvg.Spec.VolumeRefs, volume.Spec.Id)
			if err = m.k8sClient.UpdateCR(ctx, lvg); err != nil {
				ll.Errorf("Unable to add Volume ID to LogicalVolumeGroup %s volume refs: %v", lvg.Name, err)
				return ctrl.Result{Requeue: true}, err
			}
		}
		return m.prepareVolume(ctx, volume)
	default:
		ll.Warnf("Unable to recognize LogicalVolumeGroup status. LogicalVolumeGroup - %v", lvg)
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

	err := m.getProvisionerForVolume(&volume.Spec).PrepareVolume(&volume.Spec)
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

	newStatus, err := m.performVolumeRemoving(ctx, volume)
	if err != nil && newStatus == "" {
		return ctrl.Result{Requeue: true}, err
	}

	volume.Spec.CSIStatus = newStatus
	if updateErr := m.k8sClient.UpdateCRWithAttempts(ctx, volume, 10); updateErr != nil {
		ll.Error("Unable to set new status for volume")
		return ctrl.Result{Requeue: true}, updateErr
	}
	return ctrl.Result{}, err
}

func (m *VolumeManager) performVolumeRemoving(ctx context.Context, volume *volumecrd.Volume) (string, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "performVolumeRemoving",
		"volumeID": volume.Name,
	})

	if volume.Spec.GetOperationalStatus() == apiV1.OperationalStatusMissing {
		ll.Warnf("Volume - %s is MISSING. Unable to perform deletion. Set status to Removed", volume.Spec.Id)
		return apiV1.Removed, nil
	}

	// read Drive CR based on Volume.Location (vol.Location == Drive.UUID == Drive.Name)
	drive, err := m.crHelper.GetDriveCRByVolume(volume)
	if err != nil {
		updateErr := fmt.Errorf("failed to read drive CR with name %s, error %w", volume.Spec.Location, err)
		ll.Error(updateErr)
		return "", updateErr
	}
	ll.Debugf("Got drive %+v", drive)

	if err := m.getProvisionerForVolume(&volume.Spec).ReleaseVolume(&volume.Spec, &drive.Spec); err != nil {
		ll.Errorf("Failed to remove volume - %s. Error: %v. Set status to Failed", volume.Spec.Id, err)
		drive.Spec.Usage = apiV1.DriveUsageFailed
		if err := m.k8sClient.UpdateCRWithAttempts(ctx, drive, 5); err != nil {
			ll.Errorf("Unable to change drive %s usage status to %s, error: %v.",
				drive.Name, drive.Spec.Usage, err)
			return "", err
		}
		m.sendEventForDrive(drive, eventing.DriveRemovalFailed, deleteVolumeFailedMsg, volume.Name, err)
		return apiV1.Failed, err
	}

	ll.Infof("Volume - %s was successfully removed. Set status to Removed", volume.Spec.Id)
	return apiV1.Removed, nil
}

// SetupWithManager registers VolumeManager to ControllerManager
func (m *VolumeManager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&volumecrd.Volume{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e crevent.CreateEvent) bool {
				return m.isCorrespondedToNodePredicate(e.Object)
			},
			DeleteFunc: func(e crevent.DeleteEvent) bool {
				return m.isCorrespondedToNodePredicate(e.Object)
			},
			UpdateFunc: func(e crevent.UpdateEvent) bool {
				return m.isCorrespondedToNodePredicate(e.ObjectOld)
			},
			GenericFunc: func(e crevent.GenericEvent) bool {
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

	driveMgrDoneFunc := m.metricDriveMgrDuration.EvaluateDuration(prometheus.Labels{})
	drivesResponse, err := m.driveMgrClient.GetDrivesList(ctx, &api.DrivesRequest{NodeId: m.nodeID})
	driveMgrDoneFunc()
	if err != nil {
		return err
	}
	m.metricDriveMgrCount.Set(float64(len(drivesResponse.Disks)))

	updates, err := m.updateDrivesCRs(ctx, drivesResponse.Disks)
	if err != nil {
		return fmt.Errorf("updateDrivesCRs return error: %v", err)
	}
	m.handleDriveUpdates(ctx, updates)

	if m.discoverSystemLVG {
		if err = m.discoverLVGOnSystemDrive(); err != nil {
			m.log.WithField("method", "Discover").
				Errorf("unable to inspect system LogicalVolumeGroup: %v", err)
		}
	}

	if err = m.discoverDataOnDrives(); err != nil {
		return fmt.Errorf("discoverDataOnDrives return error: %v", err)
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

	if driveCRs, err = m.cachedCrHelper.GetDriveCRs(m.nodeID); err != nil {
		return nil, err
	}
	firstIteration = len(driveCRs) == 0

	var updates = new(driveUpdates)
	var searchSystemDrives = len(m.systemDrivesUUIDs) == 0
	// Try to find not existing CR for discovered drives
	for _, drivePtr := range drivesFromMgr {
		exist := false
		for index, driveCR := range driveCRs {
			driveCR := driveCR
			// If drive CR already exist, try to update, if drive was changed
			if m.drivesAreTheSame(drivePtr, &driveCR.Spec) {
				exist = true
				if searchSystemDrives && driveCR.Spec.IsSystem {
					m.systemDrivesUUIDs = append(m.systemDrivesUUIDs, driveCR.Spec.UUID)
				}
				if value, ok := driveCR.GetAnnotations()[driveHealthOverrideAnnotation]; ok {
					m.overrideDriveHealth(drivePtr, value, driveCR.Name)
				}
				if driveCR.Equals(drivePtr) {
					updates.AddNotChanged(&driveCR)
				} else {
					previousState := driveCR.DeepCopy()
					// copy fields which aren't reported by drive manager
					drivePtr.UUID = driveCR.Spec.UUID
					drivePtr.Usage = driveCR.Spec.Usage
					drivePtr.IsSystem = driveCR.Spec.IsSystem
					drivePtr.IsClean = driveCR.Spec.IsClean

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
		if !exist && drivePtr.SerialNumber != "" {
			// don't create CR for OFFLINE drives
			// todo do we need to deprecate status field reported by drive manager?
			// todo https://github.com/dell/csi-baremetal/issues/202
			if drivePtr.Status == apiV1.DriveStatusOffline {
				continue
			}
			// drive CR does not exist, try to create it
			toCreateSpec := *drivePtr
			toCreateSpec.NodeId = m.nodeID
			toCreateSpec.UUID = uuid.New().String()
			// TODO: what operational status should be if drivemgr reported drive with not a good health
			toCreateSpec.Usage = apiV1.DriveUsageInUse
			toCreateSpec.IsClean = true
			isSystem, err := m.isDriveSystem(drivePtr.Path)
			if err != nil {
				ll.Errorf("Failed to determine if drive %v is system, error: %v", drivePtr, err)
			}
			if isSystem {
				toCreateSpec.IsClean = false
				m.systemDrivesUUIDs = append(m.systemDrivesUUIDs, toCreateSpec.UUID)
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
			ll.Warnf("Set status %s for drive %v", apiV1.DriveStatusOffline, d.Spec)
			previousState := d.DeepCopy()
			toUpdate := d
			// TODO: which operational status should be in case when there is drive CR that doesn't have corresponding drive from drivemgr response
			toUpdate.Spec.Status = apiV1.DriveStatusOffline
			if value, ok := d.GetAnnotations()[driveHealthOverrideAnnotation]; ok {
				m.overrideDriveHealth(&toUpdate.Spec, value, d.Name)
			} else {
				toUpdate.Spec.Health = apiV1.HealthUnknown
			}
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
		m.handleDriveStatusChange(ctx, updDrive)
	}
	m.createEventsForDriveUpdates(updates)
}

// isDriveInLVG check whether drive is a part of some LogicalVolumeGroup or no
func (m *VolumeManager) isDriveInLVG(d api.Drive) bool {
	lvgs, err := m.cachedCrHelper.GetLVGCRs(m.nodeID)
	if err != nil {
		m.log.WithFields(logrus.Fields{
			"method":    "isDriveInLVG",
			"driveUUID": d.UUID,
		}).Errorf("Unable to read LogicalVolumeGroup CRs list: %v. Consider that drive isn't in LogicalVolumeGroup", err)
		return false
	}

	for _, lvg := range lvgs {
		if util.ContainsString(lvg.Spec.Locations, d.UUID) {
			return true
		}
	}
	return false
}

// discoverVolumeCRs matches system block devices with driveCRs
// searches drives in driveCRs that are not have volume and if there are some partitions on them - try to read
// partition uuid and create volume CR object
func (m *VolumeManager) discoverDataOnDrives() error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "discoverVolumeCRs",
	})

	driveCRs, err := m.cachedCrHelper.GetDriveCRs(m.nodeID)
	if err != nil {
		return err
	}

	volumeCRs, err := m.cachedCrHelper.GetVolumeCRs(m.nodeID)
	if err != nil {
		return err
	}

	locations := make(map[string]struct{}, len(volumeCRs))
	for _, v := range volumeCRs {
		locations[v.Spec.Location] = struct{}{}
	}

	for _, drive := range driveCRs {
		var discoverResult *types.DiscoverResult
		drive := drive
		if drive.Spec.IsSystem && m.isDriveInLVG(drive.Spec) {
			continue
		}
		if _, ok := locations[drive.Spec.UUID]; ok {
			if drive.Spec.IsClean {
				m.changeDriveIsCleanField(&drive, false)
			}
			continue
		}
		if discoverResult, err = m.dataDiscover.DiscoverData(drive.Spec.Path, drive.Spec.SerialNumber); err != nil {
			ll.Errorf("Failed to discover data on drive %s, err: %v", drive.Spec.SerialNumber, err)
			continue
		}
		if discoverResult.HasData {
			if drive.Spec.IsClean {
				ll.Info(discoverResult.Message)
				m.sendEventForDrive(&drive, eventing.DriveHasData, discoverResult.Message)
				m.changeDriveIsCleanField(&drive, false)
			}
			continue
		}
		ll.Info(discoverResult.Message)
		if !drive.Spec.IsClean {
			m.sendEventForDrive(&drive, eventing.DriveClean, discoverResult.Message)
			m.changeDriveIsCleanField(&drive, true)
		}
	}
	return nil
}

// discoverLVGOnSystemDrive discovers LogicalVolumeGroup configuration on system SSD drive and creates LogicalVolumeGroup CR and AC CR,
// return nil in case of success. If system drive is not SSD or LogicalVolumeGroup CR that points in system VG is exists - return nil.
// If system VG free space is less then threshold - AC CR will not be created but LogicalVolumeGroup will.
// Returns error in case of error on any step
func (m *VolumeManager) discoverLVGOnSystemDrive() error {
	ll := m.log.WithField("method", "discoverLVGOnSystemDrive")

	if len(m.systemDrivesUUIDs) == 0 {
		// system drive is not detected by drive manager
		// this is not an issue but might be configuration choice
		ll.Warningf("System drive is not detected by drive manager")
		// skipping LVM check for system drive
		m.discoverSystemLVG = false
		return nil
	}

	var (
		vgFreeSpace int64
		err         error
	)

	// 1. check whether LogicalVolumeGroup CR that holds info about LogicalVolumeGroup configuration on the system drive exists or not
	lvgs, err := m.cachedCrHelper.GetLVGCRs(m.nodeID)
	if err != nil {
		return err
	}
	for _, lvg := range lvgs {
		lvg := lvg
		if lvg.Spec.Node == m.nodeID && len(lvg.Spec.Locations) > 0 && util.ContainsString(m.systemDrivesUUIDs, lvg.Spec.Locations[0]) {
			if vgFreeSpace, err = m.lvmOps.GetVgFreeSpace(lvg.Spec.Name); err != nil {
				return err
			}
			ll.Infof("LogicalVolumeGroup CR that points on system VG is exists: %v", lvg)
			m.updateLVGAnnotation(&lvg, vgFreeSpace)
			ctx := context.WithValue(context.Background(), base.RequestUUID, lvg.Name)
			if err := m.k8sClient.UpdateCR(ctx, &lvg); err != nil {
				return err
			}
			return nil
		}
	}

	// 2. check whether there is LogicalVolumeGroup configuration on the system drive or not
	var driveCR = new(drivecrd.Drive)
	// TODO: handle situation when there is more then one system drive
	if err = m.k8sCache.ReadCR(context.Background(), m.systemDrivesUUIDs[0], "", driveCR); err != nil {
		return err
	}

	pvs, err := m.lvmOps.GetAllPVs()
	if err != nil {
		return fmt.Errorf("unable to list PVs on the system: %v", err)
	}

	var systemPVName string
	for _, pv := range pvs {
		// LogicalVolumeGroup could be configured on partition on the system drive, handle this case
		matched, _ := regexp.Match(fmt.Sprintf("^%s\\d*$", driveCR.Spec.Path), []byte(pv))
		if matched {
			systemPVName = pv
			break
		}
	}

	if systemPVName == "" {
		ll.Info("There is no LVM configuration on the system drive")
		m.discoverSystemLVG = false
		return nil
	}

	// 4. search VG info
	var vgName string
	vgName, err = m.lvmOps.GetVGNameByPVName(systemPVName)
	if err != nil {
		return fmt.Errorf("unable to detect system VG name: %v", err)
	}

	if vgFreeSpace, err = m.lvmOps.GetVgFreeSpace(vgName); err != nil {
		return fmt.Errorf("unable to determine VG %s free space: %v", vgName, err)
	}
	lvs, err := m.lvmOps.GetLVsInVG(vgName)
	if err != nil {
		return fmt.Errorf("unable to determine LVs in system VG %s: %v", vgName, err)
	}

	// 5. create LogicalVolumeGroup CR
	var (
		vgCRName = uuid.New().String()
		vg       = api.LogicalVolumeGroup{
			Name:       vgName,
			Node:       m.nodeID,
			Locations:  m.systemDrivesUUIDs,
			Size:       vgFreeSpace,
			Status:     apiV1.Created,
			VolumeRefs: lvs,
			Health:     apiV1.HealthGood,
		}
		vgCR = m.k8sClient.ConstructLVGCR(vgCRName, vg)
		ctx  = context.WithValue(context.Background(), base.RequestUUID, vg.Name)
	)
	m.updateLVGAnnotation(vgCR, vgFreeSpace)
	if err = m.k8sClient.CreateCR(ctx, vg.Name, vgCR); err != nil {
		return fmt.Errorf("unable to create LogicalVolumeGroup CR %v, error: %v", vgCR, err)
	}
	driveCR.Spec.IsClean = false
	if err = m.k8sClient.UpdateCR(ctx, driveCR); err != nil {
		return fmt.Errorf("unable to update Drive CR %v, error: %v", driveCR, err)
	}
	return nil
}

func (m *VolumeManager) updateLVGAnnotation(lvg *lvgcrd.LogicalVolumeGroup, vgFreeSpace int64) {
	if lvg.Annotations == nil {
		lvg.Annotations = make(map[string]string, 1)
	}
	if _, ok := lvg.Annotations[apiV1.LVGFreeSpaceAnnotation]; !ok {
		lvg.Annotations[apiV1.LVGFreeSpaceAnnotation] = strconv.FormatInt(vgFreeSpace, 10)
	}
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
// TODO the method is called every rime after drive go to OFFLINE - https://github.com/dell/csi-baremetal/issues/550
func (m *VolumeManager) handleDriveStatusChange(ctx context.Context, drive updatedDrive) {
	cur := drive.CurrentState.Spec
	prev := drive.PreviousState.Spec
	ll := m.log.WithFields(logrus.Fields{
		"method":  "handleDriveStatusChange",
		"driveID": cur.UUID,
	})

	ll.Infof("The new cur status from DriveMgr is %s", cur.Health)

	// Handle resources without LogicalVolumeGroup
	// Remove AC based on disk with health BAD, SUSPECT, UNKNOWN
	lvg, err := m.cachedCrHelper.GetLVGByDrive(ctx, cur.UUID)
	if lvg != nil {
		name := lvg.Name
		// TODO handle situation when LVG health is changing from Bad/Suspect to Good https://github.com/dell/csi-baremetal/issues/385
		lvg.Spec.Health = cur.Health
		if err := m.k8sClient.UpdateCR(ctx, lvg); err != nil {
			ll.Errorf("Failed to update lvg CR's %s health status: %v", name, err)
		}

		// check for missing disk and re-activate volume group
		if prev.Status == apiV1.DriveStatusOffline && cur.Status == apiV1.DriveStatusOnline {
			m.reactivateVG(lvg)
			m.checkVGErrors(lvg, cur.Path)
		}
	} else {
		errMsg := "Failed get LogicalVolumeGroup CR"
		if err != nil {
			errMsg = fmt.Sprintf(errMsg+" error: %v", err)
		}
		ll.Errorf(errMsg)
	}
	// Set disk's health status to volume CR
	volumes, _ := m.cachedCrHelper.GetVolumesByLocation(ctx, cur.UUID)
	for _, vol := range volumes {
		// skip if health is not changed
		if vol.Spec.Health == cur.Health {
			ll.Infof("Volume %s status is already %s", vol.Name, cur.Health)
			continue
		}
		ll.Infof("Setting updated status %s to volume %s", cur.Health, vol.Name)
		// save previous health state
		prevHealthState := vol.Spec.Health
		vol.Spec.Health = cur.Health
		// initiate volume release
		// TODO need to check for specific annotation instead
		if vol.Spec.Health == apiV1.HealthBad || vol.Spec.Health == apiV1.HealthSuspect {
			if vol.Spec.Usage == apiV1.VolumeUsageInUse {
				vol.Spec.Usage = apiV1.VolumeUsageReleasing
			}
		}

		if err := m.k8sClient.UpdateCR(ctx, vol); err != nil {
			ll.Errorf("Failed to update volume CR's %s health status: %v", vol.Name, err)
		}

		if vol.Spec.Health == apiV1.HealthBad {
			m.recorder.Eventf(vol, eventing.VolumeBadHealth,
				"Volume health transitioned from %s to %s. Inherited from %s drive on %s)",
				prevHealthState, vol.Spec.Health, cur.Health, cur.NodeId)
		}
	}
	// Handle resources with LogicalVolumeGroup
	// This is not work for the current moment because HAL doesn't monitor disks with LVM
	// TODO: Handle disk health which are used by LVGs - https://github.com/dell/csi-baremetal/issues/88
}

func (m *VolumeManager) checkVGErrors(lvg *lvgcrd.LogicalVolumeGroup, drivePath string) {
	ll := m.log.WithFields(logrus.Fields{
		"method": "checkVGErrors",
		"LVG":    lvg.Name,
	})

	ll.Infof("Scan volume group %s for IO errors", lvg.Name)
	m.recorder.Eventf(lvg, eventing.VolumeGroupScanInvolved, "Check for IO errors")

	isIOErrors, err := m.lvmOps.VGScan(lvg.Name)
	if err != nil {
		ll.Errorf("Failed to scan volume group %s for IO errors: %v", lvg.Name, err)
		m.recorder.Eventf(lvg, eventing.VolumeGroupScanFailed, err.Error())
		return
	}
	if isIOErrors {
		ll.Errorf("IO errors detected for volume group %s", lvg.Name)
		m.recorder.Eventf(lvg, eventing.VolumeGroupScanErrorsFound, "vgscan found input/output errors")
		return
	}

	blockDevices, err := m.listBlk.GetBlockDevices(drivePath)
	if err != nil {
		ll.Errorf("Failed to check volumes with lsblk for %s: %v", drivePath, err)
		m.recorder.Eventf(lvg, eventing.VolumeGroupScanFailed, err.Error())
		return
	}
	for _, v := range lvg.Spec.VolumeRefs {
		volumeFound := false
		for _, block := range blockDevices[0].Children {
			trimmedDashesName := strings.ReplaceAll(block.Name, "--", "-")
			if strings.Contains(trimmedDashesName, v) {
				volumeFound = true
				break
			}
		}

		if !volumeFound {
			ll.Errorf("Volume %s was not found on drive %s with LVG %s", v, drivePath, lvg.Name)
			ll.Debugf("Block devices on %s: %+v", drivePath, blockDevices)
			m.recorder.Eventf(lvg, eventing.VolumeGroupScanErrorsFound, "Volume %s was not found on drive %s", v, drivePath)
			return
		}
	}

	ll.Infof("No IO errors detected for volume group %s", lvg.Name)
	m.recorder.Eventf(lvg, eventing.VolumeGroupScanNoErrors, "No errors was found")
}

func (m *VolumeManager) reactivateVG(lvg *lvgcrd.LogicalVolumeGroup) {
	ll := m.log.WithFields(logrus.Fields{
		"method": "reactivateVG",
		"LVG":    lvg.Name,
	})

	ll.Infof("Trying to re-activate volume group %s", lvg.Name)
	m.recorder.Eventf(lvg, eventing.VolumeGroupReactivateInvolved,
		"Trying to re-activate volume group")

	if err := m.lvmOps.VGReactivate(lvg.Name); err != nil {
		// need to send an event if operation failed
		ll.Errorf("Failed to re-activate volume group %s: %v", lvg.Name, err)
		m.recorder.Eventf(lvg, eventing.VolumeGroupReactivateFailed, err.Error())
	}
}

// drivesAreTheSame check whether two drive represent same node drive or no
// method is rely on that each drive could be uniquely identified by it VID/PID/Serial Number
func (m *VolumeManager) drivesAreTheSame(drive1, drive2 *api.Drive) bool {
	return drive1.SerialNumber == drive2.SerialNumber &&
		drive1.VID == drive2.VID &&
		drive1.PID == drive2.PID
}

// createEventsForDriveUpdates create required events for drive state change
func (m *VolumeManager) createEventsForDriveUpdates(updates *driveUpdates) {
	for _, createdDrive := range updates.Created {
		m.sendEventForDrive(createdDrive, eventing.DriveDiscovered,
			"New drive discovered SN: %s, Node: %s.",
			createdDrive.Spec.SerialNumber, createdDrive.Spec.NodeId)
		m.createEventForDriveHealthChange(
			createdDrive, apiV1.HealthUnknown, createdDrive.Spec.Health)
	}
	for _, updDrive := range updates.Updated {
		if updDrive.CurrentState.Spec.Status != updDrive.PreviousState.Spec.Status {
			m.createEventForDriveStatusChange(
				updDrive.CurrentState, updDrive.PreviousState.Spec.Status, updDrive.CurrentState.Spec.Status)
		}
		if updDrive.CurrentState.Spec.Health != updDrive.PreviousState.Spec.Health {
			if _, ok := updDrive.CurrentState.Annotations[driveHealthOverrideAnnotation]; ok {
				m.createEventForDriveHealthOverridden(
					updDrive.CurrentState, updDrive.PreviousState.Spec.Health, updDrive.CurrentState.Spec.Health)
			}
			m.createEventForDriveHealthChange(
				updDrive.CurrentState, updDrive.PreviousState.Spec.Health, updDrive.CurrentState.Spec.Health)
		}
	}
}

func (m *VolumeManager) createEventForDriveHealthChange(
	drive *drivecrd.Drive, prevHealth, currentHealth string) {
	healthMsgTemplate := "Drive health is: %s, previous state: %s."
	var event *eventing.EventDescription
	switch currentHealth {
	case apiV1.HealthGood:
		event = eventing.DriveHealthGood
	case apiV1.HealthBad:
		event = eventing.DriveHealthFailure
	case apiV1.HealthSuspect:
		event = eventing.DriveHealthSuspect
	case apiV1.HealthUnknown:
		event = eventing.DriveHealthUnknown
	default:
		return
	}
	m.sendEventForDrive(drive, event,
		healthMsgTemplate, currentHealth, prevHealth)
}

func (m *VolumeManager) createEventForDriveStatusChange(
	drive *drivecrd.Drive, prevStatus, currentStatus string) {
	statusMsgTemplate := "Drive status is: %s, previous status: %s."
	var event *eventing.EventDescription
	switch currentStatus {
	case apiV1.DriveStatusOnline:
		event = eventing.DriveStatusOnline
	case apiV1.DriveStatusOffline:
		if drive.Spec.Usage == apiV1.DriveUsageRemoved {
			event = eventing.DriveSuccessfullyRemoved
			statusMsgTemplate = "Drive successfully removed. " + statusMsgTemplate
		} else {
			event = eventing.DriveStatusOffline
		}
	default:
		return
	}
	m.sendEventForDrive(drive, event,
		statusMsgTemplate, currentStatus, prevStatus)
}

// createEventForDriveHealthOverridden creates DriveHealthOverridden with Warning type
func (m *VolumeManager) createEventForDriveHealthOverridden(
	drive *drivecrd.Drive, realHealth, overriddenHealth string) {
	msgTemplate := "Drive health is overridden with: %s, real state: %s."
	event := eventing.DriveHealthOverridden
	m.sendEventForDrive(drive, event,
		msgTemplate, overriddenHealth, realHealth)
}

func (m *VolumeManager) sendEventForDrive(drive *drivecrd.Drive, event *eventing.EventDescription,
	messageFmt string, args ...interface{}) {
	messageFmt += " " + drive.GetDriveDescription() + fmt.Sprintf(", NodeName='%s'", m.nodeName)
	m.recorder.Eventf(drive, event, messageFmt, args...)
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
func (m *VolumeManager) isRootMountpoint(devs []lsblk.BlockDevice) bool {
	for _, device := range devs {
		if strings.TrimSpace(device.MountPoint) == base.KubeletRootPath ||
			strings.HasPrefix(strings.TrimSpace(device.MountPoint), base.HostRootPath) {
			return true
		}
		if m.isRootMountpoint(device.Children) {
			return true
		}
	}
	return false
}

// addVolumeStatusAnnotation add annotation with volume status to drive
func (m *VolumeManager) addVolumeStatusAnnotation(drive *drivecrd.Drive, volumeName, status string) {
	annotationKey := fmt.Sprintf("%s/%s", apiV1.DriveAnnotationVolumeStatusPrefix, volumeName)
	// init map if empty
	if drive.Annotations == nil {
		drive.Annotations = make(map[string]string)
	}
	drive.Annotations[annotationKey] = status
}

// handleExpandingStatus handles volume CR with Resizing status, it calls ExpandLV to expand volume
// To get logical volume name it use LVM provisioner function GetVolumePath
// Receive context, volume CR
// Return ctrl.DiscoverResult, error
func (m *VolumeManager) handleExpandingStatus(ctx context.Context, volume *volumecrd.Volume) (ctrl.Result, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method": "handleExpandingStatus",
	})
	volumePath, err := m.provisioners[p.LVMBasedVolumeType].GetVolumePath(&volume.Spec)
	if err != nil {
		ll.Errorf("Failed to get volume path, err: %v", err)
		return ctrl.Result{Requeue: true}, err
	}
	if err = m.lvmOps.ExpandLV(volumePath, volume.Spec.Size); err != nil {
		volume.Spec.CSIStatus = apiV1.Failed
	} else {
		volume.Spec.CSIStatus = apiV1.Resized
	}
	if updateErr := m.k8sClient.UpdateCR(ctx, volume); updateErr != nil {
		ll.Error("Unable to set new status for volume")
		return ctrl.Result{Requeue: true}, updateErr
	}
	return ctrl.Result{}, err
}

func (m *VolumeManager) changeDriveIsCleanField(drive *drivecrd.Drive, clean bool) {
	ll := m.log.WithFields(logrus.Fields{
		"method": "changeDriveIsCleanField",
	})
	drive.Spec.IsClean = clean
	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, drive.Name)
	if err := m.k8sClient.Update(ctxWithID, drive); err != nil {
		ll.Errorf("Unable to update drive CR %s: %v", drive.Name, err)
	}
}

func (m *VolumeManager) getPVCForVolume(volumeID string) (*corev1.PersistentVolumeClaim, error) {
	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, volumeID)

	pv := &corev1.PersistentVolume{}
	if err := m.k8sClient.Get(ctxWithID, k8sCl.ObjectKey{Name: volumeID}, pv); err != nil {
		m.log.Errorf("Failed to get Persistent Volume %s: %v", volumeID, err)
		return nil, err
	}

	pvcName := pv.Spec.ClaimRef.Name
	pvcNamespace := pv.Spec.ClaimRef.Namespace

	pvc := &corev1.PersistentVolumeClaim{}
	if err := m.k8sClient.Get(ctxWithID, k8sCl.ObjectKey{Name: pvcName, Namespace: pvcNamespace}, pvc); err != nil {
		m.log.Errorf("Failed to get Persistent Volume Claim %s in namespace %s: %v", pvcName, pvcNamespace, err)
		return nil, err
	}

	m.log.Debugf("PVC %s/%s was found for PV with ID - %s", pvc.Namespace, pvc.Name, pv.Name)
	return pvc, nil
}

func (m *VolumeManager) isPVCNeedFakeAttach(volumeID string) bool {
	pvc, err := m.getPVCForVolume(volumeID)
	if err != nil {
		m.log.Errorf("Failed to get Persistent Volume Claim for Volume %s: %+v", volumeID, err)
		return false
	}

	if value, ok := pvc.Annotations[fakeAttachAnnotation]; ok && value == fakeAttachAllowKey {
		return true
	}

	return false
}

// overrideDriveHealth replaces drive health with passed value,
// generates error message if value is not valid
func (m *VolumeManager) overrideDriveHealth(drive *api.Drive, overriddenHealth, driveCRName string) {
	overriddenHealth = strings.ToUpper(overriddenHealth)
	if (overriddenHealth == apiV1.HealthGood) ||
		(overriddenHealth == apiV1.HealthSuspect) ||
		(overriddenHealth == apiV1.HealthBad) ||
		(overriddenHealth == apiV1.HealthUnknown) {
		m.log.Warnf("Drive %s has health annotation. Health %s has been overridden with %s.",
			driveCRName, drive.Health, overriddenHealth)
		drive.Health = overriddenHealth
	} else {
		m.log.Errorf("Drive %s has health annotation, but value %s is not %s/%s/%s/%s. Health is not overridden.",
			driveCRName, overriddenHealth, apiV1.HealthGood, apiV1.HealthSuspect, apiV1.HealthBad, apiV1.HealthUnknown)
	}
}

func (m *VolumeManager) setWbtValue(vol *volumecrd.Volume) error {
	device, err := m.findDeviceName(vol)
	if err != nil {
		return err
	}

	err = m.wbtOps.SetValue(device, m.wbtConfig.Value)
	if err != nil {
		return err
	}

	return nil
}

func (m *VolumeManager) restoreWbtValue(vol *volumecrd.Volume) error {
	device, err := m.findDeviceName(vol)
	if err != nil {
		return err
	}

	err = m.wbtOps.RestoreDefault(device)
	if err != nil {
		return err
	}

	return nil
}

func (m *VolumeManager) checkWbtChangingEnable(ctx context.Context, vol *volumecrd.Volume) bool {
	if m.wbtConfig == nil {
		return false
	}

	if !m.wbtConfig.Enable {
		return false
	}

	isModeAcceptable := false
	for _, mode := range m.wbtConfig.VolumeOptions.Modes {
		if mode == vol.Spec.Mode {
			isModeAcceptable = true
			break
		}
	}
	if !isModeAcceptable {
		m.log.Infof("Skip wbt value changing: volume %s has mode %s, acceptable: %v", vol.Name, vol.Spec.Mode, m.wbtConfig.VolumeOptions.Modes)
		return false
	}

	volumeID := vol.Name
	pv := &corev1.PersistentVolume{}
	if err := m.k8sClient.Get(ctx, k8sCl.ObjectKey{Name: volumeID}, pv); err != nil {
		m.log.Errorf("Failed to get Persistent Volume %s: %v", volumeID, err)
		return false
	}
	volSC := pv.Spec.StorageClassName

	isSCAcceptable := false
	for _, sc := range m.wbtConfig.VolumeOptions.StorageClasses {
		if sc == volSC {
			isSCAcceptable = true
			break
		}
	}
	if !isSCAcceptable {
		m.log.Infof("Skip wbt value changing: volume %s has sc %s, acceptable: %v", vol.Name, volSC, m.wbtConfig.VolumeOptions.StorageClasses)
		return false
	}

	return true
}

func (m *VolumeManager) findDeviceName(vol *volumecrd.Volume) (string, error) {
	drive, err := m.crHelper.GetDriveCRByVolume(vol)
	if err != nil {
		return "", err
	}
	if drive == nil {
		return "", fmt.Errorf("drive %s is not found", vol.Spec.Location)
	}

	// expected device path - /dev/<device>
	splitedPath := regexp.MustCompile(`[A-Za-z0-9]+`).FindAllString(drive.Spec.Path, -1)
	if len(splitedPath) != 2 {
		return "", fmt.Errorf("drive path %s is not parsable as /dev/<device>", drive.Spec.Path)
	}
	if splitedPath[0] != "dev" {
		return "", fmt.Errorf("drive path %s is not parsable as /dev/<device>", drive.Spec.Path)
	}

	return splitedPath[1], nil
}

// SetWbtConfig changes Wbt Config for vlmgr instance
func (m *VolumeManager) SetWbtConfig(conf *wbtconf.WbtConfig) {
	if m.wbtConfig == nil || !reflect.DeepEqual(*m.wbtConfig, *conf) {
		m.log.Infof("Wbt config changed: %+v", *conf)
		m.wbtConfig = conf
	}
}
