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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/common"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
)

// VolumeManager is the struct to perform volume operations on node side with real storage devices
type VolumeManager struct {
	k8sClient      *k8s.KubeClient
	crHelper       *k8s.CRHelper
	driveMgrClient api.DriveServiceClient
	scMap          map[SCName]sc.StorageClassImplementer

	nodeID      string
	initialized bool

	linuxUtils *linuxutils.LinuxUtils
	acProvider common.AvailableCapacityOperations

	log *logrus.Entry
}

const (
	// DiscoverDrivesTimeout is the timeout for Discover method
	DiscoverDrivesTimeout = 300 * time.Second
	// VolumeOperationsTimeout is the timeout for local Volume creation/deletion
	VolumeOperationsTimeout = 900 * time.Second
	// SleepBetweenRetriesToSyncPartTable is the interval between syncing of partition table
	SleepBetweenRetriesToSyncPartTable = 3 * time.Second
	// NumberOfRetriesToSyncPartTable is the amount of retries for partprobe of a particular device
	NumberOfRetriesToSyncPartTable = 3
)

// NewVolumeManager is the constructor for VolumeManager struct
// Receives an instance of DriveServiceClient to interact with DriveManager, CmdExecutor to execute linux commands,
// logrus logger, base.KubeClient and ID of a node where VolumeManager works
// Returns an instance of VolumeManager
func NewVolumeManager(client api.DriveServiceClient, executor command.CmdExecutor, logger *logrus.Logger, k8sclient *k8s.KubeClient, nodeID string) *VolumeManager {
	vm := &VolumeManager{
		k8sClient:      k8sclient,
		crHelper:       k8s.NewCRHelper(k8sclient, logger),
		driveMgrClient: client,
		linuxUtils:     linuxutils.NewLinuxUtils(executor, logger),
		nodeID:         nodeID,
		scMap: map[SCName]sc.StorageClassImplementer{
			"hdd": sc.GetHDDSCInstance(logger),
			"ssd": sc.GetSSDSCInstance(logger)},
		acProvider: common.NewACOperationsImpl(k8sclient, logger),
	}
	vm.log = logger.WithField("component", "VolumeManager")
	return vm
}

