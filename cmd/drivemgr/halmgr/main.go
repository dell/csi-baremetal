package main

import (
	"flag"

	dmsetup "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/cmd/drivemgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/drivemgr/halmgr"
)

var (
	endpoint    = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "DriveManager Endpoint")
	logPath     = flag.String("logpath", "", "log path for DriveManager")
	verboseLogs = flag.Bool("verbose", false, "Debug mode in logs")
)

func main() {
	flag.Parse()

	logger, err := base.InitLogger(*logPath, *verboseLogs)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	// Server is insecure for now because credentials are nil
	serverRunner := rpc.NewServerRunner(nil, *endpoint, logger)

	driveMgr := halmgr.NewHALManager(logger)
	dmsetup.SetupAndRunDriveMgr(driveMgr, serverRunner, nil, logger)
}
