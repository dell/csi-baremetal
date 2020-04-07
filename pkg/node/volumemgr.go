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
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
)

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

	initialized bool
}

const (
	DiscoverDrivesTimout    = 300 * time.Second
	VolumeOperationsTimeout = 300 * time.Second
)

// NewVolumeManager returns new instance ov VolumeManager
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
	}
	vm.log = logger.WithField("component", "VolumeManager")
	return vm
}

func (m *VolumeManager) SetExecutor(executor base.CmdExecutor) {
	m.linuxUtils.SetExecutor(executor)
}

// Reconcile Volume CRD according to stasus which set by CSI Controller Service
func (m *VolumeManager) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), VolumeOperationsTimeout)
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
	var newStatus api.OperationalStatus
	switch volume.Spec.Status {
	case api.OperationalStatus_Creating:
		err := m.CreateLocalVolume(ctx, &volume.Spec)
		if err != nil {
			ll.Errorf("Unable to create volume size of %d bytes. Error: %v. Context Error: %v."+
				" Set volume status to FailedToCreate", volume.Spec.Size, err, ctx.Err())
			newStatus = api.OperationalStatus_FailedToCreate
		} else {
			ll.Infof("CreateLocalVolume completed successfully. Set status to Created")
			newStatus = api.OperationalStatus_Created
		}

		if err = m.k8sclient.ChangeVolumeStatus(volume.Name, newStatus); err != nil {
			ll.Error(err.Error())
			// Here we can return error because Volume created successfully and we can try to change CR's status
			// one more time
			return ctrl.Result{}, err
		}
		// If we return err here, we'll call that reconcile one more time.
		// But we set OperationalStatus as FailedToCreate. And CSI Controller handles with FailedToCreate.
		return ctrl.Result{}, nil
	case api.OperationalStatus_Removing:
		if err = m.DeleteLocalVolume(ctx, &volume.Spec); err != nil {
			ll.Errorf("Failed to delete volume - %s. Error: %v. Context Error: %v. "+
				"Set status FailToRemove", volume.Spec.Id, err, ctx.Err())
			newStatus = api.OperationalStatus_FailToRemove
		} else {
			ll.Infof("Volume - %s was successfully removed. Set status to Removed", volume.Spec.Id)
			newStatus = api.OperationalStatus_Removed
		}
		if err = m.k8sclient.ChangeVolumeStatus(volume.Name, newStatus); err != nil {
			ll.Errorf("Unable to set new status for volume: %v", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, nil
	}
}

func (m *VolumeManager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&volumecrd.Volume{}).
		Complete(m)
}

// Discover inspects drives and create volume object if partition exist.
// Also this method creates AC CRs
func (m *VolumeManager) Discover() error {
	m.log.WithField("method", "Discover").Info("Processing")

	ctx, cancelFn := context.WithTimeout(context.Background(), DiscoverDrivesTimout)
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
		m.initialized = true
	}
	return nil
}

// updateDrivesCRs updates drives cache based on provided list of Drives
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
			if e := m.k8sclient.CreateCR(ctx, driveCR, drivePtr.UUID); e != nil {
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
			d.Spec.Status = api.Status_OFFLINE
			d.Spec.Health = api.Health_UNKNOWN
			err := m.k8sclient.UpdateCR(ctx, d)
			if err != nil {
				ll.Errorf("Failed to update drive CR %s, error %s", d.Name, err.Error())
			}
			m.drivesCache[d.Spec.UUID] = d.DeepCopy()
		}
	}
}

