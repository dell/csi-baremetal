package sc

import (
	"sync"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

// HddSC is StorageClass implementation for HDD drives
type HddSC struct {
	DefaultDASC
}

var (
	hddMU         sync.Mutex
	hddSCInstance *HddSC
)

// GetHDDSCInstance singleton instance getter for HddSC
// Receives logrus logger
// Returns instance of HddSC
func GetHDDSCInstance(logger *logrus.Logger) *HddSC {
	if hddSCInstance == nil {
		hddMU.Lock()
		defer hddMU.Unlock()

		if hddSCInstance == nil {
			hddSCInstance = &HddSC{DefaultDASC{executor: &base.Executor{}}}
			hddSCInstance.executor.SetLogger(logger)
			hddSCInstance.SetLogger(logger, "HDDSC")
		}
	}
	return hddSCInstance
}

// SetHDDSCExecutor sets cmd executor to HddSC
// Receives cmd executor
func (h *HddSC) SetHDDSCExecutor(executor base.Executor) {
	h.executor = &executor
}
