package sc

import (
	"sync"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

type SsdSC struct {
	DefaultDASC
}

var (
	ssdMU         sync.Mutex
	ssdSCInstance *SsdSC
)

func GetSSDSCInstance() *SsdSC {
	if ssdSCInstance == nil {
		ssdMU.Lock()
		defer ssdMU.Unlock()

		if ssdSCInstance == nil {
			ssdSCInstance = &SsdSC{DefaultDASC{executor: &base.Executor{}}}
		}
	}
	return ssdSCInstance
}

func (s *SsdSC) SetSDDSCExecutor(executor base.Executor) {
	s.executor = &executor
}
