package sc

import (
	"sync"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

type HddSC struct {
	DefaultDASC
}

var (
	hddMU         sync.Mutex
	hddSCInstance *HddSC
)

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

func (h *HddSC) SetHDDSCExecutor(executor base.Executor) {
	h.executor = &executor
}
