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

	"github.com/dell/csi-baremetal/test/e2e/common"
	_ "github.com/dell/csi-baremetal/test/e2e/scenarios"
)

// Use env to skip this test during go test ./...
func skipIfNotCI(t *testing.T) {
	if os.Getenv("CI") != "true" {
		t.Skip("Skipping testing in not CI environment")
	}
}

func registerBMDriverFlags(flags *flag.FlagSet) {
	flags.BoolVar(&common.BMDriverTestContext.BMDeploySchedulerExtender, "bm-deploy-scheduler-extender",
		true, "Deploy extender for scheduler")
	flags.BoolVar(&common.BMDriverTestContext.BMDeploySchedulerPatcher, "bm-deploy-scheduler-patcher",
		true, "Deploy patcher for scheduler config")
	flags.BoolVar(&common.BMDriverTestContext.BMWaitSchedulerRestart, "bm-wait-scheduler-restart",
		true, "Wait for scheduler restart")
	flags.BoolVar(&common.BMDriverTestContext.BMDeployCSIBMNodeOperator, "bm-deploy-csi-bm-node-operator",
		true, "Deploy controller for CSIBMNode CRs")
}

func init() {
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
	registerBMDriverFlags(flag.CommandLine)
}

func Test(t *testing.T) {
	skipIfNotCI(t)
	flag.Parse()
	framework.AfterReadingAllFlags(&framework.TestContext)
	gomega.RegisterFailHandler(ginkgo.Fail)
	junitReporter := reporters.NewJUnitReporter("report.xml")
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "CSI Suite", []ginkgo.Reporter{junitReporter})
}

func main() {
	Test(&testing.T{})
}
