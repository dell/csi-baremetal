package sc

import (
	"sync"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

// SsdSC is StorageClass implementation for SSD drives
type SsdSC struct {
	DefaultDASC
}

var (
	ssdMU         sync.Mutex
	ssdSCInstance *SsdSC
)

// GetSSDSCInstance singleton instance getter for SsdSC
// Receives logrus logger
// Returns instance of SsdSC
func GetSSDSCInstance(logger *logrus.Logger) *SsdSC {
	if ssdSCInstance == nil {
		ssdMU.Lock()
		defer ssdMU.Unlock()

		if ssdSCInstance == nil {
			ssdSCInstance = &SsdSC{DefaultDASC{executor: &base.Executor{}}}
			hddSCInstance.executor.SetLogger(logger)
			hddSCInstance.SetLogger(logger, "SSDSC")
		}
	}
	return ssdSCInstance
}

// SetSDDSCExecutor sets cmd executor to SsdSC
// Receives cmd executor
func (s *SsdSC) SetSDDSCExecutor(executor base.CmdExecutor) {
	s.executor = executor
}
