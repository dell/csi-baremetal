package sc

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

var loggerHDDSC = logrus.New()

func TestGetHDDSCInstance(t *testing.T) {
	hddSCInstanceTest := GetHDDSCInstance(loggerHDDSC)
	assert.Equal(t, hddSCInstanceTest, hddSCInstanceTest)

	hddSCInstanceTest2 := GetHDDSCInstance(loggerHDDSC)
	assert.Equal(t, hddSCInstanceTest2, hddSCInstanceTest)
}

func TestSetHDDSCExecutor(t *testing.T) {
	hddSCInstanceTest := GetHDDSCInstance(loggerHDDSC)
	executorNew := &base.Executor{}
	hddSCInstanceTest.SetHDDSCExecutor(*executorNew)
	assert.Equal(t, executorNew, hddSCInstanceTest.executor)
}
