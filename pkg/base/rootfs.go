package base

import (
	"fmt"
	"strings"
)

const CheckSpaceCmdImpl = "df --output=target,avail --block-size=%s"

type RootFsUtils struct {
	e CmdExecutor
}

func NewRootFsUtils(e CmdExecutor) *RootFsUtils {
	return &RootFsUtils{e: e}
}

//checkRootFsSpace calls df command and check available space on root fs
func (rf *RootFsUtils) CheckRootFsSpace() (int64, error) {
	/*Example output
	Mounted on                       Avail
	/dev                             2413M
	/run                              437M
	/                               10283M
	*/
	stodout, _, err := rf.e.RunCmd(fmt.Sprintf(CheckSpaceCmdImpl, "M"))
	if err != nil {
		return 0, err
	}
	split := strings.Split(stodout, "\n")
	//Skip headers Mounter on and Available
	for j := 1; j < len(split); j++ {
		output := strings.Split(strings.TrimSpace(split[j]), " ")
		if len(output) > 1 {
			if strings.Contains(output[0], "/") && len(output[0]) == 1 {
				//Try to get size from string, e.g. "/    10283M", size has the last index in the string
				sizeIdx := len(output) - 1
				freeBytes, err := StrToBytes(output[sizeIdx])
				if err != nil {
					return 0, err
				}
				return freeBytes, nil
			}
		}
	}
	return 0, fmt.Errorf("wrong df output %s", stodout)
}
