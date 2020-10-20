package csibmnode

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

var (
	testNS     = "default"
	testLogger = logrus.New()
)

func TestNewCSIBMController(t *testing.T) {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)

	c, err := NewController(k8sClient, testLogger)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	assert.NotNil(t, c.cache)
	assert.NotNil(t, c.cache.bmToK8sNode)
	assert.NotNil(t, c.cache.k8sToBMNode)
}

func setup(t *testing.T) *Controller {
	k8sClient, err := k8s.GetFakeKubeClient(testNS, testLogger)
	assert.Nil(t, err)

	c, err := NewController(k8sClient, testLogger)
	assert.Nil(t, err)
	return c
}
