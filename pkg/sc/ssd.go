package sc

import (
	"sync"
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
			ssdSCInstance = &SsdSC{}
		}
	}
	return ssdSCInstance
}
