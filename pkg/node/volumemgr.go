package node

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/common"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
)

// VolumeManager is the struct to perform volume operations on node side with real storage devices
type VolumeManager struct {
	k8sclient *base.KubeClient

	drivesCache map[string]*drivecrd.Drive

	hWMgrClient api.HWServiceClient
	// stores volumes that actually is use, key - volume ID
	volumesCache map[string]*api.Volume
	vCacheMu     sync.Mutex
	// stores drives that had discovered on previous steps, key - S/N
	dCacheMu sync.Mutex

	scMap map[SCName]sc.StorageClassImplementer

	linuxUtils *base.LinuxUtils
	log        *logrus.Entry
	nodeID     string

	acProvider common.AvailableCapacityOperations

	initialized bool
}

const (
	// DiscoverDrivesTimeout is the timeout for Discover method
	DiscoverDrivesTimeout = 300 * time.Second
	// VolumeOperationsTimeout is the timeout for local Volume creation/deletion
	VolumeOperationsTimeout = 600 * time.Second
	// SleepBetweenRetriesToSyncPartTable is the interval between syncing of partition table
	SleepBetweenRetriesToSyncPartTable = 3 * time.Second
	// NumberOfRetriesToSyncPartTable is the amount of retries for partprobe of a particular device
	NumberOfRetriesToSyncPartTable = 3
)

// NewVolumeManager is the constructor for VolumeManager struct
// Receives an instance of HWServiceClient to interact with HWManager, CmdExecutor to execute linux commands,
// logrus logger, base.KubeClient and ID of a node where VolumeManager works
// Returns an instance of VolumeManager
func NewVolumeManager(client api.HWServiceClient, executor base.CmdExecutor, logger *logrus.Logger, k8sclient *base.KubeClient, nodeID string) *VolumeManager {
	vm := &VolumeManager{

		k8sclient:    k8sclient,
		hWMgrClient:  client,
		volumesCache: make(map[string]*api.Volume),
		linuxUtils:   base.NewLinuxUtils(executor, logger),
		drivesCache:  make(map[string]*drivecrd.Drive),
		nodeID:       nodeID,
		scMap: map[SCName]sc.StorageClassImplementer{
			"hdd": sc.GetHDDSCInstance(logger),
			"ssd": sc.GetSSDSCInstance(logger)},
		acProvider: common.NewACOperationsImpl(k8sclient, logger),
	}
	vm.log = logger.WithField("component", "VolumeManager")
	return vm
}

// SetExecutor sets provided CmdExecutor to LinuxUtils field of VolumeManager
func (m *VolumeManager) SetExecutor(executor base.CmdExecutor) {
	m.linuxUtils.SetExecutor(executor)
}

