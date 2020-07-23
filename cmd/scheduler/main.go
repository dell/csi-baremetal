package main

import (
	"fmt"
	"net/http"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/scheduler"
)

const (
	defaultPort = "8888"
)

func main()  {
	logger, _ := base.InitLogger("", "debug")
	logger.Info("Starting scheduler extender for CSI-Baremetal ...")

	extender := scheduler.NewExtender(logger)

	http.HandleFunc("/filter", extender.FilterHandler)
	logger.Infof("Starting extender on port %s ...", defaultPort)
	logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", defaultPort), nil))
}