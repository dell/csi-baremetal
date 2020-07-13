package main

import (
	"flag"
	"fmt"
	"time"

	dmsetup "github.com/dell/csi-baremetal/cmd/drivemgr"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/ipmi"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/drivemgr/idracmgr"
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

	ipmiTool := ipmi.NewIPMI(e)
	ip := ipmiTool.GetBmcIP()
	if ip == "" {
		logger.Fatal("IDRAC IP is not found")
	}

	driveMgr := idracmgr.NewIDRACManager(logger, 10*time.Second, "root", "passwd", ip)

	dmsetup.SetupAndRunDriveMgr(driveMgr, serverRunner, nil, logger)
}