// SetExecutor sets provided CmdExecutor to LinuxUtils field of VolumeManager
func (m *VolumeManager) SetExecutor(executor command.CmdExecutor) {
	m.linuxUtils.SetExecutor(executor)
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

	ll.Info("Reconciling Volume")
	var newStatus string
	switch volume.Spec.CSIStatus {
	case apiV1.Creating:
		err := m.CreateLocalVolume(ctx, &volume.Spec)
		if err != nil {
			ll.Errorf("Unable to create volume size of %d bytes. Error: %v. Context Error: %v."+
				" Set volume status to FailedToCreate", volume.Spec.Size, err, ctx.Err())
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
		if err = m.DeleteLocalVolume(ctx, &volume.Spec); err != nil {
			ll.Errorf("Failed to delete volume - %s. Error: %v. Context Error: %v. "+
				"Set status FailToRemove", volume.Spec.Id, err, ctx.Err())
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&volumecrd.Volume{}).
		Complete(m)
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

	if !m.initialized {
		err = m.discoverLVGOnSystemDrive()
		if err != nil {
			m.log.WithField("method", "Discover").
				Errorf("unable to inspect system LVG: %v", err)
		}
		m.initialized = true
	}
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
		if !wasDiscovered {
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

// discoverVolumeCRs updates volumes cache based on provided freeDrives.
// searches drives in freeDrives that are not have volume and if there are some partitions on them - try to read
// partition uuid and create volume object
func (m *VolumeManager) discoverVolumeCRs(freeDrives []*drivecrd.Drive) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "discoverVolumeCRs",
	})

	// explore each drive from freeDrives
	lsblk, err := m.linuxUtils.GetBlockDevices("")
	if err != nil {
		return fmt.Errorf("unable to inspect system block devices via lsblk, error: %v", err)
	}

	for _, d := range freeDrives {
		for _, ld := range lsblk {
			if strings.EqualFold(ld.Serial, d.Spec.SerialNumber) && len(ld.Children) > 0 {
				partUUID, err := m.linuxUtils.GetPartitionUUID(ld.Name)
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
			ll.Infof("LVG CR that points on system VG is exists: %v", lvg)
			return nil
		}
	}

	var (
		rootMountPoint, vgName string
		vgFreeSpace            int64
	)

	if rootMountPoint, err = m.linuxUtils.FindMnt(base.KubeletRootPath); err != nil {
		return fmt.Errorf(errTmpl, err)
	}

	// from container we expect here name like "VG_NAME[/var/lib/kubelet/pods]"
	rootMountPoint = strings.Split(rootMountPoint, "[")[0]

	// ensure that rootMountPoint is in SSD drive
	devices, err := m.linuxUtils.GetBlockDevices(rootMountPoint)
	if err != nil {
		return fmt.Errorf(errTmpl, err)
	}

	if devices[0].Rota != base.NonRotationalNum {
		ll.Infof("System disk is not SSD. LVG will not be created base on it.")
		return nil
	}

	if vgName, err = m.linuxUtils.FindVgNameByLvName(rootMountPoint); err != nil {
		return fmt.Errorf(errTmpl, err)
	}

	if vgFreeSpace, err = m.linuxUtils.GetVgFreeSpace(vgName); err != nil {
		return fmt.Errorf(errTmpl, err)
	}
	if vgFreeSpace == 0 {
		vgFreeSpace++ // if size is 0 it field will not display for CR
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

	if vgFreeSpace > common.AcSizeMinThresholdBytes {
		acName := uuid.New().String()
		acCR := m.k8sClient.ConstructACCR(acName, api.AvailableCapacity{
			Location:     vgCRName,
			NodeId:       m.nodeID,
			StorageClass: apiV1.StorageClassSSDLVG,
			Size:         vgFreeSpace,
		})
		if err = m.k8sClient.CreateCR(ctx, acName, acCR); err != nil {
			return fmt.Errorf("unable to create AC based on system LVG, error: %v", err)
		}
		ll.Infof("AC %v was created based on system LVG", acCR)
	}

	ll.Infof("System LVM was inspected, LVG object was created")
	return nil
}

// CreateLocalVolume performs linux operations on the node to create specified volume on hardware drives.
// If StorageClass of provided api.Volume is LVG then it creates LV based on the VG and creates file system on this LV.
// If StorageClass of provided api.Volume is HDD or SSD then it creates partition on drive and creates file system on
// this partition. api.Volume.Location must be set for correct working of this method
// Returns error if something went wrong
func (m *VolumeManager) CreateLocalVolume(ctx context.Context, vol *api.Volume) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "CreateLocalVolume",
		"volumeID": vol.Id,
	})

	ll.Infof("Creating volume: %v", vol)

	var (
		volLocation = vol.Location
		scImpl      = m.getStorageClassImpl(vol.StorageClass)
		deviceFile  string
		err         error
	)
	switch vol.StorageClass {
	//TODO AK8S-762 Use createPartitionAndSetUUID for SSDLVG, HDDLVG SC in CreateLocalVolume
	case apiV1.StorageClassSSDLVG, apiV1.StorageClassHDDLVG:
		sizeStr := fmt.Sprintf("%.2fG", float64(vol.Size)/float64(util.GBYTE))
		vgName := vol.Location

		// Volume.Location is a LVG CR name and we use such name as a real VG name
		// however for LVG based on system disk LVG CR name != VG name
		// we need to read appropriate LVG CR and use LVG CR.Spec.Name in LVCreate command
		if vol.StorageClass == apiV1.StorageClassSSDLVG {
			vgName, err = m.crHelper.GetVGNameByLVGCRName(volLocation)
			if err != nil {
				return fmt.Errorf("unable to find LVG name by LVG CR name: %v", err)
			}
		}

		// create lv with name /dev/VG_NAME/vol.Id
		ll.Infof("Creating LV %s sizeof %s in VG %s", vol.Id, sizeStr, vgName)
		if err = m.linuxUtils.LVCreate(vol.Id, sizeStr, vgName); err != nil {
			return fmt.Errorf("unable to create LV: %v", err)
		}

		ll.Info("LV was created.")
		deviceFile = fmt.Sprintf("/dev/%s/%s", vgName, vol.Id)
	default: // assume that volume location is whole disk
		drive := &drivecrd.Drive{}

		// read Drive CR based on Volume.Location (vol.Location == Drive.UUID == Drive.Name)
		if err = m.k8sClient.ReadCR(ctx, volLocation, drive); err != nil {
			return fmt.Errorf("failed to read drive CR with name %s, error %v", volLocation, err)
		}

		ll.Infof("Search device file for drive with S/N %s", drive.Spec.SerialNumber)
		device, err := m.linuxUtils.SearchDrivePath(drive)
		if err != nil {
			return err
		}

		ll.Infof("Create partition on device %s and set UUID in background", device)
		id := vol.Id

		// since volume ID starts with 'pvc-' prefix we need to remove it.
		// otherwise partition UUID won't be set correctly
		// todo can we guarantee that e2e test has 'pvc-' prefix
		volumeUUID, _ := util.GetVolumeUUID(id)
		partition, rollBacked, err := m.createPartitionAndSetUUID(device, volumeUUID, vol.Ephemeral)
		if err != nil {
			if !rollBacked {
				ll.Errorf("unable set partition uuid for dev %s, error: %v, roll back failed too, set drive status to OFFLINE", device, err)
				drive.Spec.Status = apiV1.DriveStatusOffline
				if err := m.k8sClient.UpdateCR(ctx, drive); err != nil {
					ll.Errorf("Failed to update drive CR %s, error %v", drive.Name, err)
				}
			}
			return fmt.Errorf("failed to set partition UUID: %v", err)
		}
		deviceFile = partition
		ll.Info("Partition was created successfully")
	}

	if err = scImpl.CreateFileSystem(sc.FileSystem(vol.Type), deviceFile); err != nil {
		return fmt.Errorf("failed to create file system: %v, set volume status FailedToCreate", err)
	}

	ll.Info("Local volume was created successfully")
	return nil
}

