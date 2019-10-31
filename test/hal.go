package main

/*
// #cgo LDFLAGS: -L/opt/emc/lib64 -lhalHelper
// #include </workspace/nile-hal/src/chal/hal-helper.hxx>
import "C"
*/

import (
	"fmt"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
)

func main() {
	halDisks := util.AllDisks()

	fmt.Println(halDisks)
}
