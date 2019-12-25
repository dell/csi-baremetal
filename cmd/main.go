package main

import (
	"flag"
	"log"
	"os"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/driver"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/rest"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/sirupsen/logrus"
)

var (
	endpoint   = flag.String("endpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", "baremetal-csi", "name of the driver")
	nodeID     = flag.String("nodeid", "", "node id")
	lvmMode    = flag.Bool("lmvmode", false, "whether to use LVM or no (use block device as is) ")
	isCont     = flag.Bool("controller", false, "whether binary run as controller or no (start REST)")
)

const logFile = "/var/log/logrus.log"
const restServicePort = 9999
const vgName = "csivg" // TODO: do not hardcode it

func main() {
	flag.Parse()

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		logrus.SetOutput(os.Stdout)
	}

	logrus.SetOutput(f)

	d, err := driver.NewDriver(*endpoint, *driverName, *nodeID, *lvmMode)
	if err != nil {
		logrus.Error("Could not create driver:", err)
	}

	// init LVM only on nodes which serves CSI node plugin service
	if *lvmMode && !*isCont {
		logrus.Info("Initializing LVM ...")
		vg := lvm.VolumeGroup{
			Name:       vgName,
			DiskFilter: util.AllDisksWithoutPartitions,
		}
		err := vg.InitVG()
		if err != nil {
			logrus.Fatalf("Could not init LVM: %v", err)
		}
		logrus.Info("... LVM initialized !!!")
	}

	if !*isCont {
		s := rest.Server{
			Port:      restServicePort,
			HandleLVM: *lvmMode,
		}
		logrus.Info("Starting rest ...")
		go s.Start()
	}

	logrus.Info("Starting driver")
	if err := d.Run(); err != nil {
		log.Fatalln(err)
	}
}
