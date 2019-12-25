package storagemanager

// at now LVM or BlockDevice
type StorageSubsystem interface {
	PrepareVolume(capacityGb float64, preferredNode string) (node string, volumeID string, err error)
	ReleaseVolume(node string, volumeID string) error
	IsInitialized() bool
}

// now just REST
type Communicator interface {
	GetNodeStorageInfo() (interface{}, error) // VolumeGroup{} or []HalDisks
	PrepareVolumeOnNode(vi VolumeInfo) (volumeID string, err error)
	ReleaseVolumeOnNode(vi VolumeInfo) error
}

type StorageTopology map[string]interface{} // -> map[string]VolumeGroup map[string][]HalDisk

type VolumeInfo struct {
	Name       string  // volume name
	CapacityGb float64 // capacity in Gigabytes
}
