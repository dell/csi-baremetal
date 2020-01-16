package main

import (
	"flag"
	"testing"

	_ "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/test/e2e/scenarios"
	ginkgo "github.com/onsi/ginkgo"
	gomega "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
)

func Test(t *testing.T) {
	flag.Parse()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "CSI Suite")
}

func main() {
	framework.HandleFlags()
	framework.AfterReadingAllFlags(&framework.TestContext)
	Test(&testing.T{})
}
