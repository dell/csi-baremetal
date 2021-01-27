/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/scheduler/extender"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	namespace         = flag.String("namespace", "", "Namespace in which Node Service service run")
	provisioner       = flag.String("provisioner", "", "Provisioner name which storage classes extener will be observing")
	port              = flag.Int("port", base.DefaultExtenderPort, "Port for service")
	certFile          = flag.String("certFile", "", "path to the cert file")
	privateKeyFile    = flag.String("privateKeyFile", "", "path to the private key file")
	logLevel          = flag.String("loglevel", base.InfoLevel, "Log level")
	useNodeAnnotation = flag.Bool("usenodeannotation", false,
		"Whether extender should read id from node annotation and use it as id for all CRs or not")
	metricsAddress = flag.String("metrics-address", "", "The TCP network address where the prometheus metrics endpoint will run"+
		"(example: :8080 which corresponds to port 8080 on local host). The default is empty string, which means metrics endpoint is disabled.")
	metricspath = flag.String("metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is /metrics.")
)

// TODO should be passed as parameters https://github.com/dell/csi-baremetal/issues/78
const (
	FilterPattern     string = "/filter"
	PrioritizePattern string = "/prioritize"
	BindPattern       string = "/bind"
)

func main() {
	flag.Parse()
	logger, _ := base.InitLogger("", *logLevel)
	logger.Info("Starting scheduler extender for CSI-Baremetal ...")

	if *metricspath != "" {
		go func() {
			http.Handle(*metricspath, promhttp.Handler())
			if err := http.ListenAndServe(*metricsAddress, nil); err != nil {
				logger.Warnf("metric http returned: %s ", err)
			}
		}()
	}

	featureConf := featureconfig.NewFeatureConfig()
	featureConf.Update(featureconfig.FeatureNodeIDFromAnnotation, *useNodeAnnotation)

	newExtender, err := extender.NewExtender(logger, *namespace, *provisioner, featureConf)
	if err != nil {
		logger.Fatalf("Fail to create extender: %v", err)
	}

	logger.Infof("Starting extender on port %d ...", *port)
	// filter stage
	logger.Info("Registering for filter stage ... ")
	http.HandleFunc(FilterPattern, newExtender.FilterHandler)

	// prioritize stage
	logger.Info("Registering for prioritize stage ... ")
	http.HandleFunc(PrioritizePattern, newExtender.PrioritizeHandler)

	// bind stage
	logger.Infof("Registering for bind stage ... ")
	http.HandleFunc(BindPattern, newExtender.BindHandler)

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
