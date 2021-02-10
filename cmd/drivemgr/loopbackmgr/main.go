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
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	dmsetup "github.com/dell/csi-baremetal/cmd/drivemgr"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/rpc"
	csibmnodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/csibmnode/common"
	"github.com/dell/csi-baremetal/pkg/drivemgr/loopbackmgr"
)

var (
	endpoint = flag.String("drivemgrendpoint", base.DefaultDriveMgrEndpoint, "DriveManager Endpoint")
	logPath  = flag.String("logpath", "", "log path for DriveManager")
	logLevel = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
	useNodeAnnotation = flag.Bool("usenodeannotation", false,
		"Whether svc should read id from node annotation")
)

func main() {
	flag.Parse()
	nodeName := os.Getenv("KUBE_NODE_NAME")

	featureConf := featureconfig.NewFeatureConfig()
	featureConf.Update(featureconfig.FeatureNodeIDFromAnnotation, *useNodeAnnotation)

	logger, err := base.InitLogger(*logPath, *logLevel)
	if err != nil {
		logger.Warnf("Can't set logger's output to %s. Using stdout instead.\n", *logPath)
	}

	k8SClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("fail to create kubernetes client, error: %v", err)
	}

	nodeID, err := getNodeID(k8SClient, nodeName, featureConf)
	if err != nil {
		logger.Fatalf("fail to get nodeID, error: %v", err)
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

func getNodeID(client k8sClient.Client, nodeName string, featureChecker featureconfig.FeatureChecker) (string, error) {
	if featureChecker.IsEnabled(featureconfig.FeatureNodeIDFromAnnotation) {
		k8sNode := corev1.Node{}
		if err := client.Get(context.Background(), k8sClient.ObjectKey{Name: nodeName}, &k8sNode); err != nil {
			return "", err
		}

		if val, ok := k8sNode.GetAnnotations()[csibmnodeconst.NodeIDAnnotationKey]; ok {
			return val, nil
		}
		return "", fmt.Errorf("annotation %s hadn't been set for node %s", csibmnodeconst.NodeIDAnnotationKey, nodeName)
	}
	// use hostname of pod if uniq nodeID usage isn't enabled.
	hostname := os.Getenv("HOSTNAME")
	return hostname, nil
}
