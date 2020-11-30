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

package v1

const (
	VolumeKind                       = "Volume"
	AvailableCapacityKind            = "AvailableCapacity"
	AvailableCapacityReservationKind = "AvailableCapacityReservation"
	LVGKind                          = "LVG"
	DriveKind                        = "Drive"
	CSIBMNodeKind                    = "Node"

	Version = "v1"
	// TODO: change value, https://github.com/dell/csi-baremetal/issues/134
	CSICRsGroupVersion = "baremetal-csi.dellemc.com"
	APIV1Version       = "baremetal-csi.dellemc.com/v1"
	// CSI statuses
	Creating    = "CREATING"
	Created     = "CREATED"
	VolumeReady = "VOLUME_READY"
	Published   = "PUBLISHED"
	Removing    = "REMOVING"
	Removed     = "REMOVED"
	Failed      = "FAILED"

	// Health statuses
	HealthUnknown = "UNKNOWN"
	HealthGood    = "GOOD"
	HealthSuspect = "SUSPECT"
	HealthBad     = "BAD"

	// Drive status
	DriveStatusOnline  = "ONLINE"
	DriveStatusOffline = "OFFLINE"

	// Drive Usage status
	DriveInUse     = "IN_USE"
	DriveReleasing = "RELEASING"
	DriveReleased  = "RELEASED"
	DriveFailed    = "FAILED"
	DriveRemoving  = "REMOVING"
	DriveRemoved   = "REMOVED"

	// Drive type
	DriveTypeHDD  = "HDD"
	DriveTypeSSD  = "SSD"
	DriveTypeNVMe = "NVME"

	// Drive annotations
	DriveAnnotationReplacement = "replacement"
	DriveAnnotationReplacementReady = "ready"

	// Volume operational status
	OperationalStatusOperative     = "OPERATIVE"
	OperationalStatusInoperative   = "INOPERATIVE"
	OperationalStatusStaging       = "STAGING"
	OperationalStatusMissing       = "MISSING"
	OperationalStatusMaintenance   = "MAINTENANCE"
	OperationalStatusUnknown       = "UNKNOWN"

	// Volume Usage status
	VolumeInUse     = DriveInUse
	VolumeReleasing = DriveReleasing
	VolumeReleased  = DriveReleased
	VolumeFailed    = DriveFailed

	// Release Volume annotations
	VolumeAnnotationRelease = "release"
	VolumeAnnotationReleaseDone = "done"

	// TODO Mount status?

	// Volume mode
	ModeRAW = "RAW"
	ModeFS  = "FS"

	// Volume location type
	LocationTypeDrive = "DRIVE"
	LocationTypeLVM   = "LVM"
	LocationTypeNVMe  = "NVME"

	// CSI StorageClass
	StorageClassAny       = "ANY"
	StorageClassHDD       = "HDD"
	StorageClassSSD       = "SSD"
	StorageClassNVMe      = "NVME"
	StorageClassHDDLVG    = "HDDLVG"
	StorageClassSSDLVG    = "SSDLVG"
	StorageClassNVMeLVG   = "NVMELVG"
	StorageClassSystemLVG = "SYSLVG"
)