// Reconcile is the main Reconcile loop of VolumeManager. This loop handles creation of volumes matched to Volume CR on
// VolumeManagers's node if Volume.Spec.CSIStatus is Creating. Also this loop handles volume deletion on the node if
// Volume.Spec.CSIStatus is Removing.
// Returns reconcile result as ctrl.Result or error if something went wrong
func (m *VolumeManager) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(
		context.WithValue(context.Background(), base.RequestUUID, req.Name),
		VolumeOperationsTimeout)
	defer cancelFn()

	ll := m.log.WithFields(logrus.Fields{
		"method":   "Reconcile",
		"volumeID": req.Name,
	})

	volume := &volumecrd.Volume{}

	err := m.k8sclient.ReadCR(ctx, req.Name, volume)
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
		if err = m.k8sclient.UpdateCRWithAttempts(ctx, volume, 5); err != nil {
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
		if err = m.k8sclient.UpdateCRWithAttempts(ctx, volume, 10); err != nil {
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

// Discover inspects actual drives structs from HWManager and create volume object if partition exist on some of them
// (in case of VolumeManager restart). Updates Drives CRs based on gathered from HWManager information.
// Also this method creates AC CRs. Performs at some intervals in a goroutine
// Returns error if something went wrong during discovering
func (m *VolumeManager) Discover() error {
	m.log.WithField("method", "Discover").Info("Processing")

	ctx, cancelFn := context.WithTimeout(context.Background(), DiscoverDrivesTimeout)
	defer cancelFn()
	drivesResponse, err := m.hWMgrClient.GetDrivesList(ctx, &api.DrivesRequest{NodeId: m.nodeID})
	if err != nil {
		return err
	}
	drives := drivesResponse.Disks
	m.updateDrivesCRs(ctx, drives)

	freeDrives := m.drivesAreNotUsed()
	if err = m.updateVolumesCache(freeDrives); err != nil {
		return err
	}

	if err = m.discoverAvailableCapacity(ctx, m.nodeID); err != nil {
		return err
	}

	if !m.initialized {
		err = m.discoverLVGOnSystemDrive()
		if err != nil {
			m.log.Errorf("discoverLVGOnSystemDrive finished with error: %v", err)
		}
		m.initialized = true
	}
	return nil
}

// updateDrivesCRs updates drives cache and Drives CRs based on provided list of Drives. Tries to fill drivesCache in
// case of VolumeManager restart from Drives CRs.
// Receives golang context and slice of discovered api.Drive structs usually got from HWManager
func (m *VolumeManager) updateDrivesCRs(ctx context.Context, discoveredDrives []*api.Drive) {
	ll := m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "updateDrivesCRs",
	})
	ll.Info("Processing ...")
	m.dCacheMu.Lock()
	defer m.dCacheMu.Unlock()
	// If cache is empty try to fill it with cr from ReadList filtering by NodeID
	if len(m.drivesCache) == 0 {
		driveList := &drivecrd.DriveList{}
		if err := m.k8sclient.ReadList(ctx, driveList); err != nil {
			ll.Errorf("Failed to get disk cr list, error %s", err.Error())
		} else {
			for _, d := range driveList.Items {
				if strings.EqualFold(d.Spec.NodeId, m.nodeID) {
					m.drivesCache[d.Spec.UUID] = d.DeepCopy()
				}
			}
		}
	}
	//Try to find not existing CR for discovered drives, create it and add to cache
	for _, drivePtr := range discoveredDrives {
		exist := false
		for _, d := range m.drivesCache {
			//If drive CR already exist, try to update, if drive was changed
			if d.Spec.SerialNumber == drivePtr.SerialNumber && d.Spec.VID == drivePtr.VID && d.Spec.PID == drivePtr.PID {
				exist = true
				if !d.Equals(drivePtr) {
					drivePtr.UUID = d.Spec.UUID
					// If got from HWMgr drive's health is not equal to drive's health in cache then update appropriate
					// resources
					if drivePtr.Health != d.Spec.Health || drivePtr.Status != d.Spec.Status {
						m.handleDriveStatusChange(ctx, drivePtr)
					}
					d.Spec = *drivePtr
					if err := m.k8sclient.UpdateCR(ctx, d); err != nil {
						ll.Errorf("Failed to update drive CR with Vid/Pid/SN %s-%s-%s, error %s", d.Spec.VID, d.Spec.PID, d.Spec.SerialNumber, err.Error())
					}
					m.drivesCache[d.Spec.UUID] = d.DeepCopy()
				}
				break
			}
		}
		if !exist {
			//Drive CR is not exist, try to create it
			drivePtr.UUID = uuid.New().String()
			driveCR := m.k8sclient.ConstructDriveCR(drivePtr.UUID, *drivePtr)
			if e := m.k8sclient.CreateCR(ctx, drivePtr.UUID, driveCR); e != nil {
				ll.Errorf("Failed to create drive CR Vid/Pid/SN %s-%s-%s, error %s", driveCR.Spec.VID, driveCR.Spec.PID, driveCR.Spec.SerialNumber, e.Error())
			}
			m.drivesCache[driveCR.Spec.UUID] = driveCR
		}
	}
	//Try to find missing drive in drivesCache and update according CR
	for _, d := range m.drivesCache {
		exist := false
		for _, drive := range discoveredDrives {
			if d.Spec.SerialNumber == drive.SerialNumber && d.Spec.VID == drive.VID && d.Spec.PID == drive.PID {
				exist = true
				break
			}
		}
		if !exist {
			ll.Warnf("Set status OFFLINE for drive with Vid/Pid/SN %s/%s/%s", d.Spec.VID, d.Spec.PID, d.Spec.SerialNumber)
			d.Spec.Status = apiV1.DriveStatusOffline
			d.Spec.Health = apiV1.HealthUnknown
			err := m.k8sclient.UpdateCR(ctx, d)
			if err != nil {
				ll.Errorf("Failed to update drive CR %s, error %s", d.Name, err.Error())
			}
			m.drivesCache[d.Spec.UUID] = d.DeepCopy()
		}
	}
}

