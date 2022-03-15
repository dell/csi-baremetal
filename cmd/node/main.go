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

// Package for main function of Node
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/container-storage-interface/spec/lib/go/csi"
	api "github.com/dell/csi-baremetal/api/generated/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/logger"
	"github.com/dell/csi-baremetal/pkg/base/logger/objects"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/crcontrollers/drive"
	"github.com/dell/csi-baremetal/pkg/crcontrollers/lvg"
	annotations "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/dell/csi-baremetal/pkg/metrics"
	"github.com/dell/csi-baremetal/pkg/node"
	"github.com/dell/csi-baremetal/pkg/node/wbt"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const (
	componentName = "csi-baremetal-node"
	// on loaded system drive manager might response with the delay
	numberOfRetries  = 20
	delayBeforeRetry = 5
)

var (
	namespace        = flag.String("namespace", "", "Namespace in which Node Service service run")
	driveMgrEndpoint = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "Hardware Manager endpoint")
	healthIP         = flag.String("healthip", base.DefaultHealthIP, "Node health server ip")
	csiEndpoint      = flag.String("csiendpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	nodeName         = flag.String("nodename", "", "node identification by k8s")
	logPath          = flag.String("logpath", "", "Log path for Node Volume Manager service")
	useACRs          = flag.Bool("extender", false,
		"Whether node svc should read AvailableCapacityReservation CR during NodePublish request for ephemeral volumes or not")
	useNodeAnnotation = flag.Bool("usenodeannotation", false,
		"Whether node svc should read id from node annotation and use it as id for all CRs or not")
	useExternalAnnotation = flag.Bool("useexternalannotation", false,
		"Whether node svc should read id from external annotation. It should exist before deployment. Use if \"usenodeannotation\" is True")
	nodeIDAnnotation = flag.String("nodeidannotation", "",
		"Custom node annotation name. Use if \"useexternalannotation\" is True")
	logLevel = flag.String("loglevel", logger.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", logger.InfoLevel, logger.DebugLevel, logger.TraceLevel))
	metricsAddress = flag.String("metrics-address", "", "The TCP network address where the prometheus metrics endpoint will run"+
		"(example: :8080 which corresponds to port 8080 on local host). The default is empty string, which means metrics endpoint is disabled.")
	metricspath = flag.String("metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is /metrics.")
)

func main() {
	flag.Parse()

	featureConf := featureconfig.NewFeatureConfig()
	featureConf.Update(featureconfig.FeatureACReservation, *useACRs)
	featureConf.Update(featureconfig.FeatureNodeIDFromAnnotation, *useNodeAnnotation)
	featureConf.Update(featureconfig.FeatureExternalAnnotationForNode, *useExternalAnnotation)

	var enableMetrics bool
	if *metricspath != "" {
		enableMetrics = true
	}

	logger, err := logger.InitLogger(*logPath, *logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	logger.Info("Starting Node Service")

	stopCH := ctrl.SetupSignalHandler()

	k8SClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}
	// we need to obtain node ID first before proceeding with the initialization
	nodeID, err := obtainNodeIDWithRetries(k8SClient, featureConf, logger)
	if err != nil {
		logger.Fatalf("Unable to obtain node ID: %v", err)
	}

	// gRPC client for communication with DriveMgr via TCP socket
	gRPCClient, err := rpc.NewClient(nil, *driveMgrEndpoint, enableMetrics, logger)
	if err != nil {
		logger.Fatalf("fail to create grpc client for endpoint %s, error: %v", *driveMgrEndpoint, err)
	}
	clientToDriveMgr := api.NewDriveServiceClient(gRPCClient.GRPCClient)

	// gRPC server that will serve requests (node CSI) from k8s via unix socket
	csiUDSServer := rpc.NewServerRunner(nil, *csiEndpoint, enableMetrics, logger)

	kubeCache, err := k8s.InitKubeCache(stopCH, logger,
		&drivecrd.Drive{}, &accrd.AvailableCapacity{}, &volumecrd.Volume{})
	if err != nil {
		logger.Fatalf("fail to start kubeCache, error: %v", err)
	}

	eventRecorder, err := prepareEventRecorder(*nodeName, logger)
	if err != nil {
		logger.Fatalf("fail to prepare event recorder: %v", err)
	}

	wbtWatcher, err := prepareWbtWatcher(k8SClient, eventRecorder, *nodeName, logger)
	if err != nil {
		logger.Fatalf("fail to prepare wbt watcher: %v", err)
	}

	// Wait till all events are sent/handled
	defer eventRecorder.Wait()

	wrappedK8SClient := k8s.NewKubeClient(k8SClient, logger, objects.NewObjectLogger(), *namespace)
	csiNodeService := node.NewCSINodeService(
		clientToDriveMgr, nodeID, *nodeName, logger, wrappedK8SClient, kubeCache, eventRecorder, featureConf)

	mgr := prepareCRDControllerManagers(
		csiNodeService,
		lvg.NewController(wrappedK8SClient, nodeID, logger),
		drive.NewController(wrappedK8SClient, nodeID, clientToDriveMgr, eventRecorder, logger),
		logger)

	// register CSI calls handler
	csi.RegisterNodeServer(csiUDSServer.GRPCServer, csiNodeService)
	csi.RegisterIdentityServer(csiUDSServer.GRPCServer, csiNodeService)

	handler := util.NewSignalHandler(logger)
	go handler.SetupSIGTERMHandler(csiUDSServer)
	if enableMetrics {
		grpc_prometheus.Register(csiUDSServer.GRPCServer)
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
	go func() {
		logger.Info("Starting Node Health server ...")
		if err := util.SetupAndStartHealthCheckServer(
			csiNodeService, logger,
			"tcp://"+net.JoinHostPort(*healthIP, strconv.Itoa(base.DefaultHealthPort))); err != nil {
			logger.Fatalf("Node service failed with error: %v", err)
		}
	}()
	go func() {
		logger.Info("Starting CRD Controller Manager ...")
		if err := mgr.Start(stopCH); err != nil {
			logger.Fatalf("CRD Controller Manager failed with error: %v", err)
		}
	}()
	go Discovering(csiNodeService, logger)

	// wait for readiness
	waitForVolumeManagerReadiness(csiNodeService, logger)

	// start to updating Wbt Config
	wbtWatcher.StartWatch(csiNodeService)

	logger.Info("Starting handle CSI calls ...")
	if err := csiUDSServer.RunServer(); err != nil && err != grpc.ErrServerStopped {
		logger.Fatalf("fail to serve: %v", err)
	}

	logger.Info("Got SIGTERM signal")
}

func obtainNodeIDWithRetries(client k8sClient.Client, featureConf featureconfig.FeatureChecker,
	logger *logrus.Logger) (nodeID string, err error) {
	// try to obtain node ID
	for i := 0; i < numberOfRetries; i++ {
		logger.Info("Obtaining node ID...")
		if nodeID, err = annotations.GetNodeIDByName(client, *nodeName, *nodeIDAnnotation, "", featureConf); err == nil {
			logger.Infof("Node ID is %s", nodeID)
			return nodeID, nil
		}
		logger.Warningf("Unable to get node ID due to %v, sleep and retry...", err)
		time.Sleep(delayBeforeRetry * time.Second)
	}
	// return empty node ID and error
	return "", fmt.Errorf("number of retries %d exceeded", numberOfRetries)
}

func waitForVolumeManagerReadiness(c *node.CSINodeService, logger *logrus.Logger) {
	// check here for volume manager readiness
	// input parameters are ignored by Check() function - pass empty context and health check request
	ctx := context.Background()
	req := &grpc_health_v1.HealthCheckRequest{}
	for i := 0; i < numberOfRetries; i++ {
		logger.Info("Waiting for node service to become ready ...")
		// never returns error
		resp, _ := c.Check(ctx, req)
		// disk info might be outdated (for example, block device names change on node reboot)
		// need to wait for drive info to be updated before starting accepting CSI calls
		if resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
			logger.Info("Node service is ready to handle requests")
			return
		}
		logger.Infof("Not ready yet. Sleep %d seconds and retry ...", delayBeforeRetry)
		time.Sleep(delayBeforeRetry * time.Second)
	}
	// exit if not ready
	logger.Fatalf("Number of retries %d exceeded. Exiting...", numberOfRetries)
}

// Discovering performs Discover method of the Node each 30 seconds
func Discovering(c *node.CSINodeService, logger *logrus.Logger) {
	var err error
	// set initial delay
	discoveringWaitTime := 10 * time.Second
	checker := c.GetLivenessHelper()
	for {
		time.Sleep(discoveringWaitTime)
		if err = c.Discover(); err != nil {
			checker.Fail()
			logger.Errorf("Discover finished with error: %v", err)
		} else {
			checker.OK()
			logger.Tracef("Discover finished successful")
			// Increase wait time, because we don't need to call API often after node initialization
			discoveringWaitTime = 30 * time.Second
		}
	}
}

// prepareCRDControllerManagers prepares CRD ControllerManagers to work with CSI custom resources
func prepareCRDControllerManagers(volumeCtrl *node.CSINodeService, lvgCtrl *lvg.Controller,
	driveCtrl *drive.Controller, logger *logrus.Logger) manager.Manager {
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
	// register LogicalVolumeGroup crd
	if err = lvgcrd.AddToSchemeLVG(scheme); err != nil {
		logrus.Fatal(err)
	}

	// register Drive crd
	if err = drivecrd.AddToSchemeDrive(scheme); err != nil {
		logrus.Fatal(err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		ll.Fatalf("Unable to create new CRD Controller Manager: %v", err)
	}

	// bind CSINodeService's VolumeManager to K8s Controller Manager as a controller for Volume CR
	if err = volumeCtrl.SetupWithManager(mgr); err != nil {
		logger.Fatalf("unable to create controller for volume: %v", err)
	}

	// bind LVMController to K8s Controller Manager as a controller for LogicalVolumeGroup CR
	if err = lvgCtrl.SetupWithManager(mgr); err != nil {
		logger.Fatalf("unable to create controller for LogicalVolumeGroup: %v", err)
	}

	if err = driveCtrl.SetupWithManager(mgr); err != nil {
		logger.Fatalf("unable to create controller for LogicalVolumeGroup: %v", err)
	}

	return mgr
}

// prepareEventRecorder helper which makes all the work to get EventRecorder
func prepareEventRecorder(nodeName string, logger *logrus.Logger) (*events.Recorder, error) {
	// clientset needed to send events
	k8SClientset, err := k8s.GetK8SClientset()
	if err != nil {
		return nil, fmt.Errorf("fail to create kubernetes client, error: %s", err)
	}
	eventInter := k8SClientset.CoreV1().Events("")

	// get the Scheme
	// in our case we should use Scheme that aware of our CR
	scheme, err := k8s.PrepareScheme()
	if err != nil {
		return nil, fmt.Errorf("fail to prepare kubernetes scheme, error: %s", err)
	}

	eventRecorder, err := events.New(componentName, nodeName, eventInter, scheme, logger)
	if err != nil {
		return nil, fmt.Errorf("fail to create events recorder, error: %s", err)
	}
	return eventRecorder, nil
}

func prepareWbtWatcher(client k8sClient.Client, eventsRecorder *events.Recorder, nodeName string, logger *logrus.Logger) (*wbt.ConfWatcher, error) {
	k8sNode := &corev1.Node{}
	err := client.Get(context.Background(), k8sClient.ObjectKey{Name: nodeName}, k8sNode)
	if err != nil {
		return nil, err
	}

	nodeKernel := k8sNode.Status.NodeInfo.KernelVersion
	ll := logger.WithField("componentName", "WbtWatcher")

	return wbt.NewConfWatcher(client, eventsRecorder, ll, nodeKernel), nil
}
