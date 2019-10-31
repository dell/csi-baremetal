package main

import (
	"flag"
	"log"
	"os"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/driver"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/sirupsen/logrus"
)

var (
	endpoint   = flag.String("endpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", "baremetal-csi", "name of the driver")
	nodeID     = flag.String("nodeid", "", "node id")
	startRest  = flag.Bool("startrest", true, "run rest server on port 9999 for report disks")
)

const logFile = "/var/log/logrus.log"

func main() {
	flag.Parse()
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		logrus.SetOutput(os.Stdout)
	}
	logrus.SetOutput(f)
	d, err := driver.NewDriver(*endpoint, *driverName, *nodeID)
	if err != nil {
		logrus.Error("Could not create driver:", err)
	}

	if *startRest {
		logrus.Info("Starting rest ...")
		go util.StartRest()
	}

	logrus.Info("Starting driver")
	if err := d.Run(); err != nil {
		log.Fatalln(err)
	}
}
