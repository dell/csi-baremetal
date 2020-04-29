// Package for main function of Controller
package main

import (
	"flag"
	"time"

	"github.com/sirupsen/logrus"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"github.com/container-storage-interface/spec/lib/go/csi"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// +kubebuilder:scaffold:imports
)

var (
	namespace   = flag.String("namespace", "", "Namespace in which controller service run")
	endpoint    = flag.String("endpoint", "", "Endpoint for controller service")
	logPath     = flag.String("logpath", "", "Log path for Controller service")
	verboseLogs = flag.Bool("verbose", false, "Debug mode in logs")
)

const (
	driverName        = "baremetal-csi"
	version           = "0.0.5"
	timeoutBeforeInit = 30
)

func main() {
	flag.Parse()

	var logLevel logrus.Level
	if *verboseLogs {
		logLevel = logrus.DebugLevel
	} else {
		logLevel = logrus.InfoLevel
	}

	logger, err := base.InitLogger(*logPath, logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Starting controller ...")

	csiControllerServer := base.NewServerRunner(nil, *endpoint, logger)

	k8SClient, err := base.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}
	kubeClient := base.NewKubeClient(k8SClient, logger, *namespace)
	controllerService := controller.NewControllerService(kubeClient, logger)

	ticker := time.NewTicker(timeoutBeforeInit * time.Second)
	for i := 1; ; i++ {
		<-ticker.C
		// check whether there is any ready pod with node service or no
		// controller will start  when at least one ready node service will be detected
		if !controllerService.WaitNodeServices() {
			logger.Warnf("There are no ready node services, attempt %d. Wait %d seconds and retry.", i, timeoutBeforeInit)
		} else {
			logger.Info("Ready node service detected")
			break
		}
	}
	ticker.Stop()

	csi.RegisterIdentityServer(csiControllerServer.GRPCServer, controller.NewIdentityServer(driverName, version, true))
	csi.RegisterControllerServer(csiControllerServer.GRPCServer, controllerService)

	logger.Info("Starting CSIControllerService")
	if err := csiControllerServer.RunServer(); err != nil {
		logger.Fatalf("fail to serve, error: %v", err)
	}
}
