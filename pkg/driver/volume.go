package driver

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// struct to hold created volume data
type csiVolume struct {
	Name     string
	VolumeID string
	NodeID   string
	Size     int64
}

type VolumesCache map[string]*csiVolume

// cache of created volumes
var csiVolumesCache VolumesCache = map[string]*csiVolume{}

func (vc *VolumesCache) getVolumeByName(volumeName string) *csiVolume {
	volume, ok := csiVolumesCache[volumeName]
	if ok {
		logrus.Infof("Volume %s is found in cache", volumeName)
	} else {
		logrus.Infof("Volume %s is not found in cache", volumeName)
	}

	return volume
}

func (vc *VolumesCache) addVolumeToCache(volume *csiVolume) error {
	volumeName := volume.Name

	if _, ok := csiVolumesCache[volumeName]; ok {
		logrus.Errorf("Volume %s already exists in cache", volumeName)
		return status.Errorf(codes.AlreadyExists, "Volume with the same name: %s already exist", volumeName)
	}

	csiVolumesCache[volumeName] = volume

	logrus.Infof("Volume %s is added to cache", volumeName)

	return nil
}

func (vc *VolumesCache) deleteVolumeByID(volumeID string) {
	for volumeName, volume := range csiVolumesCache {
		if volume.VolumeID == volumeID {
			logrus.Infof("Attempt to delete volume %s from cache", volumeName)
			delete(csiVolumesCache, volumeName)
		}
	}
}
