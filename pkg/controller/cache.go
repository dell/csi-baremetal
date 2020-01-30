package controller

import (
	"sync"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type VolumeID string

type csiVolume struct {
	NodeID   string
	VolumeID string
	Size     int64
}

type VolumesCache struct {
	items map[VolumeID]*csiVolume
	sync.Mutex
}

func (c *VolumesCache) getVolumeByID(volumeID string) *csiVolume {
	c.Lock()
	defer c.Unlock()
	volume, ok := c.items[VolumeID(volumeID)]
	if ok {
		logrus.Infof("Volume %s is found in items", volumeID)
	} else {
		logrus.Infof("Volume %s is not found in items", volumeID)
	}
	return volume
}

func (c *VolumesCache) addVolumeToCache(volume *csiVolume, id string) error {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.items[VolumeID(id)]; ok {
		logrus.Errorf("Volume %s already exists in items", id)
		return status.Errorf(codes.AlreadyExists, "Volume with the same id: %s already exist", id)
	}
	c.items[VolumeID(id)] = volume
	logrus.Infof("Volume %s is added to items", id)
	return nil
}

func (c *VolumesCache) deleteVolumeByID(volumeID string) {
	c.Lock()
	defer c.Unlock()
	for volumeName, volume := range c.items {
		if volume.VolumeID == volumeID {
			logrus.Infof("Deleting volume %s from items", volumeName)
			delete(c.items, volumeName)
			break
		}
	}
}
