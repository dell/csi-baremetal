package driver

import "testing"

func TestAddVolumeToCache(t *testing.T) {
	volumeName := "add-volume"
	volume := &csiVolume{
		Name: volumeName,
	}
	err := csiVolumesCache.addVolumeToCache(volume)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	if csiVolumesCache[volumeName] != volume {
		t.Errorf("Volume %s wasn't added to cache", volumeName)
	}
}

func TestAddVolumeToCacheAlreadyExists(t *testing.T) {
	volumeName := "exists"
	volume := &csiVolume{
		Name: volumeName,
	}
	err := csiVolumesCache.addVolumeToCache(volume)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	err = csiVolumesCache.addVolumeToCache(volume)
	sameNameVolume := &csiVolume{
		Name: volumeName,
	}
	err = csiVolumesCache.addVolumeToCache(sameNameVolume)
	if err == nil {
		t.Errorf("addVolumeToCache sholud throw an error")
	}
}

func TestGetVolumeByNameEmpty(t *testing.T) {
	volumeName := "doesn't exist"
	volume := csiVolumesCache.getVolumeByName(volumeName)
	if volume != nil {
		t.Errorf("Volume %s shouldn't exist", volumeName)
	}
}

func TestGetVolumeByName(t *testing.T) {
	volumeName := "get-volume"
	volume := &csiVolume{
		Name: volumeName,
	}
	err := csiVolumesCache.addVolumeToCache(volume)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	if csiVolumesCache.getVolumeByName(volumeName) != volume {
		t.Errorf("Can't get volume %s", volumeName)
	}
}

func TestDeleteVolumeById(t *testing.T) {
	volumeID := "id_of_deleted_volume"
	volumeName := "volume_to_delete"
	volume := &csiVolume{
		Name:     volumeName,
		VolumeID: volumeID,
	}
	err := csiVolumesCache.addVolumeToCache(volume)
	if err != nil {
		t.Errorf("Something went wrong: %s", err.Error())
	}
	csiVolumesCache.deleteVolumeByID(volumeID)
	if csiVolumesCache.getVolumeByName(volumeName) != nil {
		t.Errorf("Volume %s wasn't deleted", volumeName)
	}
}
