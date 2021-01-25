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

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/crcontrollers/csibmnode"
)

var (
	nodeSelector = flag.String("nodeselector", "", "controller will be work only with node with provided nodeSelector")
	namespace    = flag.String("namespace", "", "Namespace in which controller service run")
	logLevel     = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
	logFormat = flag.String("logformat", base.LogFormatText,
		fmt.Sprintf("Log level, supported value is %s. Json format is used by default", base.LogFormatText))
)

func main() {
	flag.Parse()

	// TODO: refactor this after https://github.com/dell/csi-baremetal/issues/83 will be closed
	err := os.Setenv("LOG_FORMAT", *logFormat)
	if err != nil {
		fmt.Printf("Unable to set LOG_FORMAT env: %v\n", err)
	}

	logger, _ := base.InitLogger("", *logLevel)
	if logger == nil {
		fmt.Println("Unable to initialize logger")
		os.Exit(1)
	}

	mgr, err := prepareK8sRuntimeManager()
	if err != nil {
		logger.Fatalf("fail to init manager: %v\n", err)
	}

	kubeClient := k8s.NewKubeClient(mgr.GetClient(), logger, *namespace)

	nodeCtrl, err := csibmnode.NewController(*nodeSelector, kubeClient, logger)
	if err != nil {
		logger.Fatal(err)
	}

	// bind K8s Controller Manager as a controller for CSIBMNode CR
	if err = nodeCtrl.SetupWithManager(mgr); err != nil {
		logger.Fatal(err)
	}

	logger.Info("Starting CSIBMNode Controller Manager ...")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Fatalf("CRD Controller Manager failed with error: %v", err)
	}
}

func prepareK8sRuntimeManager() (ctrl.Manager, error) {
	scheme, err := k8s.PrepareScheme()
	if err != nil {
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