// createPartitionAndSetUUID creates partition and sets partition UUID, if some step fails
// will try to rollback operation, returns error and roll back operation status (bool)
// if error occurs, status value will show whether device has roll back to the initial state
func (m *VolumeManager) createPartitionAndSetUUID(device string, uuid string, ephemeral bool) (partName string, rollBacked bool, err error) {
	ll := m.log.WithFields(logrus.Fields{
		"method": "createPartitionAndSetUUID",
		"uuid":   uuid,
	})
	ll.Infof("Processing for device %s", device)

	var exist bool
	rollBacked = true

	// check existence
	exist, err = m.linuxUtils.IsPartitionExists(device)
	if err != nil {
		return
	}
	// check partition UUID
	if exist {
		currUUID, err := m.linuxUtils.GetPartitionUUID(device)
		if err != nil {
			ll.Errorf("Partition has already exist but fail to get it UUID: %v", err)
			return "", false, fmt.Errorf("partition has already exist on device %s", device)
		}
		if currUUID == uuid {
			ll.Infof("Partition has already set.")
			return "", true, nil
		}
		return "", false, fmt.Errorf("partition has already exist on device %s", device)
	}

	// create partition table
	err = m.linuxUtils.CreatePartitionTable(device)
	if err != nil {
		return
	}

	// create partition
	err = m.linuxUtils.CreatePartition(device)
	if err != nil {
		// try to delete partition
		// todo get rid of this. might cause DL
		exist, _ = m.linuxUtils.IsPartitionExists(device)
		if exist {
			if errDel := m.linuxUtils.DeletePartition(device); errDel != nil {
				rollBacked = false
				return
			}
		}
		return
	}

	// set partition UUID
	err = m.linuxUtils.SetPartitionUUID(device, uuid)
	if err != nil {
		errDel := m.linuxUtils.DeletePartition(device)
		if errDel != nil {
			rollBacked = false
			return
		}
		return
	}
	//TODO temporary solution because of ephemeral volumes volume id https://jira.cec.lab.emc.com:8443/browse/AK8S-749
	if ephemeral {
		uuid, err = m.linuxUtils.GetPartitionUUID(device)
		if err != nil {
			ll.Errorf("Partition has already exist but fail to get it UUID: %v", err)
			return "", false, fmt.Errorf("partition has already exist on device %s", device)
		}
	}
	// get partition name
	for i := 0; i < NumberOfRetriesToSyncPartTable; i++ {
		partName, err = m.linuxUtils.GetPartitionNameByUUID(device, uuid)
		if err != nil {
			// sync partition table and try one more time
			err = m.linuxUtils.SyncPartitionTable(device)
			if err != nil {
				// log and ignore error
				ll.Warningf("Unable to sync partition table for device %s", device)
			}
			time.Sleep(SleepBetweenRetriesToSyncPartTable)
			continue
		}
		break
	}

	if partName == "" {
		// delete partition
		// todo https://jira.cec.lab.emc.com:8443/browse/AK8S-719 need to refactor this method to avoid code duplicates
		errDel := m.linuxUtils.DeletePartition(device)
		if errDel != nil {
			rollBacked = false
			err = errDel
			return
		}
		err = fmt.Errorf("unable to obtain partition name for device %s", device)
		return
	}

	return partName, false, nil
}

