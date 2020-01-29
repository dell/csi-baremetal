package node

import (
	"context"
	"fmt"
	"math"
	"sync"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"github.com/sirupsen/logrus"
)

type VolumeManager struct {
	hWMgrClient  api.HWServiceClient
	mu           sync.Mutex
	volumesCache []*api.Volume
	linuxUtils   *base.LinuxUtils
}

// NewVolumeManager returns new instance ov VolumeManager
func NewVolumeManager(client api.HWServiceClient, executor base.CmdExecutor) *VolumeManager {
	return &VolumeManager{
		hWMgrClient:  client,
		volumesCache: make([]*api.Volume, 0),
		linuxUtils:   base.NewLinuxUtils(executor),
	}
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
	ll := logrus.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "Discover",
	})
	ll.Infof("Current volumes cache is: %v", m.volumesCache)

	m.mu.Lock()
	defer m.mu.Unlock()

	drivesResponse, err := m.hWMgrClient.GetDrivesList(context.Background(), &api.DrivesRequest{})
	if err != nil {
		return err
	}
	drives := drivesResponse.Disks

	freeDrives := m.drivesAreNotUsed(drives)

	// explore each drive from freeDrives
	lsblk, err := m.linuxUtils.Lsblk(base.DriveTypeDisk)
	if err != nil {
		logrus.Errorf("Unable to inspect system block devices via lsblk, error: %v", err)
		return err
	}
	for _, d := range freeDrives {
		for _, ld := range *lsblk {
			if ld.Serial == d.SerialNumber && len(ld.Children) > 0 {
				pID, err := m.linuxUtils.GetPartitionUUID(ld.Name)
				if err != nil {
					ll.Errorf("Unable to determine partition UUID for device %s, error: %v", ld.Name, err)
					continue
				}
				v := &api.Volume{
					Id:           pID,
					Owner:        "", // TODO: need to search owner ??? CRD ???
					Size:         ld.Size,
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
	ll := logrus.WithFields(logrus.Fields{
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
				d.SerialNumber == v.Location {
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
	logrus.WithFields(logrus.Fields{
		"component": "VolumeManager",
		"method":    "CreateLocalVolume",
	}).Infof("Processing request %v", req)

	m.mu.Lock()
	defer m.mu.Unlock()

	resp := &api.CreateLocalVolumeResponse{Drive: "", Capacity: 0, Ok: false}

	drive, err := m.searchFreeDrive(req.Capacity)
	if err != nil {
		return resp, err
	}

	device, err := m.getDrivePathBySN(drive.SerialNumber)
	if err != nil {
		return resp, err
	}

	err = m.setPartitionUUIDForDev(device, req.PvcUUID)
	if err != nil {
		return resp, err
	}

	m.volumesCache = append(m.volumesCache, &api.Volume{
		Id:           req.PvcUUID,
		Owner:        "",
		Size:         drive.Size,
		Location:     device,
		LocationType: api.LocationType_Drive,
		Mode:         api.Mode_FS,
		Type:         "", // TODO: set that filed to FSType
		Health:       api.Health_GOOD,
		Status:       api.OperationalStatus_Operative,
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
		if l.Serial == sn {
			device = l.Name
			break
		}
	}

	if device == "" {
		return "", fmt.Errorf("unable to find drive path by S/N \"%s\"", sn)
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
	// TODO: get device name Controller
	device := ""

	err := m.linuxUtils.DeletePartition(device)
	if err != nil {
		return &api.DeleteLocalVolumeResponse{Ok: false}, err
	}

	return &api.DeleteLocalVolumeResponse{Ok: true}, nil
}
