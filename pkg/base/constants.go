package base

// TODO: TBD where store constants
const (
	// gRPC endpoints settings
	DefaultHWMgrEndpoint         = "tcp://localhost:8888"
	DefaultHWMgrPort             = 8888
	DefaultVolumeManagerEndpoint = "tcp://localhost:9999"
	DefaultVolumeManagerPort     = 9999

	// Linux Utils and VolumeManager constants
	DriveTypeDisk          = "disk"
	DefaultDiscoverTimeout = 120
)
