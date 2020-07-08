package main

import (
	"flag"
	"os"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"

	_ "github.com/dell/csi-baremetal.git/test/e2e/scenarios"
)

// Use env to skip this test during go test ./...
func skipIfNotCI(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("Skipping testing in not CI environment")
	}
}

func init() {
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
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
