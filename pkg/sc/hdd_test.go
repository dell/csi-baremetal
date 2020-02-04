package sc

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHDDSCInstance(t *testing.T) {
	hddSCInstanceTest := GetHDDSCInstance()
	assert.Equal(t, hddSCInstanceTest, hddSCInstanceTest)

	hddSCInstanceTest2 := GetHDDSCInstance()
	assert.Equal(t, hddSCInstanceTest2, hddSCInstanceTest)
}

func TestSetHDDSCExecutor(t *testing.T) {
	hddSCInstanceTest := GetHDDSCInstance()
	executorNew := &base.Executor{}
	hddSCInstanceTest.SetHDDSCExecutor(*executorNew)
	assert.Equal(t, executorNew, hddSCInstanceTest.executor)
}
