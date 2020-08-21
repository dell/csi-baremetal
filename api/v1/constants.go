package v1

const (
	Version            = "v1"
	CSICRsGroupVersion = "baremetal-csi.dellemc.com"
	APIV1Version       = "baremetal-csi.dellemc.com/v1"
	Creating           = "creating"
	Created            = "created"
	VolumeReady        = "volumeReady"
	Published          = "published"
	Removing           = "removing"
	Removed            = "removed"
	Failed             = "failed"

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
	StorageClassAny          = "ANY"
	StorageClassHDD          = "HDD"
	StorageClassSSD          = "SSD"
	StorageClassNVMe         = "NVME"
	StorageClassHDDLVG       = "HDDLVG"
	StorageClassSSDLVG       = "SSDLVG"
	StorageClassNVMeLVG      = "NVMELVG"
	StorageClassSystemSSDLVG = "SYSTEM-SSDLVG"
)
