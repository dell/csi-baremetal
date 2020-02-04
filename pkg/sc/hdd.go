package sc

import (
	"sync"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

type HddSC struct {
	DefaultDASC
}

var (
	hddMU         sync.Mutex
	hddSCInstance *HddSC
)

func GetHDDSCInstance() *HddSC {
	if hddSCInstance == nil {
		hddMU.Lock()
		defer hddMU.Unlock()

		if hddSCInstance == nil {
			hddSCInstance = &HddSC{DefaultDASC{executor: &base.Executor{}}}
		}
	}
	return hddSCInstance
}

func (h *HddSC) SetHDDSCExecutor(executor base.Executor) {
	h.executor = &executor
}
