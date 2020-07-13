// Package for main function of Controller
package main

import (
	"flag"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// +kubebuilder:scaffold:imports

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/controller"
)

var (
	namespace  = flag.String("namespace", "", "Namespace in which controller service run")
	healthIP   = flag.String("healthip", base.DefaultHealthIP, "IP for health service")
	healthPort = flag.Int("healthport", base.DefaultHealthPort, "Port for health service")
	endpoint   = flag.String("endpoint", "", "Endpoint for controller service")
	logPath    = flag.String("logpath", "", "Log path for Controller service")
	logLevel   = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
)

func main() {
	flag.Parse()

	logger, err := base.InitLogger(*logPath, *logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Starting controller ...")

	csiControllerServer := rpc.NewServerRunner(nil, *endpoint, logger)

	k8SClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}
	kubeClient := k8s.NewKubeClient(k8SClient, logger, *namespace)
	controllerService := controller.NewControllerService(kubeClient, logger)
	go util.SetupSignalHandler(csiControllerServer)

	csi.RegisterIdentityServer(csiControllerServer.GRPCServer, controllerService)
	csi.RegisterControllerServer(csiControllerServer.GRPCServer, controllerService)
	go func() {
		logger.Info("Starting Controller Health server ...")
		controllerHealthEndpoint := fmt.Sprintf("tcp://%s:%d", *healthIP, *healthPort)
		if err := util.SetupAndStartHealthCheckServer(controllerService, logger, controllerHealthEndpoint); err != nil {
			logger.Fatalf("Controller service failed with error: %v", err)
		}
	}()
	logger.Info("Starting CSIControllerService")
	if err := csiControllerServer.RunServer(); err != nil && err != grpc.ErrServerStopped {
		logger.Fatalf("fail to serve, error: %v", err)
	}
	logger.Info("Got SIGTERM signal")
}
