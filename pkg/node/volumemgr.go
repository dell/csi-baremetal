package node

import (
	"context"
	"errors"
	"strconv"
	"sync"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"github.com/sirupsen/logrus"
)

type VolumeManager struct {
	hWMgrClient api.HWServiceClient
	sync.Mutex
	volumesCache []*api.Volume
	linuxUtils   *base.LinuxUtils
	partition    *base.Partition
}

// NewVolumeManager returns new instance ov VolumeManager
func NewVolumeManager(client api.HWServiceClient) *VolumeManager {
	return &VolumeManager{
		hWMgrClient:  client,
		volumesCache: make([]*api.Volume, 0),
		linuxUtils:   base.NewLinuxUtils(&base.Executor{}),
		partition:    &base.Partition{Executor: &base.Executor{}},
	}
}

// SetLinuxUtilsExecutor set executor for linuxUtils instance, needed for unit tests purposes
func (m *VolumeManager) SetLinuxUtilsExecutor(e base.CmdExecutor) {
	m.linuxUtils.SetExecutor(e)
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

	m.Lock()
	defer m.Unlock()

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
				sizeInt, err := strconv.ParseUint(ld.Size, 10, 64)
				if err != nil {
					ll.Errorf("Unable to interpret size %v, lsblk output: %v", err, ld)
					continue
				}
				v := &api.Volume{
					Id:           "", // TODO: FABRIC-8507, need to search ID based on partition ID
					Owner:        "", // TODO: need to search owner ??? CRD ???
					Size:         sizeInt,
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

func (m *VolumeManager) CreateLocalVolume(ctx context.Context, request *api.CreateLocalVolumeRequest) (*api.CreateLocalVolumeResponse, error) {
	// TODO: get device name from cache
	device := ""
	if exists, _ := m.partition.IsPartitionExists(device); exists {
		err := m.partition.CreatePartitionTable(device)
		if err != nil {
			return &api.CreateLocalVolumeResponse{Drive: device, Ok: false}, err
		}

		err = m.partition.CreatePartition(device)
		if err != nil {
			return &api.CreateLocalVolumeResponse{Drive: device, Ok: false}, err
		}

		err = m.partition.SetPartitionUUID(device, request.PvcUUID)
		if err != nil {
			return &api.CreateLocalVolumeResponse{Drive: device, Ok: false}, err
		}
	}

	return &api.CreateLocalVolumeResponse{Drive: device, Ok: false}, errors.New("unable to find suitable drive")
}

func (m *VolumeManager) DeleteLocalVolume(ctx context.Context, request *api.DeleteLocalVolumeRequest) (*api.DeleteLocalVolumeResponse, error) {
	// TODO: get device name Controller
	device := ""

	err := m.partition.DeletePartition(device)
	if err != nil {
		return &api.DeleteLocalVolumeResponse{Ok: false}, err
	}

	return &api.DeleteLocalVolumeResponse{Ok: true}, nil
}
