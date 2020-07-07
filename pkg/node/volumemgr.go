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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lsblk"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/lvm"
	ph "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/partitionhelper"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/common"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/eventing"
	p "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node/provisioners"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node/provisioners/utilwrappers"
)

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
	// used for discoverLVGOnSystemDisk method
	discoverLvgSSD bool
	// whether VolumeManager was initialized or no, uses for health probes
	initialized bool
	// general logger
	log *logrus.Entry
	// sink where we write events
	recorder eventRecorder
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
		fsOps:          utilwrappers.NewFSOperationsImpl(executor, logger),
		lvmOps:         lvm.NewLVM(executor, logger),
		listBlk:        lsblk.NewLSBLK(logger),
		partOps:        ph.NewWrapPartitionImpl(executor, logger),
		nodeID:         nodeID,
		log:            logger.WithField("component", "VolumeManager"),
		recorder:       recorder,
		discoverLvgSSD: true,
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
	ctx, cancelFn := context.WithTimeout(
		context.WithValue(context.Background(), k8s.RequestUUID, req.Name),
		VolumeOperationsTimeout)
	defer cancelFn()

	ll := m.log.WithFields(logrus.Fields{
		"method":   "Reconcile",
		"volumeID": req.Name,
	})

	volume := &volumecrd.Volume{}

	err := m.k8sClient.ReadCR(ctx, req.Name, volume)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Here we need to check that this VolumeCR corresponds to this node
	// because we deploy VolumeCRD Controller as DaemonSet
	if volume.Spec.NodeId != m.nodeID {
		return ctrl.Result{}, nil
	}

	ll.Infof("Processing for status %s", volume.Spec.CSIStatus)
	var newStatus string
	switch volume.Spec.CSIStatus {
	case apiV1.Creating:
		err := m.getProvisionerForVolume(&volume.Spec).PrepareVolume(volume.Spec)
		if err != nil {
			ll.Errorf("Unable to create volume size of %d bytes. Error: %v. Context Error: %v."+
				" Set volume status to Failed", volume.Spec.Size, err, ctx.Err())
			newStatus = apiV1.Failed
		} else {
			ll.Infof("CreateLocalVolume completed successfully. Set status to Created")
			newStatus = apiV1.Created
		}

		volume.Spec.CSIStatus = newStatus
		if err = m.k8sClient.UpdateCRWithAttempts(ctx, volume, 5); err != nil {
			// Here we can return error because Volume created successfully and we can try to change CR's status
			// one more time
			ll.Errorf("Unable to update volume status to %s: %v", newStatus, err)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	case apiV1.Removing:
		if err = m.getProvisionerForVolume(&volume.Spec).ReleaseVolume(volume.Spec); err != nil {
			ll.Errorf("Failed to remove volume - %s. Error: %v. Set status to Failed", volume.Spec.Id, err)
			newStatus = apiV1.Failed
		} else {
			ll.Infof("Volume - %s was successfully removed. Set status to Removed", volume.Spec.Id)
			newStatus = apiV1.Removed
		}

		volume.Spec.CSIStatus = newStatus
		if err = m.k8sClient.UpdateCRWithAttempts(ctx, volume, 10); err != nil {
			ll.Error("Unable to set new status for volume")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, nil
	}
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

	m.updateDrivesCRs(ctx, drivesResponse.Disks)

	freeDrives := m.drivesAreNotUsed()
	if err = m.discoverVolumeCRs(freeDrives); err != nil {
		return err
	}

	if err = m.discoverAvailableCapacity(ctx, m.nodeID); err != nil {
		return err
	}

	if m.discoverLvgSSD {
		if err = m.discoverLVGOnSystemDrive(); err != nil {
			m.log.WithField("method", "Discover").
				Errorf("unable to inspect system LVG: %v", err)
		}
	}
	m.initialized = true
	return nil
}

// updateDrivesCRs updates drives cache and Drives CRs based on provided list of Drives. Tries to fill drivesCache in
// case of VolumeManager restart from Drives CRs.
// Receives golang context and slice of discovered api.Drive structs usually got from DriveManager
func (m *VolumeManager) updateDrivesCRs(ctx context.Context, discoveredDrives []*api.Drive) {
	ll := m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "updateDrivesCRs",
	})
	ll.Debugf("Processing")

	driveCRs := m.crHelper.GetDriveCRs(m.nodeID)

	// Try to find not existing CR for discovered drives
	for _, drivePtr := range discoveredDrives {
		exist := false
		for _, driveCR := range driveCRs {
			// If drive CR already exist, try to update, if drive was changed
			if m.drivesAreTheSame(drivePtr, &driveCR.Spec) {
				exist = true
				if !driveCR.Equals(drivePtr) {
					drivePtr.UUID = driveCR.Spec.UUID
					// If got from DriveMgr drive's health is not equal to drive's health in cache then update appropriate
					// resources
					if drivePtr.Health != driveCR.Spec.Health || drivePtr.Status != driveCR.Spec.Status {
						m.handleDriveStatusChange(ctx, drivePtr)
					}
					toUpdate := driveCR
					toUpdate.Spec = *drivePtr
					if err := m.k8sClient.UpdateCR(ctx, &toUpdate); err != nil {
						ll.Errorf("Failed to update drive CR (health/status) %v, error %v", toUpdate, err)
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
			driveCR := m.k8sClient.ConstructDriveCR(toCreateSpec.UUID, toCreateSpec)
			if err := m.k8sClient.CreateCR(ctx, driveCR.Name, driveCR); err != nil {
				ll.Errorf("Failed to create drive CR %v, error: %v", driveCR, err)
			}
		}
	}

	// that means that it is a first round and drives are discovered first time
	if len(driveCRs) == 0 {
		return
	}

	// Try to find missing drive in drive CRs and update according CR
	for _, d := range m.crHelper.GetDriveCRs(m.nodeID) {
		wasDiscovered := false
		for _, drive := range discoveredDrives {
			if m.drivesAreTheSame(&d.Spec, drive) {
				wasDiscovered = true
				break
			}
		}
		isInLVG := false
		if !wasDiscovered {
			ll.Debugf("Check whether drive %v in LVG or no", d)
			isInLVG = m.isDriveIsInLVG(d.Spec)
		}
		if !wasDiscovered && !isInLVG {
			// TODO: remove AC and aware Volumes here
			ll.Warnf("Set status OFFLINE for drive %v", d.Spec)
			toUpdate := d
			toUpdate.Spec.Status = apiV1.DriveStatusOffline
			toUpdate.Spec.Health = apiV1.HealthUnknown
			err := m.k8sClient.UpdateCR(ctx, &toUpdate)
			if err != nil {
				ll.Errorf("Failed to update drive CR %v, error %v", toUpdate, err)
			}
		}
	}
}

// isDriveIsInLVG check whether drive is a part of some LVG or no
func (m *VolumeManager) isDriveIsInLVG(d api.Drive) bool {
	lvgs := m.crHelper.GetLVGCRs(m.nodeID)
	for _, lvg := range lvgs {
		if util.ContainsString(lvg.Spec.Locations, d.UUID) {
			return true
		}
	}
	return false
}

// discoverVolumeCRs updates volumes cache based on provided freeDrives.
// searches drives in freeDrives that are not have volume and if there are some partitions on them - try to read
// partition uuid and create volume object
func (m *VolumeManager) discoverVolumeCRs(freeDrives []*drivecrd.Drive) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "discoverVolumeCRs",
	})

	// explore each drive from freeDrives
	lsblk, err := m.listBlk.GetBlockDevices("")
	if err != nil {
		return fmt.Errorf("unable to inspect system block devices via lsblk, error: %v", err)
	}

	for _, d := range freeDrives {
		for _, ld := range lsblk {
			if strings.EqualFold(ld.Serial, d.Spec.SerialNumber) && len(ld.Children) > 0 {
				if m.isDriveIsInLVG(d.Spec) {
					ll.Debugf("Drive %v is in LVG and not a FREE", d.Spec)
					break
				}
				partUUID, err := m.partOps.GetPartitionUUID(ld.Name, p.DefaultPartitionNumber)
				if err != nil {
					ll.Warnf("Unable to determine partition UUID for device %s, error: %v", ld.Name, err)
					continue
				}
				size, err := strconv.ParseInt(ld.Size, 10, 64)
				if err != nil {
					ll.Warnf("Unable parse string %s to int, for device %s, error: %v", ld.Size, ld.Name, err)
					continue
				}

				volumeCR := m.k8sClient.ConstructVolumeCR(partUUID, api.Volume{
					NodeId:       m.nodeID,
					Id:           partUUID,
					Size:         size,
					Location:     d.Spec.UUID,
					LocationType: apiV1.LocationTypeDrive,
					Mode:         apiV1.ModeFS,
					Type:         ld.FSType,
					Health:       d.Spec.Health,
					CSIStatus:    "",
				})
				ll.Infof("Creating volume CR: %v", volumeCR)
				if err = m.k8sClient.CreateCR(context.Background(), partUUID, volumeCR); err != nil {
					ll.Errorf("Unable to create volume CR %s: %v", partUUID, err)
				}
			}
		}
	}
	return nil
}

