package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	// +kubebuilder:scaffold:imports

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/controller"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"
)

var (
	namespace     = flag.String("namespace", "", "Namespace in which Node Service service run")
	hwMgrEndpoint = flag.String("hwmgrendpoint", base.DefaultHWMgrEndpoint, "Hardware Manager endpoint")
	volumeMgrIP   = flag.String("volumemgrip", base.DefaultVMMgrIP, "Node Volume Manager endpoint")
	csiEndpoint   = flag.String("csiendpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	nodeID        = flag.String("nodeid", "", "node identification by k8s")
	logPath       = flag.String("logpath", "", "Log path for Node Volume Manager service")
	verboseLogs   = flag.Bool("verbose", false, "Debug mode in logs")
)

func main() {
	flag.Parse()

	logger := setupLogger()
	logger.Info("Starting Node Service")

	// gRPC client for communication with HWMgr via TCP socket
	gRPCClient, err := base.NewClient(nil, *hwMgrEndpoint, logger)
	if err != nil {
		logger.Fatalf("fail to create grpc client for endpoint %s, error: %v", *hwMgrEndpoint, err)
	}
	clientToHwMgr := api.NewHWServiceClient(gRPCClient.GRPCClient)

	// gRPC server that will serve requests (node CSI) from k8s via unix socket
	csiUDSServer := base.NewServerRunner(nil, *csiEndpoint, logger)

	k8SClient, err := base.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}
	k8sClientForVolume := base.NewKubeClient(k8SClient, logger, *namespace)
	k8sClientForLVG := base.NewKubeClient(k8SClient, logger, *namespace)
	csiNodeService := node.NewCSINodeService(clientToHwMgr, *nodeID, logger, k8sClientForVolume)
	csiIdentityService := controller.NewIdentityServer("baremetal-csi", "0.0.3", true)

	mgr := prepareCRDControllerManagers(
		csiNodeService,
		lvm.NewLVGController(k8sClientForLVG, *nodeID, logger),
		logger)

	// register CSI calls handler
	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiIdentityService)

	go func() {
		if err := StartNodeHealthServer(csiNodeService, logger); err != nil {
			logger.Infof("VolumeManager server failed with error: %v", err)
		}
	}()
	go func() {
		logger.Info("Starting CRD Controller Manager ...")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			logger.Fatalf("CRD Controller Manager failed with error: %v", err)
		}
	}()
	go Discovering(csiNodeService, logger)

	logger.Info("Starting handle CSI calls ...")
	if err := csiUDSServer.RunServer(); err != nil {
		logger.Fatalf("fail to serve: %v", err)
	}
}

// TODO: implement logic for discover  AK8S-64
func Discovering(c *node.CSINodeService, logger *logrus.Logger) {
	var err error
	for range time.Tick(30 * time.Second) {
		if err = c.Discover(); err != nil {
			logger.Infof("Discover finished with error: %v", err)
		} else {
			logger.Info("Discover finished successful")
		}
	}
}

// StartNodeHealthServer starts gRPC server to handle Health checking requests
func StartNodeHealthServer(c health.HealthServer, logger *logrus.Logger) error {
	logger.Info("Starting Node Health server ...")
	// gRPC server that will serve requests for Node Health checking
	nodeHealthEndpoint := fmt.Sprintf("tcp://%s:%d", *volumeMgrIP, base.DefaultVolumeManagerPort)
	nodeHealthServer := base.NewServerRunner(nil, nodeHealthEndpoint, logger)
	// register Health checks
	logger.Info("Registering Node service health check")
	health.RegisterHealthServer(nodeHealthServer.GRPCServer, c)
	return nodeHealthServer.RunServer()
}

func prepareCRDControllerManagers(volumeCtrl *node.CSINodeService, lvgCtrl *lvm.LVGController,
	logger *logrus.Logger) manager.Manager {
	var (
		ll     = logger.WithField("method", "prepareCRDControllerManagers")
		scheme = runtime.NewScheme()
		err    error
	)

	if err = clientgoscheme.AddToScheme(scheme); err != nil {
		logger.Fatal(err)
	}
	// register volume crd
	if err = volumecrd.AddToScheme(scheme); err != nil {
		logger.Fatal(err)
	}
	// register LVG crd
	if err = lvgcrd.AddToSchemeLVG(scheme); err != nil {
		logrus.Fatal(err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:    scheme,
		Namespace: *namespace,
	})
	if err != nil {
		ll.Fatalf("Unable to create new CRD Controller Manager: %v", err)
	}

	// bind CSINodeService's VolumeManager to K8s Controller Manager as a controller for Volume CR
	if err = volumeCtrl.SetupWithManager(mgr); err != nil {
		logger.Fatalf("unable to create controller for volume: %v", err)
	}

	// bind LVMController to K8s Controller Manager as a controller for LVG CR
	if err = lvgCtrl.SetupWithManager(mgr); err != nil {
		logger.Fatalf("unable to create controller for LVG: %v", err)
	}

	return mgr
}

func setupLogger() *logrus.Logger {
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
	return logger
}
