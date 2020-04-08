package base

import "time"

const (
	// gRPC endpoints settings
	DefaultHWMgrEndpoint     = "tcp://localhost:8888"
	DefaultHWMgrPort         = 8888
	DefaultVMMgrIP           = "127.0.0.1"
	DefaultVolumeManagerPort = 9999

	// Linux Utils and VolumeManager constants
	DriveTypeDisk          = "disk"
	DefaultDiscoverTimeout = 120

	// timeout in which we expect that any operation should be finished
	DefaultTimeoutForOperations = 10 * time.Minute
	DefaultTimeoutForCR         = 1 * time.Minute
)
