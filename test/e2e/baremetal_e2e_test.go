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
	if os.Getenv("CI") == "" {
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
