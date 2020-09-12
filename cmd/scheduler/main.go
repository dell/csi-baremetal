package main

import (
	"flag"
	"fmt"
	extender2 "github.com/dell/csi-baremetal/pkg/scheduling/extender"
	"net/http"
	"os"

	"github.com/dell/csi-baremetal/pkg/base"
)

var (
	namespace      = flag.String("namespace", "", "Namespace in which Node Service service run")
	provisioner    = flag.String("provisioner", "", "Provisioner name which storage classes extener will be observing")
	port           = flag.Int("port", base.DefaultExtenderPort, "Port for service")
	certFile       = flag.String("certFile", "", "path to the cert file")
	privateKeyFile = flag.String("privateKeyFile", "", "path to the private key file")
	logLevel       = flag.String("loglevel", base.InfoLevel, "Log level")
)

// todo these values a defined in config file and should be passed as parameters
const (
	FILTER_PATTERN string = "/filter"
	BIND_PATTERN string = "/bind"
)

func main() {
	flag.Parse()
	logger, _ := base.InitLogger("", *logLevel)
	logger.Info("Starting scheduler extender for CSI-Baremetal ...")

	extender, err := extender2.NewExtender(logger, *namespace, *provisioner)
	if err != nil {
		logger.Fatalf("Fail to create extender: %v", err)
	}

	logger.Infof("Starting extender on port %d ...", *port)
	// filter stage
	logger.Info("Registering for filter stage ... ")
	http.HandleFunc(FILTER_PATTERN, extender.FilterHandler)

	// bind stage
	logger.Infof("Registering for bind stage ... ")
	http.HandleFunc(BIND_PATTERN, extender.BindHandler)


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
