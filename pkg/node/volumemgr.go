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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
)

type VolumeManager struct {
	k8sclient *base.KubeClient

	availableCapacityCache map[string]*api.AvailableCapacity
	drivesCache            map[string]*drivecrd.Drive
	acCacheMu              sync.Mutex

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
}

const (
	DiscoverDrivesTimout     = 300 * time.Second
	CreateLocalVolumeTimeout = 300 * time.Second
)

// NewVolumeManager returns new instance ov VolumeManager
func NewVolumeManager(client api.HWServiceClient, executor base.CmdExecutor, logger *logrus.Logger, k8sclient *base.KubeClient, nodeID string) *VolumeManager {
	vm := &VolumeManager{

		k8sclient:              k8sclient,
		hWMgrClient:            client,
		volumesCache:           make(map[string]*api.Volume),
		linuxUtils:             base.NewLinuxUtils(executor, logger),
		drivesCache:            make(map[string]*drivecrd.Drive),
		availableCapacityCache: make(map[string]*api.AvailableCapacity),
		nodeID:                 nodeID,
		scMap: map[SCName]sc.StorageClassImplementer{"hdd": sc.GetHDDSCInstance(logger),
			"ssd": sc.GetSSDSCInstance(logger)},
	}
	vm.log = logger.WithField("component", "VolumeManager")
	return vm
}

func (m *VolumeManager) SetExecutor(executor base.CmdExecutor) {
	m.linuxUtils.SetLinuxUtilsExecutor(executor)
}

func (m *VolumeManager) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), CreateLocalVolumeTimeout)
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
	if volume.Spec.Owner != m.nodeID {
		return ctrl.Result{}, nil
	}

	ll.Info("Reconciling Volume")
	switch volume.Spec.Status {
	case api.OperationalStatus_Creating:
		err := m.CreateLocalVolume(ctx, &volume.Spec)
		var newStatus api.OperationalStatus
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
	default:
		return ctrl.Result{}, nil
	}
}

func (m *VolumeManager) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&volumecrd.Volume{}).
		Complete(m)
}

// GetAvailableCapacity request return array of free capacity on node
func (m *VolumeManager) GetAvailableCapacity(ctx context.Context, req *api.AvailableCapacityRequest) (*api.AvailableCapacityResponse, error) {
	m.log.WithField("method", "GetAvailableCapacity").Info("Processing ...")

	// TODO Make CSI Controller wait til drives cache is empty AK8S-379
	if len(m.drivesCache) == 0 {
		return nil, fmt.Errorf("drives cache has not initialized yet")
	}

	if err := m.DiscoverAvailableCapacity(req.NodeId); err != nil {
		return nil, err
	}
	ac := make([]*api.AvailableCapacity, len(m.availableCapacityCache))
	i := 0
	for _, item := range m.availableCapacityCache {
		ac[i] = item
		i++
	}
	return &api.AvailableCapacityResponse{AvailableCapacity: ac}, nil
}

// Discover inspects drives and create volume object if partition exist
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
	return m.updateVolumesCache(freeDrives) // lock vCacheMu
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
			ll.Warnf("Set status OFFLINE for drive with Vid/Pid/SN %s-%s-%s", d.Spec.VID, d.Spec.PID, d.Spec.SerialNumber)
			d.Spec.Status = api.Status_OFFLINE
			d.Spec.Health = api.Health_UNKNOWN
			err := m.k8sclient.UpdateCR(ctx, d)
			if err != nil {
				ll.Errorf("Failed to update drive CR %s, error %s", d.Name, err.Error())
			}
			m.drivesCache[d.Spec.UUID] = d.DeepCopy()
		}
	}
	ll.Info("Current drives cache: ", m.drivesCache)
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
					Owner:        "", // TODO: need to search owner ??? CRD ???
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

