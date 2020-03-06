package base

// TODO: TBD where store constants

const (
	// gRPC endpoints settings
	DefaultHWMgrPort             = 8888
	DefaultHWMgrEndpoint         = "tcp://localhost:8888"
	DefaultVolumeManagerPort     = 9999
	DefaultVolumeManagerEndpoint = "tcp://localhost:9999"

	// Linux Utils and VolumeManager constants
	DriveTypeDisk          = "disk"
	DefaultDiscoverTimeout = 120
)
