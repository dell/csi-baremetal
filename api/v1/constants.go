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
	LVGKind                          = "LogicalVolumeGroup"
	DriveKind                        = "Drive"
	CSIBMNodeKind                    = "Node"

	Version            = "v1"
	CSICRsGroupVersion = "csi-baremetal.dell.com"
	APIV1Version       = "csi-baremetal.dell.com/v1"

	// CSI statuses
	Creating    = "CREATING"
	Created     = "CREATED"
	VolumeReady = "VOLUME_READY"
	Published   = "PUBLISHED"
	Removing    = "REMOVING"
	Removed     = "REMOVED"
	Failed      = "FAILED"
	Empty       = ""
	Resizing    = "RESIZING"
	Resized     = "RESIZED"

	// Health statuses
	HealthUnknown = "UNKNOWN"
	HealthGood    = "GOOD"
	HealthSuspect = "SUSPECT"
	HealthBad     = "BAD"

	// TODO need to split constants by different packages
	// Drive status
	DriveStatusOnline  = "ONLINE"
	DriveStatusOffline = "OFFLINE"

	// Drive Usage status
	DriveUsageInUse     = "IN_USE"
	DriveUsageReleasing = "RELEASING"
	DriveUsageReleased  = "RELEASED"
	DriveUsageFailed    = "FAILED"
	DriveUsageRemoving  = "REMOVING"
	DriveUsageRemoved   = "REMOVED"

	// Drive type
	DriveTypeHDD  = "HDD"
	DriveTypeSSD  = "SSD"
	DriveTypeNVMe = "NVME"

	// Drive annotations
	DriveAnnotationRemoval            = "removal"
	DriveAnnotationRemovalReady       = "ready"
	DriveAnnotationVolumeStatusPrefix = "status"
	// Deprecated annotations
	DriveAnnotationReplacement = "replacement"

	// Volume operational status
	OperationalStatusOperative   = "OPERATIVE"
	OperationalStatusInoperative = "INOPERATIVE"
	OperationalStatusStaging     = "STAGING"
	OperationalStatusMissing     = "MISSING"
	OperationalStatusMaintenance = "MAINTENANCE"
	OperationalStatusUnknown     = "UNKNOWN"

	// Volume Usage status
	VolumeUsageInUse     = DriveUsageInUse
	VolumeUsageReleasing = DriveUsageReleasing
	VolumeUsageReleased  = DriveUsageReleased
	VolumeUsageFailed    = DriveUsageFailed

	// Release Volume annotations
	VolumeAnnotationRelease       = "release"
	VolumeAnnotationReleaseDone   = "done"
	VolumeAnnotationReleaseFailed = "failed"
	VolumeAnnotationReleaseStatus = "status"

	//Volume expansion annotations
	VolumePreviousStatus   = "expansion/previous-status"
	VolumePreviousCapacity = "expansion/previous-capacity"
	// TODO Mount status?
	// Volume mode
	ModeRAW     = "RAW"
	ModeRAWPART = "RAW_PART"
	ModeFS      = "FS"

	//LVG annotations
	LVGFreeSpaceAnnotation = "lvg/free-space"

	// Volume location type
	LocationTypeDrive = "DRIVE"
	LocationTypeLVM   = "LVM"
	LocationTypeNVMe  = "NVME"

	// Available Capacity Reservation statuses
	ReservationRequested = "REQUESTED"
	ReservationConfirmed = "RESERVED"
	ReservationRejected  = "REJECTED"
	ReservationCancelled = "CANCELLED"

	// CSI StorageClass
	// For volumes with storage class 'ANY' CSI will pick any AC except LVG AC
	StorageClassAny       = "ANY"
	StorageClassHDD       = "HDD"
	StorageClassSSD       = "SSD"
	StorageClassNVMe      = "NVME"
	StorageClassHDDLVG    = "HDDLVG"
	StorageClassSSDLVG    = "SSDLVG"
	StorageClassNVMeLVG   = "NVMELVG"
	StorageClassSystemLVG = "SYSLVG"

	LocateStart  = int32(0)
	LocateStop   = int32(1)
	LocateStatus = int32(2)

	LocateStatusOff          = int32(0)
	LocateStatusOn           = int32(1)
	LocateStatusNotAvailable = int32(2)

	DockerImageKernelVersion = "5.4"
)
