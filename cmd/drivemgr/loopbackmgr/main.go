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
	"os"

	"github.com/fsnotify/fsnotify"

	dmsetup "github.com/dell/csi-baremetal/cmd/drivemgr"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/logger"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	annotations "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
	"github.com/dell/csi-baremetal/pkg/drivemgr/loopbackmgr"
)

var (
	endpoint = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "DriveManager Endpoint")
	logPath  = flag.String("logpath", "", "log path for DriveManager")
	logLevel = flag.String("loglevel", logger.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", logger.InfoLevel, logger.DebugLevel, logger.TraceLevel))
	useNodeAnnotation = flag.Bool("usenodeannotation", false,
		"Whether svc should read id from node annotation")
	useExternalAnnotation = flag.Bool("useexternalannotation", false,
		"Whether node should read id from external annotation. It should exist before deployment. Use if \"usenodeannotation\" is True")
	nodeIDAnnotation = flag.String("nodeidannotation", "",
		"Custom node annotation name. Use if \"useexternalannotation\" is True")
)

func main() {
	nodeName := os.Getenv("KUBE_NODE_NAME")

	flag.Parse()

	featureConf := featureconfig.NewFeatureConfig()
	featureConf.Update(featureconfig.FeatureNodeIDFromAnnotation, *useNodeAnnotation)
	featureConf.Update(featureconfig.FeatureExternalAnnotationForNode, *useExternalAnnotation)

	logger, err := logger.InitLogger(*logPath, *logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	k8SClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}

	// we need to obtain node ID first before proceeding with the initialization
	nodeID, err := annotations.ObtainNodeIDWithRetries(k8SClient, featureConf, nodeName, *nodeIDAnnotation,
		logger, annotations.NumberOfRetries, annotations.DelayBetweenRetries)
	if err != nil {
		logger.Fatalf("Unable to obtain node ID: %v", err)
	}

	// Server is insecure for now because credentials are nil
	serverRunner := rpc.NewServerRunner(nil, *endpoint, false, logger)

	e := command.NewExecutor(logger)

	// creates a new file watcher for config
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatalf("Failed to create fs watcher: %v", err)
	}
	//nolint:errcheck
	defer watcher.Close()

	driveMgr := loopbackmgr.NewLoopBackManager(e, nodeID, nodeName, logger)

	go driveMgr.UpdateOnConfigChange(watcher)
	dmsetup.SetupAndRunDriveMgr(driveMgr, serverRunner, driveMgr.CleanupLoopDevices, logger)
}
