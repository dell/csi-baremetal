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

	"github.com/dell/csi-baremetal/pkg/base"
)

var (
	namespace  = flag.String("namespace", "", "Namespace in which controller service run")
	logLevel = flag.String("loglevel", base.InfoLevel,
		fmt.Sprintf("Log level, support values are %s, %s, %s", base.InfoLevel, base.DebugLevel, base.TraceLevel))
	logFormat = flag.String("logformat", base.LogFormatText,
		fmt.Sprintf("Log level, supported value is %s. Json format is used by default", base.LogFormatText))
)

func main() {
	flag.Parse()

	logger, _ := base.InitLogger("", *logLevel)
	if logger == nil {
		fmt.Println("Unable to initialize logger")
		os.Exit(1)
	}

	logger.Info("Starting CSI Bare-metal operator controller ...")

}
