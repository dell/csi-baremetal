package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/scheduler"
)

var (
	port           = flag.Int("port", base.DefaultExtenderPort, "Port for service")
	certFile       = flag.String("certFile", "", "path to the cert file")
	privateKeyFile = flag.String("privateKeyFile", "", "path to the private key file")
	logLevel       = flag.String("loglevel", base.InfoLevel, "Log level")
)

func main() {
	flag.Parse()
	logger, _ := base.InitLogger("", *logLevel)
	logger.Info("Starting scheduler extender for CSI-Baremetal ...")

	extender := scheduler.NewExtender(logger)

	logger.Infof("Starting extender on port %d ...", *port)
	http.HandleFunc("/filter", extender.FilterHandler)

	var (
		addr = fmt.Sprintf(":%d", *port)
		err  error
	)

	if *certFile != "" && *privateKeyFile != "" {
		logger.Info("Handle with TLS")
		err = http.ListenAndServeTLS(addr, *certFile, *privateKeyFile, nil)
	} else {
		err = http.ListenAndServe(addr, nil)
	}

	if err != nil {
		logger.Fatal(err)
	}
	os.Exit(0)
}
