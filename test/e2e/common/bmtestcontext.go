package common

import (
	"k8s.io/kubernetes/test/e2e/framework"
)

type BMDriverTestContextType struct {
	*framework.TestContextType
	BMDeploySchedulerExtender bool
	BMDeploySchedulerPatcher  bool
	BMWaitSchedulerRestart    bool
}

var BMDriverTestContext BMDriverTestContextType

func init() {
	BMDriverTestContext.TestContextType = &framework.TestContext
}
