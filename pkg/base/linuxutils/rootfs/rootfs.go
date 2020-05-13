package rootfs

import (
	"fmt"
	"strings"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
)

// CheckSpaceCmdImpl check free space on system drive in unit of --block-size
const CheckSpaceCmdImpl = "df --output=target,avail --block-size=%s"

// Agent is the struct to interact with root file system
type Agent struct {
	e command.CmdExecutor
}

// NewRootFsAgent is the constructor for Agent struct
// Receives an instance of CmdExecutor
// Returns an instance of Agent
func NewRootFsAgent(e command.CmdExecutor) *Agent {
	return &Agent{e: e}
}

// GetRootFsSpace calls df command and check available space on root fs
// Returns free bytes on root fs as int64 or error if something went wrong
func (rf *Agent) GetRootFsSpace() (int64, error) {
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
				freeBytes, err := util.StrToBytes(output[sizeIdx])
				if err != nil {
					return 0, err
				}
				return freeBytes, nil
			}
		}
	}
	return 0, fmt.Errorf("wrong df output %s", stodout)
}