// DiscoverAvailableCapacity inspect current available capacity on nodes and fill AC CRs. This method manages only
// hardware available capacity such as HDD or SSD. If drive is healthy and online and also it is not used in LVGs
// and it doesn't contain volume then this drive is in AvailableCapacity CRs.
// Returns error if at least one drive from cache was handled badly
func (m *VolumeManager) discoverAvailableCapacity(ctx context.Context, nodeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "discoverAvailableCapacity",
	})

	var (
		err       error
		wasError  = false
		volumeCRs = m.crHelper.GetVolumeCRs(m.nodeID)
		lvgList   = &lvgcrd.LVGList{}
		acList    = &accrd.AvailableCapacityList{}
	)

	if err = m.k8sClient.ReadList(ctx, lvgList); err != nil {
		return fmt.Errorf("failed to get LVG CRs list: %v", err)
	}
	if err = m.k8sClient.ReadList(ctx, acList); err != nil {
		return fmt.Errorf("unable to read AC list: %v", err)
	}

	for _, drive := range m.crHelper.GetDriveCRs(m.nodeID) {
		if drive.Spec.Health != apiV1.HealthGood || drive.Spec.Status != apiV1.DriveStatusOnline {
			continue
		}

		isUsed := false

		// check whether drive is consumed by volume or no
		for _, volume := range volumeCRs {
			if strings.EqualFold(volume.Spec.Location, drive.Spec.UUID) {
				isUsed = true
				break
			}
		}

		// check whether drive is consumed by LVG or no
		if !isUsed {
			for _, lvg := range lvgList.Items {
				if util.ContainsString(lvg.Spec.Locations, drive.Spec.UUID) {
					isUsed = true
					break
				}
			}
		}

		// drive is consumed by volume or LVG
		if isUsed {
			continue
		}

		// If drive isn't used by Volume or LVG then try to create AC CR from it
		capacity := &api.AvailableCapacity{
			Size:         drive.Spec.Size,
			Location:     drive.Spec.UUID,
			StorageClass: util.ConvertDriveTypeToStorageClass(drive.Spec.Type),
			NodeId:       nodeID,
		}

		name := capacity.NodeId + "-" + strings.ToLower(capacity.Location)

		// check whether appropriate AC exists or no
		acExist := false
		for _, ac := range acList.Items {
			if ac.Spec.Location == drive.Spec.UUID {
				acExist = true
				break
			}
		}
		if !acExist {
			newAC := m.k8sClient.ConstructACCR(name, *capacity)
			ll.Infof("Creating Available Capacity %v", newAC)
			if err := m.k8sClient.CreateCR(context.WithValue(ctx, k8s.RequestUUID, name),
				name, newAC); err != nil {
				ll.Errorf("Error during CreateAvailableCapacity request to k8s: %v, error: %v",
					capacity, err)
				wasError = true
			}
		}
	}

	if wasError {
		return errors.New("not all available capacity were created")
	}

	return nil
}

