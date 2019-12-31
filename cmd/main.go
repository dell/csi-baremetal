package main

import (
	"flag"
	"log"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/driver"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/rest"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
)

var (
	endpoint   = flag.String("endpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", "baremetal-csi", "name of the driver")
	nodeID     = flag.String("nodeid", "", "node id")
	lvmMode    = flag.Bool("lmvmode", false, "whether to use LVM or no (use block device as is) ")
	isCont     = flag.Bool("controller", false, "whether binary run as controller or no (start REST)")
)

const restServicePort = 9999
const vgName = "csivg" // TODO: do not hardcode it

func main() {
	logger := util.CreateLogger("cmd")

	flag.Parse()

	d, err := driver.NewDriver(*endpoint, *driverName, *nodeID, *lvmMode)
	if err != nil {
		logger.Error("Could not create driver:", err)
	}

	// init LVM only on nodes which serves CSI node plugin service
	if *lvmMode && !*isCont {
		logger.Info("Initializing LVM ...")
		vg := lvm.VolumeGroup{
			Name:       vgName,
			DiskFilter: util.AllDisksWithoutPartitions,
		}
		err := vg.InitVG()
		if err != nil {
			logger.Fatalf("Could not init LVM: %v", err)
		}
		logger.Info("... LVM initialized !!!")
	}

	if !*isCont {
		s := rest.Server{
			Port:      restServicePort,
			HandleLVM: *lvmMode,
		}
		logger.Info("Starting rest ...")
		go s.Start()
	}

	logger.Info("Starting driver")
	if err := d.Run(); err != nil {
		log.Fatalln(err)
	}
}
