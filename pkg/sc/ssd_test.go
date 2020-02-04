package sc

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSSDSCInstance(t *testing.T) {
	ssdSCInstanceTest := GetSSDSCInstance()
	assert.Equal(t, ssdSCInstanceTest, ssdSCInstanceTest)

	ssdSCInstanceTest2 := GetSSDSCInstance()
	assert.Equal(t, ssdSCInstanceTest2, ssdSCInstanceTest)
}

func TestSetSSDSCExecutor(t *testing.T) {
	ssdSCInstanceTest := GetSSDSCInstance()
	executorNew := &base.Executor{}
	ssdSCInstanceTest.SetSDDSCExecutor(*executorNew)
	assert.Equal(t, executorNew, ssdSCInstanceTest.executor)
}
