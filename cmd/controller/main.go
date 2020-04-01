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
	version           = "0.0.3"
	timeoutBeforeInit = 30
	attemptsToInit    = 5
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

	logger.Infof("Wait %d seconds before start controller initialization in %d attempts",
		timeoutBeforeInit, attemptsToInit)
	ticker := time.NewTicker(timeoutBeforeInit * time.Second)
	for i := 1; i <= attemptsToInit; i++ {
		<-ticker.C
		if err = controllerService.InitController(); err != nil {
			if i == attemptsToInit {
				logger.Fatal(err)
			}
			logger.Errorf("Failed to Init Controller: %v, attempt %d out of %d", err, i, attemptsToInit)
		} else {
			logger.Info("Controller was initialized.")
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
