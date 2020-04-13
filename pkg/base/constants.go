package base

import "time"

const (
	// gRPC endpoints settings
	DefaultHWMgrEndpoint     = "tcp://localhost:8888"
	DefaultVMMgrIP           = "127.0.0.1"
	DefaultVolumeManagerPort = 9999

	RomDeviceType   = "rom"
	KubeletRootPath = "/var/lib/kubelet/pods"
	// points on SSD drive
	NonRotationalNum = "0"

	// timeout in which we expect that any operation should be finished
	DefaultTimeoutForOperations = 10 * time.Minute

	SystemDriveAsLocation = "system drive"
)