// updateVolumesCache updates volumes cache based on provided freeDrives.
// searches drives in freeDrives that are not have volume and if there are some partitions on them - try to read
// partition uuid and create volume object
func (m *VolumeManager) updateVolumesCache(freeDrives []*drivecrd.Drive) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "updateVolumesCache",
	})
	ll.Info("Processing")

	// explore each drive from freeDrives
	lsblk, err := m.linuxUtils.Lsblk("")
	if err != nil {
		return fmt.Errorf("unable to inspect system block devices via lsblk, error: %v", err)
	}

	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()
	for _, d := range freeDrives {
		for _, ld := range lsblk {
			if strings.EqualFold(ld.Serial, d.Spec.SerialNumber) && len(ld.Children) > 0 {
				uuid, err := m.linuxUtils.GetPartitionUUID(ld.Name)
				if err != nil {
					ll.Warnf("Unable to determine partition UUID for device %s, error: %v", ld.Name, err)
					continue
				}
				size, err := strconv.ParseInt(ld.Size, 10, 64)
				if err != nil {
					ll.Warnf("Unable parse string %s to int, for device %s, error: %v", ld.Size, ld.Name, err)
					continue
				}
				v := &api.Volume{
					Id:           uuid,
					Size:         size,
					Location:     d.Spec.UUID,
					LocationType: apiV1.LocationTypeDrive,
					Mode:         apiV1.ModeFS,
					Type:         ld.FSType,
					Health:       d.Spec.Health,
					CSIStatus:    "",
				}
				ll.Infof("Add in cache volume: %v", v)
				m.volumesCache[v.Id] = v
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

	ll.Infof("Starting discovering Available Capacity with %d drives in cache", len(m.drivesCache))

	wasError := false

	for _, drive := range m.drivesCache {
		if drive.Spec.Health == apiV1.HealthGood && drive.Spec.Status == apiV1.DriveStatusOnline {
			removed := false
			for _, volume := range m.volumesCache {
				// if drive contains volume then available capacity for this drive shouldn't exist
				if strings.EqualFold(volume.Location, drive.Spec.UUID) {
					ll.Infof("Drive %s is occupied by volume %s", drive.Spec.UUID, volume.Id)
					removed = true
				}
			}
			// Don't create ACs with devices which are used by LVG
			lvgList := &lvgcrd.LVGList{}
			if err := m.k8sclient.ReadList(ctx, lvgList); err != nil {
				ll.Errorf("Failed to get LVG CR list, error %v", err)
				wasError = true
			} else {
				for _, lvg := range lvgList.Items {
					if base.ContainsString(lvg.Spec.Locations, drive.Spec.UUID) {
						removed = true
					}
				}
			}
			// If drive isn't used by Volume then try to create AC from it
			if !removed {
				capacity := &api.AvailableCapacity{
					Size:         drive.Spec.Size,
					Location:     drive.Spec.UUID,
					StorageClass: base.ConvertDriveTypeToStorageClass(drive.Spec.Type),
					NodeId:       nodeID,
				}

				name := capacity.NodeId + "-" + strings.ToLower(capacity.Location)

				if err := m.k8sclient.ReadCR(context.WithValue(ctx, base.RequestUUID, name), name,
					&accrd.AvailableCapacity{}); err != nil {
					if k8sError.IsNotFound(err) {
						newAC := m.k8sclient.ConstructACCR(name, *capacity)
						ll.Infof("Creating Available Capacity %v", newAC)
						if err := m.k8sclient.CreateCR(context.WithValue(ctx, base.RequestUUID, name),
							name, newAC); err != nil {
							ll.Errorf("Error during CreateAvailableCapacity request to k8s: %v, error: %v",
								capacity, err)
							wasError = true
						}
					} else {
						ll.Errorf("Unable to read Available Capacity %s, error: %v", name, err)
						wasError = true
					}
				}
			}
		}
	}

	if wasError {
		return errors.New("not all available capacity were created")
	}

	return nil
}

// drivesAreNotUsed search drives in drives cache that isn't have any volumes
// Returns slice of drivecrd.Drive structs
func (m *VolumeManager) drivesAreNotUsed() []*drivecrd.Drive {
	ll := m.log.WithFields(logrus.Fields{
		"method": "drivesAreNotUsed",
	})
	ll.Info("Processing")
	// search drives that don't have parent volume
	drives := make([]*drivecrd.Drive, 0)
	for _, d := range m.drivesCache {
		isUsed := false
		for _, v := range m.volumesCache {
			// expect only Drive LocationType, for Drive LocationType Location will be a UUID of the drive
			if d.Spec.Type != apiV1.DriveTypeNVMe &&
				v.LocationType == apiV1.LocationTypeDrive &&
				strings.EqualFold(d.Spec.UUID, v.Location) {
				isUsed = true
				ll.Infof("Found volume with ID \"%s\" in cache for drive with UUID \"%s\"",
					v.Id, d.Spec.UUID)
				break
			}
		}
		if !isUsed {
			drives = append(drives, d)
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
	if err = m.k8sclient.ReadList(context.Background(), &lvgList); err != nil {
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
	devices, err := m.linuxUtils.Lsblk(rootMountPoint)
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
		vgCR = m.k8sclient.ConstructLVGCR(vgCRName, vg)
		ctx  = context.WithValue(context.Background(), base.RequestUUID, vg.Name)
	)
	if err = m.k8sclient.CreateCR(ctx, vg.Name, vgCR); err != nil {
		return fmt.Errorf("unable to create LVG CR %v, error: %v", vgCR, err)
	}

	if vgFreeSpace > common.AcSizeMinThresholdBytes {
		acName := uuid.New().String()
		acCR := m.k8sclient.ConstructACCR(acName, api.AvailableCapacity{
			Location:     vgCRName,
			NodeId:       m.nodeID,
			StorageClass: apiV1.StorageClassSSDLVG,
			Size:         vgFreeSpace,
		})
		if err = m.k8sclient.CreateCR(ctx, acName, acCR); err != nil {
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

	// TODO: should read from Volume CRD AK8S-170
	if v, ok := m.getFromVolumeCache(vol.Id); ok {
		ll.Infof("Found volume in cache with status: %s", m.getVolumeStatus(v.CSIStatus))
		return m.pullCreateLocalVolume(ctx, vol.Id)
	}

	var (
		volLocation = vol.Location
		newStatus   = apiV1.Created
		scImpl      = m.getStorageClassImpl(vol.StorageClass)
		device      string
		err         error
	)
	switch vol.StorageClass {
	//TODO AK8S-762 Use createPartitionAndSetUUID for SSDLVG, HDDLVG SC in CreateLocalVolume
	case apiV1.StorageClassSSDLVG, apiV1.StorageClassHDDLVG:
		sizeStr := fmt.Sprintf("%.2fG", float64(vol.Size)/float64(base.GBYTE))
		vgName := vol.Location

		// Volume.Location is a LVG CR name and we use such name as a real VG name
		// however for LVG based on system disk LVG CR name != VG name
		// we need to read appropriate LVG CR and use LVG CR.Spec.Name in LVCreate command
		if vol.StorageClass == apiV1.StorageClassSSDLVG {
			vgName, err = m.k8sclient.GetVGNameByLVGCRName(ctx, volLocation)
			if err != nil {
				return fmt.Errorf("unable to find LVG name by LVG CR name: %v", err)
			}
		}

		m.setVolumeCacheValue(vol.Id, vol)

		// create lv with name /dev/VG_NAME/vol.Id
		ll.Infof("Creating LV %s sizeof %s in VG %s", vol.Id, sizeStr, vgName)
		if err = m.linuxUtils.LVCreate(vol.Id, sizeStr, vgName); err != nil {
			ll.Errorf("Unable to create LV: %v", err)
			return err
		}

		ll.Info("LV was created.")

		go func() {
			if err := scImpl.CreateFileSystem(sc.XFS, fmt.Sprintf("/dev/%s/%s", vgName, vol.Id)); err != nil {
				ll.Error("Failed to create file system, set volume status FailedToCreate", err)
				newStatus = apiV1.Failed
			}
			m.setVolumeStatus(vol.Id, newStatus)
		}()

	default: // assume that volume location is whole disk
		drive := &drivecrd.Drive{}

		// read Drive CR based on Volume.Location (vol.Location == Drive.UUID == Drive.Name)
		if err = m.k8sclient.ReadCR(ctx, volLocation, drive); err != nil {
			ll.Errorf("Failed to read crd with name %s, error %s", volLocation, err.Error())
			return err
		}

		ll.Infof("Search device file for drive with S/N %s", drive.Spec.SerialNumber)
		device, err = m.linuxUtils.SearchDrivePath(drive)
		if err != nil {
			return err
		}

		m.setVolumeCacheValue(vol.Id, vol)
		go func() {
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
					if err := m.k8sclient.UpdateCR(ctx, drive); err != nil {
						ll.Errorf("Failed to update drive CRd with name %s, error %s", drive.Name, err.Error())
					}
				}
				ll.Errorf("Failed to set partition UUID: %v, set volume status to Failed", err)
				m.setVolumeStatus(vol.Id, apiV1.Failed)
			} else {
				ll.Info("Partition was created successfully")
				newStatus := apiV1.Created
				// TODO AK8S-632 Make CreateFileSystem work with different type of file systems
				if err := scImpl.CreateFileSystem(sc.XFS, partition); err != nil {
					ll.Error("Failed to create file system, set volume status FailedToCreate", err)
					newStatus = apiV1.Failed
				}
				m.setVolumeStatus(id, newStatus)
			}
		}()
	}
	return m.pullCreateLocalVolume(ctx, vol.Id)
}

// pullCreateLocalVolume pulls volume's CSIStatus each second. Waits for Created or Failed state. Uses for non-blocking
// execution of CreateLocalVolume.
// Returns error if volume's CSIStatus became Failed or if provided context was done
func (m *VolumeManager) pullCreateLocalVolume(ctx context.Context, volumeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "pullCreateLocalVolume",
		"volumeID": volumeID,
	})
	ll.Infof("Pulling status, current: %s", m.getVolumeStatus(volumeID))

	var (
		currStatus string
		vol        *api.Volume
	)
	for {
		select {
		case <-ctx.Done():
			ll.Errorf("Context was closed set volume %s status to FailedToCreate", vol.Location)
			m.setVolumeStatus(volumeID, apiV1.Failed)
		case <-time.After(time.Second):
			vol, _ = m.getFromVolumeCache(volumeID)
			currStatus = vol.CSIStatus
			switch currStatus {
			case apiV1.Creating:
				ll.Info("Volume is in Creating state, continue pulling")
			case apiV1.Created:
				ll.Info("Volume was became Created, return it")
				return nil
			case apiV1.Failed:
				ll.Info("Volume was became failed, return it and try to restore.")
				return fmt.Errorf("unable to create local volume %s size of %d", vol.Id, vol.Size)
			}
		}
	}
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
		err    error
		device string
	)
	switch volume.StorageClass {
	case apiV1.StorageClassHDD, apiV1.StorageClassSSD:
		m.dCacheMu.Lock()
		drive := m.drivesCache[volume.Location]
		m.dCacheMu.Unlock()
		if drive == nil {
			return errors.New("unable to find drive by volume location")
		}
		// get device path
		device, err = m.linuxUtils.SearchDrivePath(drive)
		if err != nil {
			return fmt.Errorf("unable to find device for drive with S/N %s", volume.Location)
		}

		err = m.linuxUtils.DeletePartition(device)
		if err != nil {
			wErr := fmt.Errorf("failed to delete partition, error: %v", err)
			ll.Errorf("%v, set CSI status - failed", wErr)
			m.setVolumeStatus(volume.Id, apiV1.Failed)
			return wErr
		}
		ll.Info("Partition was deleted")
	case apiV1.StorageClassSSDLVG, apiV1.StorageClassHDDLVG:
		vgName := volume.Location
		var err error
		// Volume.Location is a LVG CR however for LVG based on system disk LVG CR name != VG name
		// we need to read appropriate LVG CR and use LVG CR.Spec.Name as VG name
		if volume.StorageClass == apiV1.StorageClassSSDLVG {
			vgName, err = m.k8sclient.GetVGNameByLVGCRName(ctx, volume.Location)
			if err != nil {
				return fmt.Errorf("unable to find LVG name by LVG CR name: %v", err)
			}
		}
		device = fmt.Sprintf("/dev/%s/%s", vgName, volume.Id) // /dev/VG_NAME/LV_NAME
	default:
		return fmt.Errorf("unable to determine storage class for volume %v", volume)
	}

	ll.Infof("Found device file %s", device)
	scImpl := m.getStorageClassImpl(volume.StorageClass)

	if err = scImpl.DeleteFileSystem(device); err != nil {
		wErr := fmt.Errorf("failed to wipefs device, error: %v", err)
		ll.Errorf("%v, set CSI status - failed", wErr)
		m.setVolumeStatus(volume.Id, apiV1.Failed)
		return wErr
	}
	ll.Info("File system was deleted")

	if volume.StorageClass == apiV1.StorageClassHDDLVG || volume.StorageClass == apiV1.StorageClassSSDLVG {
		lvgName := volume.Location
		ll.Infof("Removing LV %s from LVG %s", volume.Id, lvgName)
		if err = m.linuxUtils.LVRemove(device); err != nil {
			m.setVolumeStatus(volume.Id, apiV1.Failed)
			return fmt.Errorf("unable to remove lv: %v", err)
		}
	}
	m.deleteFromVolumeCache(volume.Id)

	ll.Info("Volume was successfully deleted")
	return nil
}

// getFromVolumeCache returns api.Volume from volumeCache of VolumeManager by a provided key
// Returns an instance of api.Volume struct and bool that shows the existence of volume in cache
// TODO: remove that methods when AK8S-170 will be closed
func (m *VolumeManager) getFromVolumeCache(key string) (*api.Volume, bool) {
	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()

	v, ok := m.volumesCache[key]
	return v, ok
}

// deleteFromVolumeCache deletes api.Volume from volumeCache of VolumeManager by a provided key
func (m *VolumeManager) deleteFromVolumeCache(key string) {
	m.vCacheMu.Lock()
	delete(m.volumesCache, key)
	m.vCacheMu.Unlock()
}

// setVolumeCacheValue sets api.Volume from volumeCache of VolumeManager by a provided key
// Receives key which is volumeID and api.Volume that would be set as value for that key
func (m *VolumeManager) setVolumeCacheValue(key string, v *api.Volume) {
	m.vCacheMu.Lock()
	m.volumesCache[key] = v
	m.vCacheMu.Unlock()
}

// getVolumeStatus returns CSIStatus of the volume from volumeCache of VolumeManager by a provided key
// Returns CSIStatus of the volume as a string
func (m *VolumeManager) getVolumeStatus(key string) string {
	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()

	v, ok := m.volumesCache[key]
	if !ok {
		m.log.WithField("method", "getVolumeStatus").Errorf("Unable to find volume with ID %s in cache", key)
		return ""
	}
	return v.CSIStatus
}

// setVolumeStatus sets CSIStatus of the volume from volumeCache of VolumeManager by a provided key
// Receives key which is volume ID and newStatus which is CSIStatus
func (m *VolumeManager) setVolumeStatus(key string, newStatus string) {
	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()

	v, ok := m.volumesCache[key]
	if !ok {
		m.log.WithField("method", "setVolumeStatus").Errorf("Unable to find volume with ID %s in cache", key)
		return
	}
	v.CSIStatus = newStatus
	m.volumesCache[key] = v
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

// addVolumeOwner tries to add owner to Volume's CR Owners slice with retries
// Receives volumeID of the volume where the owner should be added and a podName that will be used as owner
// Returns error if something went wrong
func (m *VolumeManager) addVolumeOwner(volumeID string, podName string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "addVolumeOwner",
		"volumeID": volumeID,
	})

	var (
		v        = &volumecrd.Volume{}
		attempts = 10
		ctx      = context.WithValue(context.Background(), base.RequestUUID, volumeID)
	)

	ll.Infof("Try to add owner as a pod name %s", podName)

	if err := m.k8sclient.ReadCRWithAttempts(volumeID, v, attempts); err != nil {
		ll.Errorf("failed to read volume cr after %d attempts", attempts)
	}

	owners := v.Spec.Owners

	podNameExists := base.ContainsString(owners, podName)

	if !podNameExists {
		owners = append(owners, podName)
		v.Spec.Owners = owners
	}

	if err := m.k8sclient.UpdateCRWithAttempts(ctx, v, attempts); err == nil {
		return nil
	}

	ll.Warnf("Unable to update volume CR's owner %s.", podName)

	return fmt.Errorf("unable to persist owner to %s for volume %s", podName, volumeID)
}

