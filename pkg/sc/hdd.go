package sc

import (
	"sync"
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
			hddSCInstance = &HddSC{}
		}
	}
	return hddSCInstance
}