// updateVolumesCache updates volumes cache based on provided freeDrives
// search drives in freeDrives that are not have volume and if there are
// some partitions on them - try to read partition uuid and create volume object
func (m *VolumeManager) updateVolumesCache(freeDrives []*drivecrd.Drive) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "updateVolumesCache",
	})
	ll.Info("Processing")

	// explore each drive from freeDrives
	lsblk, err := m.linuxUtils.Lsblk(base.DriveTypeDisk)
	if err != nil {
		return fmt.Errorf("unable to inspect system block devices via lsblk, error: %v", err)
	}

	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()
	for _, d := range freeDrives {
		for _, ld := range *lsblk {
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
					LocationType: api.LocationType_Drive,
					Mode:         api.Mode_FS,
					Type:         ld.FSType,
					Health:       d.Spec.Health,
					Status:       api.OperationalStatus_Operative,
				}
				ll.Infof("Add in cache volume: %v", v)
				m.volumesCache[v.Id] = v
			}
		}
	}
	return nil
}

// DiscoverAvailableCapacity inspect current available capacity on nodes and fill AC CRs
func (m *VolumeManager) discoverAvailableCapacity(ctx context.Context, nodeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method": "discoverAvailableCapacity",
	})

	ll.Infof("Starting discovering Available Capacity with %d drives in cache", len(m.drivesCache))

	wasError := false

	for _, drive := range m.drivesCache {
		if drive.Spec.Health == api.Health_GOOD && drive.Spec.Status == api.Status_ONLINE {
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
							newAC, name); err != nil {
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
			if d.Spec.Type != api.DriveType_NVMe &&
				v.LocationType == api.LocationType_Drive &&
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

func (m *VolumeManager) CreateLocalVolume(ctx context.Context, vol *api.Volume) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "CreateLocalVolume",
		"volumeID": vol.Id,
	})

	ll.Infof("Creating volume: %v", vol)

	// TODO: should read from Volume CRD AK8S-170
	if v, ok := m.getFromVolumeCache(vol.Id); ok {
		ll.Infof("Found volume in cache with status: %s", api.OperationalStatus_name[int32(v.Status)])
		return m.pullCreateLocalVolume(ctx, vol.Id)
	}

	var (
		volLocation = vol.Location
		device      string
		err         error
	)
	switch vol.StorageClass {
	case api.StorageClass_SSDLVG, api.StorageClass_HDDLVG:
		sizeStr := fmt.Sprintf("%.2fG", float64(vol.Size)/float64(base.GBYTE))
		// create lv with name /dev/VG_NAME/vol.Id
		ll.Infof("Creating LV %s sizeof %s in VG %s", vol.Id, sizeStr, volLocation)
		if err = m.linuxUtils.LVCreate(vol.Id, sizeStr, volLocation); err != nil {
			ll.Errorf("Unable to create LV: %v", err)
			return err
		}

		ll.Info("LV was created.")
		m.setVolumeCacheValue(vol.Id, vol)
		m.setVolumeStatus(vol.Id, api.OperationalStatus_Created)
		return nil
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
			rollBacked, err := m.setPartitionUUIDForDev(device, vol.Id)
			if err != nil {
				if !rollBacked {
					ll.Errorf("unable set partition uuid for dev %s, roll back failed too, set drive status to OFFLINE", device)
					drive.Spec.Status = api.Status_OFFLINE
					if err := m.k8sclient.UpdateCR(ctx, drive); err != nil {
						ll.Errorf("Failed to update drive CRd with name %s, error %s", drive.Name, err.Error())
					}
				}
				ll.Errorf("Failed to set partition UUID: %v, set volume status to FailedToCreate", err)
				m.setVolumeStatus(vol.Id, api.OperationalStatus_FailedToCreate)
			} else {
				ll.Info("Partition UUID was set successfully, set volume status to Created")
				m.setVolumeStatus(vol.Id, api.OperationalStatus_Created)
			}
		}()
	}

	return m.pullCreateLocalVolume(ctx, vol.Id)
}

