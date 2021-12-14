package common

// WbtConfig is a part of WBT ConfigMap
// contains changing value and options to select acceptable volume
type WbtConfig struct {
	Enable        bool          `yaml:"enable"`
	Value         uint32        `yaml:"wbt_lat_usec_value"`
	VolumeOptions VolumeOptions `yaml:"acceptable_volume_options"`
}

// VolumeOptions contains options to select acceptable volume
type VolumeOptions struct {
	Modes          []string `yaml:"modes"`
	StorageClasses []string `yaml:"storage_classes"`
}

// AcceptableKernelsConfig is a part of WBT ConfigMap
// contains the list of kernel versions from nodes,
// which should be able to set custom WBT value
type AcceptableKernelsConfig struct {
	EnableForAll   bool     `yaml:"enable_for_all"`
	KernelVersions []string `yaml:"node_kernel_versions"`
}
