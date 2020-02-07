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
	log *logrus.Entry
}

func (c *VolumesCache) SetLogger(logger *logrus.Logger) {
	c.log = logger.WithField("component", "VolumesCache")
}

func (c *VolumesCache) getVolumeByID(volumeID string) *csiVolume {
	ll := c.log.WithField("volumeID", volumeID)

	c.Lock()
	defer c.Unlock()
	volume, ok := c.items[VolumeID(volumeID)]
	if ok {
		ll.Info("Volume is found in items")
	} else {
		ll.Info("Volume is not found in items")
	}
	return volume
}

func (c *VolumesCache) addVolumeToCache(volume *csiVolume, id string) error {
	ll := c.log.WithField("volumeID", id)

	c.Lock()
	defer c.Unlock()
	if _, ok := c.items[VolumeID(id)]; ok {
		ll.Errorf("Volume already exists in items")
		return status.Errorf(codes.AlreadyExists, "Volume with the same id: %s already exist", id)
	}
	c.items[VolumeID(id)] = volume
	ll.Info("Volume is added to items")
	return nil
}

func (c *VolumesCache) deleteVolumeByID(volumeID string) {
	ll := c.log.WithField("volumeID", volumeID)

	c.Lock()
	defer c.Unlock()
	for volumeName, volume := range c.items {
		if volume.VolumeID == volumeID {
			ll.Info("Deleting volume from items")
			delete(c.items, volumeName)
			break
		}
	}
}
