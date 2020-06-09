// Package for main function of Node
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	health "google.golang.org/grpc/health/grpc_health_v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	// +kubebuilder:scaffold:imports

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/rpc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/node"
)

var (
	namespace        = flag.String("namespace", "", "Namespace in which Node Service service run")
	driveMgrEndpoint = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "Hardware Manager endpoint")
	volumeMgrIP      = flag.String("volumemgrip", base.DefaultVMMgrIP, "Node Volume Manager endpoint")
	csiEndpoint      = flag.String("csiendpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	nodeID           = flag.String("nodeid", "", "node identification by k8s")
	logPath          = flag.String("logpath", "", "Log path for Node Volume Manager service")
	verboseLogs      = flag.Bool("verbose", false, "Debug mode in logs")
)

func main() {
	flag.Parse()

	logger, err := base.InitLogger(*logPath, *verboseLogs)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Starting Node Service")

	// gRPC client for communication with DriveMgr via TCP socket
	gRPCClient, err := rpc.NewClient(nil, *driveMgrEndpoint, logger)
	if err != nil {
		logger.Fatalf("fail to create grpc client for endpoint %s, error: %v", *driveMgrEndpoint, err)
	}
	clientToDriveMgr := api.NewDriveServiceClient(gRPCClient.GRPCClient)

	// gRPC server that will serve requests (node CSI) from k8s via unix socket
	csiUDSServer := rpc.NewServerRunner(nil, *csiEndpoint, logger)

	k8SClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}

	nodeUID, err := getNodeUID(k8SClient, *nodeID)
	if err != nil {
		logger.Fatalf("fail to get uid of k8s Node object: %v", err)
	}

	k8sClientForVolume := k8s.NewKubeClient(k8SClient, logger, *namespace)
	k8sClientForLVG := k8s.NewKubeClient(k8SClient, logger, *namespace)
	csiNodeService := node.NewCSINodeService(clientToDriveMgr, nodeUID, logger, k8sClientForVolume)

	mgr := prepareCRDControllerManagers(
		csiNodeService,
		lvm.NewLVGController(k8sClientForLVG, nodeUID, logger),
		logger)

	// register CSI calls handler
	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiNodeService)

	go util.SetupSignalHandler(csiUDSServer)

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
	if err := csiUDSServer.RunServer(); err != nil && err != grpc.ErrServerStopped {
		logger.Fatalf("fail to serve: %v", err)
	}

	logger.Info("Got SIGTERM signal")
}

// Discovering performs Discover method of the Node each 30 seconds
// TODO: implement logic for discover  AK8S-64
func Discovering(c *node.CSINodeService, logger *logrus.Logger) {
	var err error
	discoveringWaitTime := 10 * time.Second
	for {
		time.Sleep(discoveringWaitTime)
		if err = c.Discover(); err != nil {
			logger.Infof("Discover finished with error: %v", err)
		} else {
			logger.Info("Discover finished successful")
			//Increase wait time, because we don't need to call API often after node initialization
			discoveringWaitTime = 30 * time.Second
		}
	}
}

// StartNodeHealthServer starts gRPC server to handle Health checking requests
func StartNodeHealthServer(c health.HealthServer, logger *logrus.Logger) error {
	logger.Info("Starting Node Health server ...")
	// gRPC server that will serve requests for Node Health checking
	nodeHealthEndpoint := fmt.Sprintf("tcp://%s:%d", *volumeMgrIP, base.DefaultVolumeManagerPort)
	nodeHealthServer := rpc.NewServerRunner(nil, nodeHealthEndpoint, logger)
	// register Health checks
	logger.Info("Registering Node service health check")
	health.RegisterHealthServer(nodeHealthServer.GRPCServer, c)
	return nodeHealthServer.RunServer()
}

// prepareCRDControllerManagers prepares CRD ControllerManagers to work with CSI custom resources
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

func getNodeUID(client k8sClient.Client, nodeName string) (string, error) {
	k8sNode := corev1.Node{}
	if err := client.Get(context.Background(), k8sClient.ObjectKey{Name: nodeName}, &k8sNode); err != nil {
		return "", err
	}
	return string(k8sNode.UID), nil
}
