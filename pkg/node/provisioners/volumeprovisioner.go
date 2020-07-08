// Package provisioners contains code for Volume CR reconcile handling
// during which volumes on node are created or removed
// It operates by underlying structures such as a drives/partitions/file system
// and encapsulates all low-level work with these objects.
package provisioners

import api "github.com/dell/csi-baremetal.git/api/generated/v1"

// VolumeType is used for describing class of volume depending on underlying structures
// volume could be based on partitions, logical volume and so on
type VolumeType string

const (
	// DriveBasedVolumeType represents volume that consumes whole drive
	DriveBasedVolumeType VolumeType = "DriveBased"
	// LVMBasedVolumeType represents volume that based on Volume Group
	LVMBasedVolumeType VolumeType = "LVMBased"
)

// Provisioner is a high-level interface that encapsulates all low-level work with volumes on node
type Provisioner interface {
	// Prepare volume for mount
	PrepareVolume(volume api.Volume) error
	// Completely release underlying resources that had consumed by volume
	ReleaseVolume(volume api.Volume) error
	// Return full path of device file that represent volume on node
	GetVolumePath(volume api.Volume) (string, error)
}