func (m *VolumeManager) pullCreateLocalVolume(ctx context.Context, volumeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "pullCreateLocalVolume",
		"volumeID": volumeID,
	})
	ll.Infof("Pulling status, current: %s", api.OperationalStatus_name[int32(m.getVolumeStatus(volumeID))])

	var (
		currStatus api.OperationalStatus
		vol        *api.Volume
	)
	for {
		select {
		case <-ctx.Done():
			ll.Errorf("Context was closed set volume %s status to FailedToCreate", vol.Location)
			m.setVolumeStatus(volumeID, api.OperationalStatus_FailedToCreate)
		case <-time.After(time.Second):
			vol, _ = m.getFromVolumeCache(volumeID)
			currStatus = vol.Status
			switch currStatus {
			case api.OperationalStatus_Creating:
				{
					ll.Info("Volume is in Creating state, continue pulling")
				}
			case api.OperationalStatus_Created:
				{
					ll.Info("Volume was became Created, return it")
					return nil
				}
			case api.OperationalStatus_FailedToCreate:
				{
					ll.Info("Volume was became FailedToCreate, return it and try to restrore.")
					return fmt.Errorf("unable to create local volume %s size of %d", vol.Id, vol.Size)
				}
			}
		}
	}
}

// setPartitionUUIDForDev creates partition and sets partition UUID, if some step fails
// will try to rollback operation, returns error and roll back operation status (bool)
// if error occurs, status value will show whether device has roll back to the initial state
func (m *VolumeManager) setPartitionUUIDForDev(device string, uuid string) (rollBacked bool, err error) {
	ll := m.log.WithFields(logrus.Fields{
		"method": "setPartitionUUIDForDev",
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
			return false, fmt.Errorf("partition has already exist on device %s", device)
		}
		if currUUID == uuid {
			ll.Infof("Partition has already set.")
			return true, nil
		}
		return false, fmt.Errorf("partition has already exist on device %s", device)
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
	return rollBacked, err
}

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
	case api.StorageClass_HDD, api.StorageClass_SSD:
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
			ll.Errorf("%v, set operational status - fail to remove", wErr)
			m.setVolumeStatus(volume.Id, api.OperationalStatus_FailToRemove)
			return wErr
		}
		ll.Info("Partition was deleted")
	case api.StorageClass_SSDLVG, api.StorageClass_HDDLVG:
		device = fmt.Sprintf("/dev/%s/%s", volume.Location, volume.Id) // /dev/VG_NAME/LV_NAME
	default:
		return fmt.Errorf("unable to determine storage class for volume %v", volume)
	}

	ll.Infof("Found device file %s", device)
	scImpl := m.getStorageClassImpl(volume.StorageClass)

	if err = scImpl.DeleteFileSystem(device); err != nil {
		wErr := fmt.Errorf("failed to wipefs device, error: %v", err)
		ll.Errorf("%v, set operational status - fail to remove", wErr)
		m.setVolumeStatus(volume.Id, api.OperationalStatus_FailToRemove)
		return wErr
	}
	ll.Info("File system was deleted")

	if volume.StorageClass == api.StorageClass_HDDLVG || volume.StorageClass == api.StorageClass_SSDLVG {
		lvgName := volume.Location
		ll.Infof("Removing LV %s from LVG %s", volume.Id, lvgName)
		if err = m.linuxUtils.LVRemove(volume.Id, volume.Location); err != nil {
			m.setVolumeStatus(volume.Id, api.OperationalStatus_FailToRemove)
			return fmt.Errorf("unable to remove lv: %v", err)
		}
	}
	m.deleteFromVolumeCache(volume.Id)

	ll.Info("Volume was successfully deleted")
	return nil
}

// TODO: remove that methods when AK8S-170 will be closed
func (m *VolumeManager) getFromVolumeCache(key string) (*api.Volume, bool) {
	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()

	v, ok := m.volumesCache[key]
	return v, ok
}

func (m *VolumeManager) deleteFromVolumeCache(key string) {
	m.vCacheMu.Lock()
	delete(m.volumesCache, key)
	m.vCacheMu.Unlock()
}

func (m *VolumeManager) setVolumeCacheValue(key string, v *api.Volume) {
	m.vCacheMu.Lock()
	m.volumesCache[key] = v
	m.vCacheMu.Unlock()
}

