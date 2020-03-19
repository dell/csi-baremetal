package main

import (
	"flag"
	"os"
	"testing"

	ginkgo "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	gomega "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"

	_ "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/test/e2e/scenarios"
)

// Use env to skip this test during go test ./...
func skipIfNotCI(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("Skipping testing in not CI environment")
	}
}

func init() {
	framework.HandleFlags()
	framework.AfterReadingAllFlags(&framework.TestContext)
}

func Test(t *testing.T) {
	skipIfNotCI(t)
	flag.Parse()
	gomega.RegisterFailHandler(ginkgo.Fail)
	junitReporter := reporters.NewJUnitReporter("report.xml")
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "CSI Suite", []ginkgo.Reporter{junitReporter})
}

func main() {
	Test(&testing.T{})
}
