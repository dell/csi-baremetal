// Package for main function of Node
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/dell/csi-baremetal/pkg/lvm"
	"github.com/dell/csi-baremetal/pkg/node"
)

const (
	componentName = "baremetal-csi-node"
)

var (
	namespace        = flag.String("namespace", "", "Namespace in which Node Service service run")
	driveMgrEndpoint = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "Hardware Manager endpoint")
	healthIP         = flag.String("healthip", base.DefaultHealthIP, "Node health server ip")
	csiEndpoint      = flag.String("csiendpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	nodeID           = flag.String("nodeid", "", "node identification by k8s")
	logPath          = flag.String("logpath", "", "Log path for Node Volume Manager service")
	eventConfigPath  = flag.String("eventConfigPath", "/etc/config/alerts.yaml", "path for the events config file")
	logLevel         = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
)

func main() {
	flag.Parse()

	logger, err := base.InitLogger(*logPath, *logLevel)
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
	eventRecorder, err := prepareEventRecorder(*eventConfigPath, nodeUID, logger)
	if err != nil {
		logger.Fatalf("fail to prepare event recorder: %v", err)
	}

	// Wait till all events are sent/handled
	defer eventRecorder.Wait()

	k8sClientForVolume := k8s.NewKubeClient(k8SClient, logger, *namespace)
	k8sClientForLVG := k8s.NewKubeClient(k8SClient, logger, *namespace)
	csiNodeService := node.NewCSINodeService(clientToDriveMgr, nodeUID, logger, k8sClientForVolume, eventRecorder)

	mgr := prepareCRDControllerManagers(
		csiNodeService,
		lvm.NewLVGController(k8sClientForLVG, nodeUID, logger),
		logger)

	// register CSI calls handler
	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiNodeService)

	go util.SetupSignalHandler(csiUDSServer)

	go func() {
		logger.Info("Starting Node Health server ...")
		nodeHealthEndpoint := fmt.Sprintf("tcp://%s:%d", *healthIP, base.DefaultHealthPort)
		if err := util.SetupAndStartHealthCheckServer(csiNodeService, logger, nodeHealthEndpoint); err != nil {
			logger.Fatalf("Node service failed with error: %v", err)
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
			logger.Errorf("Discover finished with error: %v", err)
		} else {
			logger.Info("Discover finished successful")
			//Increase wait time, because we don't need to call API often after node initialization
			discoveringWaitTime = 30 * time.Second
		}
	}
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

// prepareEventRecorder helper which makes all the work to get EventRecorder
func prepareEventRecorder(configfile, nodeUID string, logger *logrus.Logger) (*events.Recorder, error) {
	// clientset needed to send events
	k8SClientset, err := k8s.GetK8SClientset()
	if err != nil {
		return nil, fmt.Errorf("fail to create kubernetes client, error: %s", err)
	}
	eventInter := k8SClientset.CoreV1().Events(*namespace)

	// get the Scheme
	// in our case we should use Scheme that aware of our CR
	scheme, err := k8s.PrepareScheme()
	if err != nil {
		return nil, fmt.Errorf("fail to prepare kubernetes scheme, error: %s", err)
	}
	// Setup Option
	// It's used for label overriding and logging events

	var opt events.Options

	// Optional will be used when
	alertFile, err := ioutil.ReadFile(configfile)
	if err != nil {
		logger.Infof("fail to open events config file. error: %s. Will proceed without overriding.", err)
	}

	err = yaml.Unmarshal(alertFile, &opt)
	if err != nil {
		return nil, fmt.Errorf("fail to unmarshal config file, error: %s", err)
	}

	opt.Logger = logger.WithField("componentName", "Events")
	//

	eventRecorder, err := events.New(componentName, nodeUID, eventInter, scheme, opt)
	if err != nil {
		return nil, fmt.Errorf("fail to create events recorder, error: %s", err)
	}
	return eventRecorder, nil
}
