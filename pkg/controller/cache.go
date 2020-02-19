package controller

import (
	"sync"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type VolumeID string

// thread safety cache
type VolumesCache struct {
	items map[VolumeID]*volumecrd.Volume
	sync.Mutex
	log *logrus.Entry
}

func (c *VolumesCache) SetLogger(logger *logrus.Logger) {
	c.log = logger.WithField("component", "VolumesCache")
}

func (c *VolumesCache) getVolumeByID(volumeID string) *volumecrd.Volume {
	ll := c.log.WithField("volumeID", volumeID)

	c.Lock()
	defer c.Unlock()
	volume, ok := c.items[VolumeID(volumeID)]
	if ok {
		ll.Debug("Volume is found in items")
	} else {
		ll.Debug("Volume is not found in items")
	}
	return volume
}

func (c *VolumesCache) addVolumeToCache(volume *volumecrd.Volume, id string) error {
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
		if volume.Spec.Id == volumeID {
			ll.Info("Deleting volume from items")
			delete(c.items, volumeName)
			break
		}
	}
}

func (c *VolumesCache) setVolumeStatus(volumeID string, newStatus api.OperationalStatus) {
	ll := c.log.WithFields(logrus.Fields{
		"volumeID": volumeID,
		"method":   "setVolumeStatus"})

	// getVolumeByID works through mutex
	vol := c.getVolumeByID(volumeID)

	c.Lock()
	defer c.Unlock()
	ll.Infof("Change status from %s to %s", api.OperationalStatus_name[int32(vol.Spec.Status)],
		api.OperationalStatus_name[int32(newStatus)])

	vol.ObjectMeta.Annotations[VolumeStatusAnnotationKey] = api.OperationalStatus_name[int32(newStatus)]
	vol.Spec.Status = newStatus
}
