package controller

import "testing"

func TestAddVolumeToCache(t *testing.T) {
	volumeName := "add-volume"
	volume := &csiVolume{}
	err := svc.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	if svc.volumeCache.Cache[VolumeID(volumeName)] != volume {
		t.Errorf("Volume %s wasn't added to cache", volumeName)
	}
}

func TestAddVolumeToCacheAlreadyExists(t *testing.T) {
	volumeName := "exists"
	volume := &csiVolume{}
	err := svc.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	err = svc.addVolumeToCache(volume, volumeName)
	sameNameVolume := &csiVolume{}
	err = svc.addVolumeToCache(sameNameVolume, volumeName)
	if err == nil {
		t.Errorf("addVolumeToCache sholud throw an error")
	}
}

func TestGetVolumeByNameEmpty(t *testing.T) {
	volumeName := "doesn't exist"
	volume := svc.getVolumeByID(volumeName)
	if volume != nil {
		t.Errorf("Volume %s shouldn't exist", volumeName)
	}
}

func TestGetVolumeByName(t *testing.T) {
	volumeName := "get-volume"
	volume := &csiVolume{}
	err := svc.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	if svc.getVolumeByID(volumeName) != volume {
		t.Errorf("Can't get volume %s", volumeName)
	}
}

func TestDeleteVolumeById(t *testing.T) {
	volumeID := "id_of_deleted_volume"
	volumeName := "volume_to_delete"
	volume := &csiVolume{
		VolumeID: volumeID,
	}
	err := svc.addVolumeToCache(volume, volumeName)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	svc.deleteVolumeByID(volumeID)
	if svc.getVolumeByID(volumeName) != nil {
		t.Errorf("Volume %s wasn't deleted", volumeName)
	}
}
