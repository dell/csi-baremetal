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

	Version               = "v1"
	// TODO: remove that constanta with "Old" prefix as a part of https://github.com/dell/csi-baremetal/issues/134
	CSICRsGroupVersionOld = "baremetal-csi.dellemc.com"
	APIV1VersionOld       = "baremetal-csi.dellemc.com/v1"

	CSICRsGroupVersion    = "csi-baremetal.dell.com"
	APIV1Version          = "csi-baremetal.dell.com/v1"
	Creating              = "creating"
	Created               = "created"
	VolumeReady           = "volumeReady"
	Published             = "published"
	Removing              = "removing"
	Removed               = "removed"
	Failed                = "failed"

	// Health statuses
	HealthUnknown = "UNKNOWN"
	HealthGood    = "GOOD"
	HealthSuspect = "SUSPECT"
	HealthBad     = "BAD"

	// Drive status
	DriveStatusOnline  = "ONLINE"
	DriveStatusOffline = "OFFLINE"

	// Drive type
	DriveTypeHDD  = "HDD"
	DriveTypeSSD  = "SSD"
	DriveTypeNVMe = "NVME"

	// Volume operational status
	OperationalStatusOperative     = "OPERATIVE"
	OperationalStatusInoperative   = "INOPERATIVE"
	OperationalStatusStaging       = "STAGING"
	OperationalStatusMissing       = "MISSING"
	OperationalStatusRemoving      = "REMOVING"
	OperationalStatusReadyToRemove = "READY_TO_REMOVE"
	OperationalStatusFailToRemove  = "FAIL_TO_REMOVE"
	OperationalStatusMaintenance   = "MAINTENANCE"
	OperationalStatusRemoved       = "REMOVED"
	OperationalStatusUnknown       = "UNKNOWN"

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
