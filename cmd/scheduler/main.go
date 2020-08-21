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
	namespace      = flag.String("namespace", "", "Namespace in which Node Service service run")
	provisioner    = flag.String("provisioner", "", "Provisioner name which storage classes extener will be observing")
	port           = flag.Int("port", base.DefaultExtenderPort, "Port for service")
	certFile       = flag.String("certFile", "", "path to the cert file")
	privateKeyFile = flag.String("privateKeyFile", "", "path to the private key file")
	logLevel       = flag.String("loglevel", base.InfoLevel, "Log level")
)

func main() {
	flag.Parse()
	logger, _ := base.InitLogger("", *logLevel)
	logger.Info("Starting scheduler extender for CSI-Baremetal ...")

	extender, err := scheduler.NewExtender(logger, *namespace, *provisioner)
	if err != nil {
		logger.Fatalf("Fail to create extender: %v", err)
	}

	logger.Infof("Starting extender on port %d ...", *port)
	http.HandleFunc("/filter", extender.FilterHandler)

	var addr = fmt.Sprintf(":%d", *port)
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
