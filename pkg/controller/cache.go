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
	Cache map[VolumeID]*csiVolume
	sync.Mutex
}

func (c *CSIControllerService) getVolumeByID(volumeID string) *csiVolume {
	c.volumeCache.Lock()
	defer c.volumeCache.Unlock()
	volume, ok := c.volumeCache.Cache[VolumeID(volumeID)]
	if ok {
		logrus.Infof("Volume %s is found in Cache", volumeID)
	} else {
		logrus.Infof("Volume %s is not found in Cache", volumeID)
	}
	return volume
}

func (c *CSIControllerService) addVolumeToCache(volume *csiVolume, name string) error {
	c.volumeCache.Lock()
	defer c.volumeCache.Unlock()
	if _, ok := c.volumeCache.Cache[VolumeID(name)]; ok {
		logrus.Errorf("Volume %s already exists in Cache", name)
		return status.Errorf(codes.AlreadyExists, "Volume with the same name: %s already exist", name)
	}
	c.volumeCache.Cache[VolumeID(name)] = volume
	logrus.Infof("Volume %s is added to Cache", name)
	return nil
}

func (c *CSIControllerService) deleteVolumeByID(volumeID string) {
	c.volumeCache.Lock()
	defer c.volumeCache.Unlock()
	for volumeName, volume := range c.volumeCache.Cache {
		if volume.VolumeID == volumeID {
			logrus.Infof("Deleting volume %s from Cache", volumeName)
			delete(c.volumeCache.Cache, volumeName)
			break
		}
	}
}
