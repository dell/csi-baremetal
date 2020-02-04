package node

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"github.com/sirupsen/logrus"
)

type VolumeManager struct {
	hWMgrClient  api.HWServiceClient
	volumesCache []*api.Volume
	cacheMutex   sync.Mutex
	linuxUtils   *base.LinuxUtils
	log          *logrus.Logger
}

// NewVolumeManager returns new instance ov VolumeManager
func NewVolumeManager(client api.HWServiceClient, executor base.CmdExecutor) *VolumeManager {
	l := logrus.New()
	l.Out = os.Stdout
	return &VolumeManager{
		hWMgrClient:  client,
		volumesCache: make([]*api.Volume, 0),
		linuxUtils:   base.NewLinuxUtils(executor),
		log:          l,
	}
}

func (m *VolumeManager) setLogger(logger *logrus.Logger) {
	m.log = logger
	m.log.Info("Logger was set in VolumeManager")
}

// GetLocalVolumes request return array of volumes on node
func (m *VolumeManager) GetLocalVolumes(context.Context, *api.VolumeRequest) (*api.VolumeResponse, error) {
	return &api.VolumeResponse{Volume: m.volumesCache}, nil
}

// GetAvailableCapacity request return array of free capacity on node
func (m *VolumeManager) GetAvailableCapacity(context.Context, *api.AvailableCapacityRequest) (*api.AvailableCapacityResponse, error) {
	capacities := make([]*api.AvailableCapacity, 0)
	return &api.AvailableCapacityResponse{AvailableCapacity: capacities}, nil
}

// Discover inspects drives and create volume object if partition exist
func (m *VolumeManager) Discover() error {
	ll := m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "Discover",
	})
	ll.Infof("Current volumes cache is: %v", m.volumesCache)

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	drivesResponse, err := m.hWMgrClient.GetDrivesList(context.Background(), &api.DrivesRequest{})
	if err != nil {
		return err
	}
	drives := drivesResponse.Disks

	freeDrives := m.drivesAreNotUsed(drives)

	// explore each drive from freeDrives
	lsblk, err := m.linuxUtils.Lsblk(base.DriveTypeDisk)
	if err != nil {
		ll.Errorf("Unable to inspect system block devices via lsblk, error: %v", err)
		return err
	}
	for _, d := range freeDrives {
		for _, ld := range *lsblk {
			if strings.EqualFold(ld.Serial, d.SerialNumber) && len(ld.Children) > 0 {
				pID, err := m.linuxUtils.GetPartitionUUID(ld.Name)
				if err != nil {
					ll.Errorf("Unable to determine partition UUID for device %s, error: %v", ld.Name, err)
					continue
				}
				size, err := strconv.ParseInt(ld.Size, 10, 64)
				if err != nil {
					ll.Infof("Unable parse string %s to int, for device %s, error: %v", ld.Size, ld.Name, err)
					continue
				}
				v := &api.Volume{
					Id:           pID,
					Owner:        "", // TODO: need to search owner ??? CRD ???
					Size:         size,
					Location:     d.SerialNumber,
					LocationType: api.LocationType_Drive,
					Mode:         api.Mode_FS,
					Type:         ld.FSType,
					Health:       d.Health,
					Status:       api.OperationalStatus_Operative,
				}
				// search ID and owner here
				ll.Info("Search volume ID and volume owner: Not implemented, skip ...")
				ll.Infof("Add in cache volume: %v", v)
				m.volumesCache = append(m.volumesCache, v)
			}
		}
	}
	return nil
}

// drivesAreNotUsed search drives that isn't have any volumes
func (m *VolumeManager) drivesAreNotUsed(drives []*api.Drive) []*api.Drive {
	ll := m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "drivesAreNotUsed",
	})

	// search drives that don't have parent volume
	drivesNotInUse := make([]*api.Drive, 0)
	for _, d := range drives {
		isUsed := false
		for _, v := range m.volumesCache {
			// expect only Drive LocationType, for Drive LocationType Location will be a SN of the drive
			if d.Type != api.DriveType_NVMe &&
				v.LocationType == api.LocationType_Drive &&
				strings.EqualFold(d.SerialNumber, v.Location) {
				isUsed = true
				ll.Infof("Found volume with ID \"%s\" in cache for drive with S/N \"%s\"",
					v.Id, d.SerialNumber)
				break
			}
		}
		if !isUsed {
			drivesNotInUse = append(drivesNotInUse, d)
		}
	}

	return drivesNotInUse
}

