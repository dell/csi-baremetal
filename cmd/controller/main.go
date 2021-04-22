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

// Package for main function of Controller
package main

import (
	"flag"
	"fmt"

	"net"
	"net/http"
	"strconv"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/container-storage-interface/spec/lib/go/csi"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// +kubebuilder:scaffold:imports
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/controller"
	"github.com/dell/csi-baremetal/pkg/crcontrollers/reservation"
	"github.com/dell/csi-baremetal/pkg/metrics"
)

var (
	namespace  = flag.String("namespace", "", "Namespace in which controller service run")
	healthIP   = flag.String("healthip", base.DefaultHealthIP, "IP for health service")
	healthPort = flag.Int("healthport", base.DefaultHealthPort, "Port for health service")
	endpoint   = flag.String("endpoint", "", "Endpoint for controller service")
	logPath    = flag.String("logpath", "", "Log path for Controller service")
	useACRs    = flag.Bool("extender", false,
		"Whether controller should read AvailableCapacityReservation CR during CreateVolume request or not")
	logLevel = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
	metricsAddress = flag.String("metrics-address", "", "The TCP network address where the prometheus metrics endpoint will run"+
		"(example: :8080 which corresponds to port 8080 on local host). The default is empty string, which means metrics endpoint is disabled.")
	metricspath = flag.String("metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is /metrics.")
)

func main() {
	flag.Parse()

	featureConf := featureconfig.NewFeatureConfig()
	featureConf.Update(featureconfig.FeatureACReservation, *useACRs)

	var enableMetrics bool
	if *metricspath != "" {
		enableMetrics = true
	}

	logger, err := base.InitLogger(*logPath, *logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Starting controller ...")

	csiControllerServer := rpc.NewServerRunner(nil, *endpoint, enableMetrics, logger)

	k8SClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}
	kubeClient := k8s.NewKubeClient(k8SClient, logger, *namespace)
	controllerService := controller.NewControllerService(kubeClient, logger, featureConf)
	handler := util.NewSignalHandler(logger)
	go handler.SetupSIGTERMHandler(csiControllerServer)

	csi.RegisterIdentityServer(csiControllerServer.GRPCServer, controllerService)
	csi.RegisterControllerServer(csiControllerServer.GRPCServer, controllerService)

	if enableMetrics {
		grpc_prometheus.Register(csiControllerServer.GRPCServer)
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpc_prometheus.EnableClientHandlingTimeHistogram()
		prometheus.MustRegister(metrics.BuildInfo)

		go func() {
			http.Handle(*metricspath, promhttp.Handler())
			if err := http.ListenAndServe(*metricsAddress, nil); err != nil {
				logger.Warnf("metric http returned: %s ", err)
			}
		}()
	}

	// todo make ACR feature mandatory and get rid of feature flag https://github.com/dell/csi-baremetal/issues/366
	// check ACR feature flag
	if featureConf.IsEnabled(featureconfig.FeatureACReservation) {
		// create Reservation manager
		reservationManager, err := createReservationManager(kubeClient, logger)
		if err != nil {
			logger.Fatal(err)
		}
		// start Reservation manager
		go func() {
			logger.Info("Starting Reservation Controller")
			if err := reservationManager.Start(ctrl.SetupSignalHandler()); err != nil {
				logger.Fatalf("Reservation Controller failed with error: %v", err)
			}
		}()
	}

	go func() {
		logger.Info("Starting Controller Health server")
		if err := util.SetupAndStartHealthCheckServer(
			controllerService, logger,
			"tcp://"+net.JoinHostPort(*healthIP, strconv.Itoa(*healthPort))); err != nil {
			logger.Fatalf("Controller service failed with error: %v", err)
		}
	}()
	logger.Info("Starting CSIControllerService")
	if err := csiControllerServer.RunServer(); err != nil && err != grpc.ErrServerStopped {
		logger.Fatalf("fail to serve, error: %v", err)
	}
	logger.Info("Got SIGTERM signal")
}

func createReservationManager(client *k8s.KubeClient, log *logrus.Logger) (mgr ctrl.Manager, err error) {
	// create scheme
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	// register ACR CRD
	if err = acrcrd.AddToSchemeACR(scheme); err != nil {
		return nil, err
	}

	mgr, err = ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:    scheme,
		Namespace: *namespace,
	})

	if err != nil {
		return nil, err
	}

	// controller
	reservationController := reservation.NewController(client, log)
	if err = reservationController.SetupWithManager(mgr); err != nil {
		return nil, err
	}

	return mgr, nil
}