// DiscoverAvailableCapacity inspect current available capacity on nodes and fill cache
func (m *VolumeManager) DiscoverAvailableCapacity(nodeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "DiscoverAvailableCapacity",
	})
	ll.Infof("Current available capacity cache is: %v", m.availableCapacityCache)

	m.acCacheMu.Lock()
	defer m.acCacheMu.Unlock()
	for _, drive := range m.drivesCache {
		if drive.Spec.Health == api.Health_GOOD && drive.Spec.Status == api.Status_ONLINE {
			removed := false
			for _, volume := range m.volumesCache {
				//if drive contains volume then available capacity for this drive will be removed
				if strings.EqualFold(volume.Location, drive.Spec.UUID) {
					delete(m.availableCapacityCache, drive.Spec.UUID)
					ll.Infof("Remove available capacity on node %s, because drive %s has volume", nodeID, drive.Spec.UUID)
					removed = true
				}
			}
			//if drive is empty
			if !removed {
				capacity := &api.AvailableCapacity{
					Size:         drive.Spec.Size,
					Location:     drive.Spec.UUID,
					StorageClass: base.ConvertDriveTypeToStorageClass(drive.Spec.Type),
					NodeId:       nodeID,
				}
				ll.Infof("Adding available capacity: %s-%s", capacity.NodeId, capacity.Location)
				m.availableCapacityCache[capacity.Location] = capacity
			}
		} else {
			//If drive is unhealthy or offline, remove available capacity
			for _, ac := range m.availableCapacityCache {
				if drive.Spec.UUID == ac.Location {
					ll.Infof("Remove available capacity on node %s, because drive %s is not ready", ac.NodeId, ac.Location)
					delete(m.availableCapacityCache, ac.Location)
					break
				}
			}
		}
	}
	ll.Info("Current available capacity cache: ", m.availableCapacityCache)
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
	if vol, ok := m.getFromVolumeCache(vol.Id); ok {
		ll.Infof("Found volume in cache with status: %s", api.OperationalStatus_name[int32(vol.Status)])
		return m.pullCreateLocalVolume(ctx, vol.Id)
	}

	var (
		drive     = &drivecrd.Drive{}
		driveUUID = vol.Location
		err       error
	)
	// read Drive CR based on Volume.Location (vol.Location == Drive.UUID == Drive.Name)
	if err = m.k8sclient.ReadCR(ctx, driveUUID, drive); err != nil {
		ll.Errorf("Failed to read crd with name %s, error %s", driveUUID, err.Error())
		return err
	}

	ll.Infof("Search device file")
	device, err := m.searchDrivePathBySN(drive.Spec.SerialNumber)
	if err != nil {
		return err
	}

	m.setVolumeCacheValue(vol.Id, vol)

	ll.Infof("Create partition on device %s and set UUID in background", device)
	go func() {
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

	return m.pullCreateLocalVolume(ctx, vol.Id)
}

func (m *VolumeManager) pullCreateLocalVolume(ctx context.Context, volumeID string) error {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "pullCreatedLocalVolume",
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

// searchDrivePathBySN returns drive path based on drive S/N
func (m *VolumeManager) searchDrivePathBySN(sn string) (string, error) {
	lsblkOut, err := m.linuxUtils.Lsblk("disk")
	if err != nil {
		return "", err
	}

	device := ""
	for _, l := range *lsblkOut {
		if strings.EqualFold(l.Serial, sn) {
			device = l.Name
			break
		}
	}

	if device == "" {
		return "", fmt.Errorf("unable to find drive path by S/N %s", sn)
	}

	return device, nil
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

func (m *VolumeManager) DeleteLocalVolume(ctx context.Context, request *api.DeleteLocalVolumeRequest) (*api.DeleteLocalVolumeResponse, error) {
	ll := m.log.WithFields(logrus.Fields{
		"method":   "DeleteLocalVolume",
		"volumeID": request.GetPvcUUID(),
	})

	ll.Info("Processing request")

	var (
		volume *api.Volume
		ok     bool
	)
	if volume, ok = m.getFromVolumeCache(request.PvcUUID); !ok {
		return &api.DeleteLocalVolumeResponse{Ok: false}, errors.New("unable to find volume by PVC UUID in volume manager cache")
	}

	drive := m.drivesCache[volume.Location]
	if drive == nil {
		return &api.DeleteLocalVolumeResponse{Ok: false}, errors.New("unable to find drive by volume location")
	}

	device, err := m.searchDrivePathBySN(drive.Spec.SerialNumber)
	if err != nil {
		return &api.DeleteLocalVolumeResponse{Ok: false},
			fmt.Errorf("unable to find device for drive with S/N %s", volume.Location)
	}
	ll.Infof("Found device %s", device)

	err = m.linuxUtils.DeletePartition(device)
	if err != nil {
		wErr := fmt.Errorf("failed to delete partition, error: %v", err)
		ll.Errorf("%v, set operational status - fail to remove", wErr)
		m.setVolumeStatus(request.PvcUUID, api.OperationalStatus_FailToRemove)
		return &api.DeleteLocalVolumeResponse{Ok: false}, wErr
	}
	ll.Info("Partition was deleted")

	scImpl := m.getStorageClassImpl(volume.StorageClass)
	ll.Infof("Chosen StorageClass is %s", volume.StorageClass.String())

	err = scImpl.DeleteFileSystem(device)
	if err != nil {
		wErr := fmt.Errorf("failed to wipefs device, error: %v", err)
		ll.Errorf("%v, set operational status - fail to remove", wErr)
		m.setVolumeStatus(request.PvcUUID, api.OperationalStatus_FailToRemove)
		return &api.DeleteLocalVolumeResponse{Ok: false}, wErr
	}
	ll.Info("File system was deleted")

	m.deleteFromVolumeCache(volume.Id)

	ll.Infof("Volume was successfully deleted, return it: %v", volume)
	return &api.DeleteLocalVolumeResponse{Ok: true, Volume: volume}, nil
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
