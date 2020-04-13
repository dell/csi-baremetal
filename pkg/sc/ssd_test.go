package sc

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

var loggerSSDSC = logrus.New()

func TestGetSSDSCInstance(t *testing.T) {
	ssdSCInstanceTest := GetSSDSCInstance(loggerSSDSC)
	assert.Equal(t, ssdSCInstanceTest, ssdSCInstanceTest)

	ssdSCInstanceTest2 := GetSSDSCInstance(loggerSSDSC)
	assert.Equal(t, ssdSCInstanceTest2, ssdSCInstanceTest)
}

func TestSetSSDSCExecutor(t *testing.T) {
	ssdSCInstanceTest := GetSSDSCInstance(loggerSSDSC)
	executorNew := &base.Executor{}
	ssdSCInstanceTest.SetSDDSCExecutor(executorNew)
	assert.Equal(t, executorNew, ssdSCInstanceTest.executor)
}
