package controller

import "testing"

var c = VolumesCache{items: make(map[VolumeID]*csiVolume)}

func TestAddVolumeToCache(t *testing.T) {
	volumeName := "add-volume"
	volume := &csiVolume{}
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
	volume := &csiVolume{}
	err := c.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	err = c.addVolumeToCache(volume, volumeName)
	sameNameVolume := &csiVolume{}
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
	volume := &csiVolume{}
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
	volume := &csiVolume{
		VolumeID: volumeID,
	}
	err := c.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	c.deleteVolumeByID(volumeID)
	if c.getVolumeByID(volumeName) != nil {
		t.Errorf("Volume %s wasn't deleted", volumeName)
	}
}
