// Package base is for basic methods which can be used by all CSI components
package base

import "time"

const (
	// PluginName is a name of current CSI plugin
	PluginName = "baremetal-csi"
	// PluginVersion is a version of current CSI plugin
	PluginVersion = "0.0.6"
	// DefaultDriveMgrEndpoint is the default gRPC endpoint for drivemgr
	DefaultDriveMgrEndpoint = "tcp://localhost:8888"
	// DefaultHealthIP is the default gRPC IP for Health server
	DefaultHealthIP = "127.0.0.1"
	// DefaultHealthPort is the default gRPC port for Health Server
	DefaultHealthPort = 9999

	// KubeletRootPath is the pods' path on the node
	KubeletRootPath = "/var/lib/kubelet/pods"
	// NonRotationalNum points on SSD drive
	NonRotationalNum = "0"

	// DefaultTimeoutForOperations is the timeout in which we expect that any operation should be finished
	DefaultTimeoutForOperations = 10 * time.Minute

	// SystemDriveAsLocation is the const to fill Location field in CRs if the location based on system drive
	SystemDriveAsLocation = "system drive"

	// DefaultFsType FS type that used by default
	DefaultFsType = "xfs"

	// StorageTypeKey key from volume_context in CreateVolumeRequest of NodePublishVolumeRequest
	StorageTypeKey = "storageType"
	// SizeKey key from volume_context in CreateVolumeRequest of NodePublishVolumeRequest
	SizeKey = "size"
)