// clearVolumeOwners tries to clear owners slice in Volume's CR spec
// Receives volumeID whose owners must be cleared
// Returns error if something went wrong
func (m *VolumeManager) clearVolumeOwners(volumeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "ClearVolumeOwners",
		"volumeID": volumeID,
	})

	var (
		v        = &volumecrd.Volume{}
		attempts = 10
		ctx      = context.WithValue(context.Background(), base.RequestUUID, volumeID)
	)

	ll.Infof("Try to clear owner fieild")

	if err := m.k8sclient.ReadCRWithAttempts(volumeID, v, attempts); err != nil {
		ll.Errorf("failed to read volume cr after %d attempts", attempts)
	}

	v.Spec.Owners = nil

	if err := m.k8sclient.UpdateCRWithAttempts(ctx, v, attempts); err == nil {
		return nil
	}

	ll.Warnf("Unable to clear volume CR's owners")

	return fmt.Errorf("unable to clear volume CR's owners for volume %s", volumeID)
}

// handleDriveStatusChange removes AC that is based on unhealthy drive, returns AC if drive returned to healthy state,
// mark volumes of the unhealthy drive as unhealthy.
// Receives golang context and api.Drive that should be handled
func (m *VolumeManager) handleDriveStatusChange(ctx context.Context, drive *api.Drive) {
	ll := m.log.WithFields(logrus.Fields{
		"method":  "handleDriveStatusChange",
		"driveID": drive.UUID,
	})

	ll.Infof("The new drive status from HWMgr is %s", drive.Health)

	// Handle resources without LVG
	// Remove AC based on disk with health BAD, SUSPECT, UNKNOWN
	if drive.Health != apiV1.HealthGood || drive.Status == apiV1.DriveStatusOffline {
		ac := m.getACByLocation(ctx, drive.UUID)
		if ac != nil {
			ll.Infof("Removing AC %s based on unhealthy location %s", ac.Name, ac.Spec.Location)
			if err := m.k8sclient.DeleteCR(ctx, ac); err != nil {
				ll.Errorf("Failed to delete unhealthy available capacity CR: %v", err)
			}
		}
	}

	// Set disk's health status to volume CR
	vol := m.getVolumeByLocation(ctx, drive.UUID)
	if vol != nil {
		ll.Infof("Setting updated status %s to volume %s", drive.Health, vol.Name)
		vol.Spec.Health = drive.Health
		if err := m.k8sclient.UpdateCR(ctx, vol); err != nil {
			ll.Errorf("Failed to update volume CR's health status: %v", err)
		}
	}

	// Handle resources with LVG
	// This is not work for the current moment because HAL doesn't monitor disks with LVM
	// TODO AK8S-472 Handle disk health which are used by LVGs
}

