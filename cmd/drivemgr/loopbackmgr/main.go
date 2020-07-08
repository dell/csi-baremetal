package main

import (
	"flag"
	"fmt"

	dmsetup "github.com/dell/csi-baremetal.git/cmd/drivemgr"
	"github.com/dell/csi-baremetal.git/pkg/base"
	"github.com/dell/csi-baremetal.git/pkg/base/command"
	"github.com/dell/csi-baremetal.git/pkg/base/rpc"
	"github.com/dell/csi-baremetal.git/pkg/drivemgr/loopbackmgr"
)

var (
	endpoint = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "DriveManager Endpoint")
	logPath  = flag.String("logpath", "", "log path for DriveManager")
	logLevel = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
)

func main() {
	flag.Parse()

	logger, err := base.InitLogger(*logPath, *logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	// Server is insecure for now because credentials are nil
	serverRunner := rpc.NewServerRunner(nil, *endpoint, logger)

	e := &command.Executor{}
	e.SetLogger(logger)

	driveMgr := loopbackmgr.NewLoopBackManager(e, logger)
	// initialize
	if err = driveMgr.Init(); err != nil {
		logger.Fatalf("Failed to initialize LoopBack drive manager: %v", err)
	}

	dmsetup.SetupAndRunDriveMgr(driveMgr, serverRunner, driveMgr.CleanupLoopDevices, logger)
}