// DeleteLocalVolume performs linux operations on the node to delete specified volume from hardware drives.
// If StorageClass of provided api.Volume is LVG then it deletes file system from the LV that based on the VG and
// then deletes the LV. If StorageClass of provided api.Volume is HDD or SSD then it deletes file system from the
// partition of Volume's drive and then it deletes this partition.
// Returns error if something went wrong
func (m *VolumeManager) DeleteLocalVolume(ctx context.Context, volume *api.Volume) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "DeleteLocalVolume",
		"volumeID": volume.Id,
	})

	ll.Info("Processing request")

	var (
		err        error
		deviceFile string
	)
	switch volume.StorageClass {
	case apiV1.StorageClassHDD, apiV1.StorageClassSSD:
		drive := m.crHelper.GetDriveCRByUUID(volume.Location)

		if drive == nil {
			return errors.New("unable to find drive by volume location")
		}
		// get deviceFile path
		deviceFile, err = m.linuxUtils.SearchDrivePath(drive)
		if err != nil {
			return fmt.Errorf("unable to find device for drive with S/N %s", volume.Location)
		}

		err = m.linuxUtils.DeletePartition(deviceFile)
		if err != nil {
			return fmt.Errorf("failed to delete partition, error: %v", err)
		}
		ll.Info("Partition was deleted successfully")
	case apiV1.StorageClassSSDLVG, apiV1.StorageClassHDDLVG:
		vgName := volume.Location
		var err error
		// Volume.Location is a LVG CR however for LVG based on system disk LVG CR name != VG name
		// we need to read appropriate LVG CR and use LVG CR.Spec.Name as VG name
		if volume.StorageClass == apiV1.StorageClassSSDLVG {
			vgName, err = m.crHelper.GetVGNameByLVGCRName(volume.Location)
			if err != nil {
				return fmt.Errorf("unable to find LVG name by LVG CR name: %v", err)
			}
		}
		deviceFile = fmt.Sprintf("/dev/%s/%s", vgName, volume.Id) // /dev/VG_NAME/LV_NAME
	default:
		return fmt.Errorf("unable to determine storage class for volume %v", volume)
	}

	ll.Infof("Found device file %s", deviceFile)
	scImpl := m.getStorageClassImpl(volume.StorageClass)

	if err = scImpl.DeleteFileSystem(deviceFile); err != nil {
		return fmt.Errorf("failed to wipefs deviceFile, error: %v", err)
	}

	if volume.StorageClass == apiV1.StorageClassHDDLVG || volume.StorageClass == apiV1.StorageClassSSDLVG {
		lvgName := volume.Location
		ll.Infof("Removing LV %s from LVG %s", volume.Id, lvgName)
		if err = m.linuxUtils.LVRemove(deviceFile); err != nil {
			return fmt.Errorf("unable to remove lv: %v", err)
		}
	}

	ll.Infof("Local volume %v was removed successfully", volume)
	return nil
}

// getStorageClassImpl returns appropriate StorageClass implementation from VolumeManager scMap field
func (m *VolumeManager) getStorageClassImpl(storageClass string) sc.StorageClassImplementer {
	switch storageClass {
	case apiV1.StorageClassHDD:
		return m.scMap[SCName("hdd")]
	case apiV1.StorageClassSSD:
		return m.scMap[SCName("ssd")]
	default:
		return m.scMap[SCName("hdd")]
	}
}

// SetSCImplementer sets sc.StorageClassImplementer implementation to scMap of VolumeManager
// Receives scName which uses as a key and an instance of sc.StorageClassImplementer
func (m *VolumeManager) SetSCImplementer(scName string, implementer sc.StorageClassImplementer) {
	m.scMap[SCName(scName)] = implementer
}

// updateVolumeCRSpec reads volume CR with name volName and update it's spec to newSpec
// returns nil or error in case of error
func (m *VolumeManager) updateVolumeCRSpec(volName string, newSpec api.Volume) error {
	var (
		volumeCR = &volumecrd.Volume{}
		err      error
	)

	if err = m.k8sClient.ReadCR(context.Background(), volName, volumeCR); err != nil {
		return err
	}

	volumeCR.Spec = newSpec
	return m.k8sClient.UpdateCR(context.Background(), volumeCR)
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
		vol.Spec.Health = drive.Health
		if err := m.k8sClient.UpdateCR(ctx, vol); err != nil {
			ll.Errorf("Failed to update volume CR's %s health status: %v", vol.Name, err)
		}
	}

	// Handle resources with LVG
	// This is not work for the current moment because HAL doesn't monitor disks with LVM
	// TODO AK8S-472 Handle disk health which are used by LVGs
}

func (m *VolumeManager) drivesAreTheSame(drive1, drive2 *api.Drive) bool {
	return drive1.SerialNumber == drive2.SerialNumber &&
		drive1.VID == drive2.VID &&
		drive1.PID == drive2.PID
}