// drivesAreNotUsed search drives in drives CRs that isn't have any volumes
// Returns slice of pointers on drivecrd.Drive structs
func (m *VolumeManager) drivesAreNotUsed() []*drivecrd.Drive {
	// search drives that don't have parent volume
	drives := make([]*drivecrd.Drive, 0)
	for _, d := range m.crHelper.GetDriveCRs(m.nodeID) {
		isUsed := false
		for _, v := range m.crHelper.GetVolumeCRs(m.nodeID) {
			// expect only Drive LocationType, for Drive LocationType Location will be a UUID of the drive
			if d.Spec.Type != apiV1.DriveTypeNVMe &&
				v.Spec.LocationType == apiV1.LocationTypeDrive &&
				strings.EqualFold(d.Spec.UUID, v.Spec.Location) {
				isUsed = true
				break
			}
		}
		if !isUsed {
			dInst := d
			drives = append(drives, &dInst)
		}
	}
	return drives
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
		if lvg.Spec.Node == m.nodeID && lvg.Spec.Locations[0] == base.SystemDriveAsLocation {
			var vgFreeSpace int64
			if vgFreeSpace, err = m.lvmOps.GetVgFreeSpace(lvg.Spec.Name); err != nil {
				return err
			}
			ll.Infof("LVG CR that points on system VG is exists: %v", lvg)
			return m.createACIfFreeSpace(lvg.Name, apiV1.StorageClassSSDLVG, vgFreeSpace)
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

	isMountPoint, err := m.lvmOps.IsLVGExists(rootMountPoint)

	if err != nil {
		return fmt.Errorf(errTmpl, err)
	}

	if !isMountPoint {
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
	var (
		vgCRName = uuid.New().String()
		vg       = api.LogicalVolumeGroup{
			Name:      vgName,
			Node:      m.nodeID,
			Locations: []string{base.SystemDriveAsLocation},
			Size:      vgFreeSpace,
			Status:    apiV1.Created,
		}
		vgCR = m.k8sClient.ConstructLVGCR(vgCRName, vg)
		ctx  = context.WithValue(context.Background(), k8s.RequestUUID, vg.Name)
	)
	if err = m.k8sClient.CreateCR(ctx, vg.Name, vgCR); err != nil {
		return fmt.Errorf("unable to create LVG CR %v, error: %v", vgCR, err)
	}
	return m.createACIfFreeSpace(vgCRName, apiV1.StorageClassSSDLVG, vgFreeSpace)
}

// getProvisionerForVolume returns appropriate Provisioner implementation for volume
func (m *VolumeManager) getProvisionerForVolume(vol *api.Volume) p.Provisioner {
	switch vol.StorageClass {
	case apiV1.StorageClassHDDLVG, apiV1.StorageClassSSDLVG:
		return m.provisioners[p.LVMBasedVolumeType]
	default:
		return m.provisioners[p.DriveBasedVolumeType]
	}
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
	// TODO AK8S-472 Handle disk health which are used by LVGs
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