func (m *VolumeManager) CreateLocalVolume(ctx context.Context, req *api.CreateLocalVolumeRequest) (*api.CreateLocalVolumeResponse, error) {
	m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "CreateLocalVolume",
	}).Infof("Processing request %v", req)

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	resp := &api.CreateLocalVolumeResponse{Drive: "", Capacity: 0, Ok: false}

	drive, err := m.searchFreeDrive(req.Capacity)
	if err != nil {
		return resp, err
	}
	m.log.Infof("Found drive: %v", drive)

	device, err := m.getDrivePathBySN(drive.SerialNumber)
	if err != nil {
		return resp, err
	}
	m.log.Infof("Choose device: %s", device)

	err = m.setPartitionUUIDForDev(device, req.PvcUUID)
	if err != nil {
		return resp, err
	}

	m.volumesCache = append(m.volumesCache, &api.Volume{
		Id:    req.PvcUUID,
		Owner: "",
		Size:  drive.Size,
		// TODO: Ruslan to fix, Location - SN
		Location:     device,
		LocationType: api.LocationType_Drive,
		Mode:         api.Mode_FS,
		Type:         "", // TODO: set that filed to FSType
		Health:       api.Health_GOOD,
		Status:       api.OperationalStatus_Staging, // becomes operative in NodePublishCall
	})

	return &api.CreateLocalVolumeResponse{Drive: device, Capacity: drive.Size, Ok: true}, nil
}

// getDrivePathBySN returns drive path based on drive S/N
func (m *VolumeManager) getDrivePathBySN(sn string) (string, error) {
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

// searchFreeDrive search drive via HWMgr with appropriate capacity
func (m *VolumeManager) searchFreeDrive(capacity int64) (*api.Drive, error) {
	drivesResponse, err := m.hWMgrClient.GetDrivesList(context.Background(), &api.DrivesRequest{})
	if err != nil {
		return nil, err
	}
	drives := drivesResponse.Disks

	freeDrives := m.drivesAreNotUsed(drives)
	minSize := int64(math.MaxInt64)
	var drive *api.Drive
	for _, d := range freeDrives {
		if d.Size >= capacity && d.Size < minSize {
			drive = d
			minSize = d.Size
		}
	}

	if drive == nil {
		return nil, fmt.Errorf("unable to find suitable drive with capacity %d", capacity)
	}

	return drive, nil
}

// setPartitionUUIDForDev creates partition on device and set partition uuid
func (m *VolumeManager) setPartitionUUIDForDev(device string, uuid string) error {
	exist, err := m.linuxUtils.IsPartitionExists(device)
	if err != nil {
		return err
	}
	if exist {
		return fmt.Errorf("partition has already exist for device %s", device)
	}

	err = m.linuxUtils.CreatePartitionTable(device)
	if err != nil {
		return err
	}

	err = m.linuxUtils.CreatePartition(device)
	if err != nil {
		return err
	}

	err = m.linuxUtils.SetPartitionUUID(device, uuid)
	if err != nil {
		return err
	}

	return nil
}

func (m *VolumeManager) DeleteLocalVolume(ctx context.Context, request *api.DeleteLocalVolumeRequest) (*api.DeleteLocalVolumeResponse, error) {
	ll := m.log.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "DeleteLocalVolume",
	})
	ll.Info("processing")

	m.cacheMutex.Lock()
	ll.Info("lock mutex")
	defer func() {
		m.cacheMutex.Unlock()
		ll.Info("unlock mutex")
	}()
	volume := m.getVolumeFromCache(request.PvcUUID)

	if volume == nil {
		return &api.DeleteLocalVolumeResponse{Ok: false}, errors.New("unable to find volume by PVC UUID in volume manager cache")
	}

	// TODO: Ruslan to fix
	device := volume.Location
	//device, err := m.getDrivePathBySN(volume.Location)
	//if err != nil {
	//	return &api.DeleteLocalVolumeResponse{Ok: false}, err
	//}

	err := m.linuxUtils.DeletePartition(device)
	if err != nil {
		ll.Infof("failed to delete partition with %s, set operational status - fail to remove", err)
		volume.Status = api.OperationalStatus_FailToRemove
		return &api.DeleteLocalVolumeResponse{Ok: false}, err
	}

	// TODO: Ruslan to make cache as map
	m.removeVolumeFromCache(volume.Id)

	return &api.DeleteLocalVolumeResponse{Ok: true}, nil
}

func (m *VolumeManager) getVolumeFromCache(volumeID string) *api.Volume {
	for _, v := range m.volumesCache {
		if v.Id == volumeID {
			return v
		}
	}
	return nil
}

func (m *VolumeManager) removeVolumeFromCache(volumeID string) {
	index := 0
	for i, v := range m.volumesCache {
		if v.Id == volumeID {
			index = i
		}
	}

	m.volumesCache = append(m.volumesCache[:index], m.volumesCache[index+1:]...)
}
