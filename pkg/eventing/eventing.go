package eventing

// Event types
const (
	InfoType     = "Info"
	WarningType  = "Warning"
	ErrorType    = "Error"
	CriticalType = "Critical"
)

// Volume event reason list
const (
	VolumeDiscovered    = "VolumeDiscovered"
	VolumeBadHealth     = "VolumeBadHealth"
	VolumeUnknownHealth = "VolumeUnknownHealth"
	VolumeGoodHealth    = "VolumeGoodHealth"
	VolumeSuspectHealth = "VolumeSuspectHealth"

	DriveDiscovered    = "DriveDiscovered"
	DriveHealthSuspect = "DriveHealthSuspect"
	DriveHealthFailure = "DriveHealthFailure"
	DriveHealthGood    = "DriveHealthGood"
	DriveHealthUnknown = "DriveHealthUnknown"
	DriveStatusOnline  = "DriveStatusOnline"
	DriveStatusOffline = "DriveStatusOffline"
)
