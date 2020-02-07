package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	endpoint    = flag.String("endpoint", "", "Endpoint for controller service")
	logPath     = flag.String("logpath", "", "Log path for Controller service")
	verboseLogs = flag.Bool("verbose", false, "Debug mode in logs")
)

const (
	driverName = "baremetal-csi"
	version    = "0.0.1"
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
		fmt.Printf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Starting controller ...")

	csiControllerServer := base.NewServerRunner(nil, *endpoint, logger)

	k8sC := prepareCRD()
	controllerService := controller.NewControllerService(k8sC, logger)

	csi.RegisterIdentityServer(csiControllerServer.GRPCServer, controller.NewIdentityServer(driverName, version, true))
	csi.RegisterControllerServer(csiControllerServer.GRPCServer, controllerService)

	logger.Info("Starting CSIControllerService")
	if err := csiControllerServer.RunServer(); err != nil {
		logger.Fatalf("fail to serve, error: %v", err)
		os.Exit(1)
	}
}

func prepareCRD() k8sClient.Client {
	scheme := runtime.NewScheme()
	setupLog := ctrl.Log.WithName("setup")

	_ = clientgoscheme.AddToScheme(scheme)
	//register volume crd
	_ = volumecrd.AddToScheme(scheme)
	//register available capacity crd
	_ = accrd.AddToSchemeAvailableCapacity(scheme)
	ctrl.SetLogger(zap.Logger(true))
	cl, err := k8sClient.New(ctrl.GetConfigOrDie(), k8sClient.Options{
		Scheme: scheme,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	return cl
}