// getACByLocation reads the whole list of AC CRs from a cluster and searches the AC with provided location
// Receive golang context and location name which should be equal to AvailableCapacity.Spec.Location
// Returns an instance of accrd.AvailableCapacity
func (m *VolumeManager) getACByLocation(ctx context.Context, location string) *accrd.AvailableCapacity {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "getACByLocation",
		"location": location,
	})

	acList := &accrd.AvailableCapacityList{}
	if err := m.k8sclient.ReadList(ctx, acList); err != nil {
		ll.Errorf("Failed to get available capacity CR list, error %v", err)
		return nil
	}

	for _, ac := range acList.Items {
		if strings.EqualFold(ac.Spec.Location, location) {
			return &ac
		}
	}

	ll.Infof("Can't find AC assigned to provided location")

	return nil
}

// getVolumeByLocation reads the whole list of Volume CRs from a cluster and searches the volume with provided location
// Receive golang context and location name which should be equal to Volume.Spec.Location
// Returns an instance of volumecrd.Volume
func (m *VolumeManager) getVolumeByLocation(ctx context.Context, location string) *volumecrd.Volume {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "getVolumeByLocation",
		"location": location,
	})

	volList := &volumecrd.VolumeList{}
	if err := m.k8sclient.ReadList(ctx, volList); err != nil {
		ll.Errorf("Failed to get volume CR list, error %v", err)
		return nil
	}

	for _, v := range volList.Items {
		if strings.EqualFold(v.Spec.Location, location) {
			return &v
		}
	}

	ll.Infof("Can't find VolumeCR assigned to provided location")

	return nil
}

// SetSCImplementer sets sc.StorageClassImplementer implementation to scMap of VolumeManager
// Receives scName which uses as a key and an instance of sc.StorageClassImplementer
func (m *VolumeManager) SetSCImplementer(scName string, implementer sc.StorageClassImplementer) {
	m.scMap[SCName(scName)] = implementer
}
