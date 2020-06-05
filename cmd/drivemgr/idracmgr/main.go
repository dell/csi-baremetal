package main

import (
	"flag"
	"time"

	dmsetup "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/cmd/drivemgr"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/ipmi"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/drivemgr/idracmgr"
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

	e := &command.Executor{}
	e.SetLogger(logger)

	ipmiTool := ipmi.NewIPMI(e)
	ip := ipmiTool.GetBmcIP()
	if ip == "" {
		logger.Fatal("IDRAC IP is not found")
	}

	driveMgr := idracmgr.NewIDRACManager(logger, 10*time.Second, "root", "passwd", ip)

	dmsetup.SetupAndRunDriveMgr(driveMgr, serverRunner, nil, logger)
}
