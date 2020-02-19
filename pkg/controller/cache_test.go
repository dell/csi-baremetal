package controller

import (
	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"testing"

	"github.com/sirupsen/logrus"
)

var c = VolumesCache{items: make(map[VolumeID]*vcrd.Volume)}

func TestAddVolumeToCache(t *testing.T) {
	c.SetLogger(logrus.New())
	volumeName := "add-volume"
	volume := &vcrd.Volume{}
	err := c.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	if c.items[VolumeID(volumeName)] != volume {
		t.Errorf("Volume %s wasn't added to items", volumeName)
	}
}

func TestAddVolumeToCacheAlreadyExists(t *testing.T) {
	volumeName := "exists"
	volume := &vcrd.Volume{}
	err := c.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	err = c.addVolumeToCache(volume, volumeName)
	sameNameVolume := &vcrd.Volume{}
	err = c.addVolumeToCache(sameNameVolume, volumeName)
	if err == nil {
		t.Errorf("addVolumeToCache sholud throw an error")
	}
}

func TestGetVolumeByNameEmpty(t *testing.T) {
	volumeName := "doesn't exist"
	volume := c.getVolumeByID(volumeName)
	if volume != nil {
		t.Errorf("Volume %s shouldn't exist", volumeName)
	}
}

func TestGetVolumeByName(t *testing.T) {
	volumeName := "get-volume"
	volume := &vcrd.Volume{}
	err := c.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	if c.getVolumeByID(volumeName) != volume {
		t.Errorf("Can't get volume %s", volumeName)
	}
}

func TestDeleteVolumeById(t *testing.T) {
	volumeID := "id_of_deleted_volume"
	volumeName := "volume_to_delete"
	volume := &vcrd.Volume{Spec: api.Volume{Id: volumeID}}
	err := c.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	c.deleteVolumeByID(volumeID)
	if c.getVolumeByID(volumeName) != nil {
		t.Errorf("Volume %s wasn't deleted", volumeName)
	}
}
