package sc

import (
	"sync"
)

type SSDSC struct {
	DefaultDASC
}

var (
	ssdMU sync.Mutex
	ssdSC *SSDSC
)

func (s SSDSC) GetInstance() *SSDSC {
	if ssdSC == nil {
		ssdMU.Lock()
		defer ssdMU.Unlock()

		if ssdSC == nil {
			ssdSC = &SSDSC{}
		}
	}
	return ssdSC
}
