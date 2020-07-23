package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/scheduler"
)

var (
	port = flag.Int("port", base.DefaultExtenderPort, "Port for service")
	logLevel = flag.String("logLevel", base.InfoLevel, "Log level")
)

func main() {
	flag.Parse()
	logger, _ := base.InitLogger("", *logLevel)
	logger.Info("Starting scheduler extender for CSI-Baremetal ...")

	extender := scheduler.NewExtender(logger)

	logger.Infof("Starting extender on port %d ...", *port)
	http.HandleFunc("/filter", extender.FilterHandler)
	logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
