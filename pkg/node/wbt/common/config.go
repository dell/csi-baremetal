package common

// WbtConfig is a part of WBT ConfigMap
// contains changing value and options to select acceptable volume
type WbtConfig struct {
	Enable        bool   `yaml:"enable"`
	Value         uint32 `yaml:"wbt_lat_usec_value"`
	VolumeOptions struct {
		Modes        []string `yaml:"modes"`
		StorageTypes []string `yaml:"storage_types"`
	} `yaml:"acceptable_volume_options"`
}

// AcceptableKernelsConfig is a part of WBT ConfigMap
// contains the list of kernel versions from nodes,
// which should be able to set custom WBT value
type AcceptableKernelsConfig struct {
	KernelVersions []string `yaml:"node_kernel_versions"`
}
