package sc

import (
	"sync"
)

type HDDSC struct {
	DefaultDASC
}

var (
	hddMU sync.Mutex
	hddSC *HDDSC
)

func (h HDDSC) GetInstance() *HDDSC {
	if hddSC == nil {
		hddMU.Lock()
		defer hddMU.Unlock()

		if hddSC == nil {
			hddSC = &HDDSC{}
		}
	}
	return hddSC
}