func (m *VolumeManager) getVolumeStatus(key string) api.OperationalStatus {
	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()

	v, ok := m.volumesCache[key]
	if !ok {
		m.log.WithField("method", "getVolumeStatus").Errorf("Unable to find volume with ID %s in cache", key)
		return 17
	}
	return v.Status
}

func (m *VolumeManager) setVolumeStatus(key string, newStatus api.OperationalStatus) {
	m.vCacheMu.Lock()
	defer m.vCacheMu.Unlock()

	v, ok := m.volumesCache[key]
	if !ok {
		m.log.WithField("method", "setVolumeStatus").Errorf("Unable to find volume with ID %s in cache", key)
		return
	}
	v.Status = newStatus
	m.volumesCache[key] = v
}

// Return appropriate StorageClass implementation from VolumeManager scMap field
func (m *VolumeManager) getStorageClassImpl(storageClass api.StorageClass) sc.StorageClassImplementer {
	switch storageClass {
	case api.StorageClass_HDD:
		return m.scMap[SCName("hdd")]
	case api.StorageClass_SSD:
		return m.scMap[SCName("ssd")]
	default:
		return m.scMap[SCName("hdd")]
	}
}

// addVolumeOwner tries to add owner to Volume's CR Owners slice with retries
func (m *VolumeManager) addVolumeOwner(volumeID string, podName string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "addVolumeOwner",
		"volumeID": volumeID,
	})

	var (
		v        = &volumecrd.Volume{}
		attempts = 10
	)

	ll.Infof("Try to add owner as a pod name %s", podName)

	if err := m.k8sclient.ReadVolumeCRWithAttempts(volumeID, v, attempts); err != nil {
		ll.Errorf("failed to read volume cr after %d attempts", attempts)
	}

	owners := v.Spec.Owners

	podNameExists := base.ContainsString(owners, podName)

	if !podNameExists {
		owners = append(owners, podName)
		v.Spec.Owners = owners
	}

	if err := m.k8sclient.UpdateVolumeCRWithAttempts(v, attempts); err == nil {
		return nil
	}

	ll.Warnf("Unable to update volume CR's owner %s.", podName)

	return fmt.Errorf("unable to persist owner to %s for volume %s", podName, volumeID)
}

// clearVolumeOwners tries to clear owners slice in Volume's CR spec
func (m *VolumeManager) clearVolumeOwners(volumeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "ClearVolumeOwners",
		"volumeID": volumeID,
	})

	var (
		v        = &volumecrd.Volume{}
		attempts = 10
	)

	ll.Infof("Try to clear owner fieild")

	if err := m.k8sclient.ReadVolumeCRWithAttempts(volumeID, v, attempts); err != nil {
		ll.Errorf("failed to read volume cr after %d attempts", attempts)
	}

	v.Spec.Owners = nil

	if err := m.k8sclient.UpdateVolumeCRWithAttempts(v, attempts); err == nil {
		return nil
	}

	ll.Warnf("Unable to clear volume CR's owners")

	return fmt.Errorf("unable to clear volume CR's owners for volume %s", volumeID)
}

func (m *VolumeManager) handleDriveStatusChange(ctx context.Context, drive *api.Drive) {
	ll := m.log.WithFields(logrus.Fields{
		"method":  "handleDriveStatusChange",
		"driveID": drive.UUID,
	})

	ll.Infof("The new drive status from HWMgr is %s", drive.Health.String())

	// Handle resources without LVG
	// Remove AC based on disk with health BAD, SUSPECT, UNKNOWN
	if drive.Health != api.Health_GOOD || drive.Status == api.Status_OFFLINE {
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
		ll.Infof("Setting updated status %s to volume %s", drive.Health.String(), vol.Name)
		vol.Spec.Health = drive.Health
		if err := m.k8sclient.UpdateCR(ctx, vol); err != nil {
			ll.Errorf("Failed to update volume CR's health status: %v", err)
		}
	}

	// Handle resources with LVG
	// This is not work for the current moment because HAL doesn't monitor disks with LVM
	// TODO AK8S-472 Handle disk health which are used by LVGs
}

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
