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

	//Volume expansion annotations
	VolumePreviousStatus   = "expansion/previous-status"
	VolumePreviousCapacity = "expansion/previous-capacity"

	//LVG annotations
	LVGFreeSpaceAnnotation = "lvg/free-space"

	// TODO Mount status?

	DockerImageKernelVersion = "5.4"
)

type CSIStatus string

const (
	Creating    CSIStatus = "CREATING"
	Created     CSIStatus = "CREATED"
	VolumeReady CSIStatus = "VOLUME_READY"
	Published   CSIStatus = "PUBLISHED"
	Removing    CSIStatus = "REMOVING"
	Removed     CSIStatus = "REMOVED"
	Failed      CSIStatus = "FAILED"
	Empty       CSIStatus = ""
	Resizing    CSIStatus = "RESIZING"
	Resized     CSIStatus = "RESIZED"
)

func MatchCSIStatus(cs CSIStatus) string { return string(cs) }

type DriveAnnotation string

const (
	DriveAnnotationRemoval            DriveAnnotation = "removal"
	DriveAnnotationRemovalReady       DriveAnnotation = "ready"
	DriveAnnotationVolumeStatusPrefix DriveAnnotation = "status"
	DriveAnnotationReplacement        DriveAnnotation = "replacement" // Deprecated
)

func MatchDriveAnnotation(da DriveAnnotation) string { return string(da) }

type DriveType string

const (
	DriveTypeHDD  DriveType = "HDD"
	DriveTypeSSD  DriveType = "SSD"
	DriveTypeNVMe DriveType = "NVME"
)

func MatchDriveType(dt DriveType) string { return string(dt) }

type DriveStatus string

const (
	DriveStatusOnline  DriveStatus = "ONLINE"
	DriveStatusOffline DriveStatus = "OFFLINE"
	DriveStatusFailed  DriveStatus = "FAILED"
)

func MatchDriveStatus(ds DriveStatus) string { return string(ds) }

type DriveUsage string

const (
	DriveUsageInUse     DriveUsage = "IN_USE"
	DriveUsageReleasing DriveUsage = "RELEASING"
	DriveUsageReleased  DriveUsage = "RELEASED"
	DriveUsageFailed    DriveUsage = "FAILED"
	DriveUsageRemoving  DriveUsage = "REMOVING"
	DriveUsageRemoved   DriveUsage = "REMOVED"
)

func MatchDriveUsage(du DriveUsage) string { return string(du) }

type HealthStatus string

const (
	HealthUnknown HealthStatus = "UNKNOWN"
	HealthGood    HealthStatus = "GOOD"
	HealthSuspect HealthStatus = "SUSPECT"
	HealthBad     HealthStatus = "BAD"
)

func MatchHealthStatus(hs HealthStatus) string { return string(hs) }

type LocateStatus int32

const (
	LocateStatusOn LocateStatus = iota
	LocateStatusOff
	LocateStatusNA
)

func MatchLocateStatus(ls LocateStatus) int32 { return int32(ls) }

type LocationType string

const (
	LocationTypeDrive LocationType = "DRIVE"
	LocationTypeLVM   LocationType = "LVM"
	LocationTypeNVMe  LocationType = "NVME"
)

func MatchLocationType(lt LocationType) string { return string(lt) }

type ReservationStatus string

const (
	ReservationRequested ReservationStatus = "REQUESTED"
	ReservationConfirmed ReservationStatus = "RESERVED"
	ReservationRejected  ReservationStatus = "REJECTED"
	ReservationCancelled ReservationStatus = "CANCELLED"
)

func MatchReservationStatus(rs ReservationStatus) string { return string(rs) }

type StorageClass string

const (
	StorageClassAny       StorageClass = "ANY"
	StorageClassHDD       StorageClass = "HDD"
	StorageClassSSD       StorageClass = "SSD"
	StorageClassNVMe      StorageClass = "NVME"
	StorageClassHDDLVG    StorageClass = "HDDLVG"
	StorageClassSSDLVG    StorageClass = "SSDLVG"
	StorageClassNVMeLVG   StorageClass = "NVMELVG"
	StorageClassSystemLVG StorageClass = "SYSLVG"
)

func MatchStorageClass(sc StorageClass) string { return string(sc) }

type VolumeAnnotation string

const (
	VolumeAnnotationRelease       VolumeAnnotation = "release"
	VolumeAnnotationReleaseDone   VolumeAnnotation = "done"
	VolumeAnnotationReleaseFailed VolumeAnnotation = "failed"
	VolumeAnnotationReleaseStatus VolumeAnnotation = "status"
)

func MatchVolumeAnnotation(va VolumeAnnotation) string { return string(va) }

type VolumeMode string

const (
	ModeRAW     VolumeMode = "RAW"
	ModeRAWPART VolumeMode = "RAW_PART"
	ModeFS      VolumeMode = "FS"
	ModeEmpty   VolumeMode = ""
)

func MatchVolumeMode(vm VolumeMode) string { return string(vm) }

type VolumeOperationalStatus string

const (
	OperationalStatusOperative   VolumeOperationalStatus = "OPERATIVE"
	OperationalStatusInoperative VolumeOperationalStatus = "INOPERATIVE"
	OperationalStatusStaging     VolumeOperationalStatus = "STAGING"
	OperationalStatusMissing     VolumeOperationalStatus = "MISSING"
	OperationalStatusMaintenance VolumeOperationalStatus = "MAINTENANCE"
	OperationalStatusUnknown     VolumeOperationalStatus = "UNKNOWN"
)

func MatchVolumeOperationalStatus(vos VolumeOperationalStatus) string { return string(vos) }

type VolumeUsage string

const (
	VolumeUsageInUse     = VolumeUsage(DriveUsageInUse)
	VolumeUsageReleasing = VolumeUsage(DriveUsageReleasing)
	VolumeUsageReleased  = VolumeUsage(DriveUsageReleased)
	VolumeUsageFailed    = VolumeUsage(DriveUsageFailed)
)

func MatchVolumeUsage(vu VolumeUsage) string { return string(vu) }
