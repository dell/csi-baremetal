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

// Package for main function of CSI Bare-metal operator
package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/logger"
	"github.com/dell/csi-baremetal/pkg/crcontrollers/node"
)

var (
	nodeSelector          = flag.String("nodeselector", "", "controller will be work only with node with provided nodeSelector")
	namespace             = flag.String("namespace", "", "Namespace in which controller service run")
	useExternalAnnotation = flag.Bool("useexternalannotation", false,
		"Whether operator should read id from external annotation. It should exist before deployment. Use if \"usenodeannotation\" is True")
	nodeIDAnnotation = flag.String("nodeidannotation", "",
		"Custom node annotation name. Use if \"useexternalannotation\" is True")
	logLevel = flag.String("loglevel", logger.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", logger.InfoLevel, logger.DebugLevel, logger.TraceLevel))
	logFormat = flag.String("logformat", logger.LogFormatText,
		fmt.Sprintf("Log level, supported value is %s. Json format is used by default", logger.LogFormatText))
)

func main() {
	flag.Parse()

	// TODO: refactor this after https://github.com/dell/csi-baremetal/issues/83 will be closed
	err := os.Setenv("LOG_FORMAT", *logFormat)
	if err != nil {
		fmt.Printf("Unable to set LOG_FORMAT env: %v\n", err)
	}

	logger, _ := logger.InitLogger("", *logLevel)
	if logger == nil {
		fmt.Println("Unable to initialize logger")
		os.Exit(1)
	}

	k8sClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatalf("Unable to create k8s client: %v", err)
	}
	kubeClient := k8s.NewKubeClient(k8sClient, logger, *namespace)

	nodeCtrl, err := node.NewController(*nodeSelector, *useExternalAnnotation, *nodeIDAnnotation, kubeClient, logger)
	if err != nil {
		logger.Fatal(err)
	}

	mgr, err := prepareK8sRuntimeManager()
	if err != nil {
		logger.Fatal(err)
	}

	// bind K8s Controller Manager as a controller for Node CR
	if err = nodeCtrl.SetupWithManager(mgr); err != nil {
		logger.Fatal(err)
	}

	logger.Info("Starting Node Controller Manager ...")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Fatalf("CRD Controller Manager failed with error: %v", err)
	}
}

func prepareK8sRuntimeManager() (ctrl.Manager, error) {
	var (
		scheme = runtime.NewScheme()
		err    error
	)

	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	// register Node CRD
	if err = nodecrd.AddToSchemeCSIBMNode(scheme); err != nil {
		return nil, err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:    scheme,
		Namespace: *namespace,
	})

	if err != nil {
		return nil, err
	}

	return mgr, nil
}
