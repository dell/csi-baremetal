/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package provisioners contains code for Volume CR reconcile handling
// during which volumes on node are created or removed
// It operates by underlying structures such as a drives/partitions/file system
// and encapsulates all low-level work with these objects.
package provisioners

import api "github.com/dell/csi-baremetal/api/generated/v1"

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
	// PrepareVolume prepares volume for mount
	PrepareVolume(volume *api.Volume) error
	// ReleaseVolume completely releases underlying resources that had consumed by volume
	ReleaseVolume(volume *api.Volume, drive *api.Drive) error
	// GetVolumePath returns full path of device file that represent volume on node
	GetVolumePath(volume *api.Volume) (string, error)
}
