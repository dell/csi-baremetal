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
	"os"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/dell/csi-baremetal-e2e-tests/e2e/common"
	_ "github.com/dell/csi-baremetal-e2e-tests/e2e/scenarios"
)

// Use env to skip this test during go test ./...
func skipIfNotCI(t *testing.T) {
	if os.Getenv("CI") != "true" {
		t.Skip("Skipping testing in not CI environment")
	}
}

func registerCustomFlags(flags *flag.FlagSet) {
	flags.StringVar(&common.BMDriverTestContext.ChartsDir, "chartsDir",
		"/tmp/charts", "Path to folder with helm charts")
	flags.BoolVar(&common.BMDriverTestContext.CompleteUninstall, "completeUninstall",
		true, "Uninstall pvc, volumes, lvgs, csibmnodes")
	flags.BoolVar(&common.BMDriverTestContext.NeedAllTests, "all-tests",
		false, "Execute all existing e2e tests")
	flags.DurationVar(&common.BMDriverTestContext.Timeout, "timeout-short-ci",
		0, "Timeout for test suite. Available only if not all-tests")
}

func init() {
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	registerCustomFlags(flag.CommandLine)
}

func Test(t *testing.T) {
	skipIfNotCI(t)
	flag.Parse()
	framework.AfterReadingAllFlags(&framework.TestContext)
	gomega.RegisterFailHandler(ginkgo.Fail)
	junitReporter := reporters.NewJUnitReporter("reports/report.xml")
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "CSI Suite", []ginkgo.Reporter{junitReporter})
}

func main() {
	Test(&testing.T{})
}
