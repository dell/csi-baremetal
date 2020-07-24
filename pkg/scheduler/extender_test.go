package scheduler

import (
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

var (
	testLogger = logrus.New()
)

func TestNewExtender(t *testing.T) {
	e := NewExtender(logrus.New())
	assert.NotNil(t, e)
	assert.NotNil(t, e.k8sClient)
	assert.NotNil(t, e.logger)
	assert.Equal(t, namespace, e.k8sClient.Namespace)
}

func TestExtender_gatherVolumesByProvisioner_Success(t *testing.T) {

}

func TestExtender_gatherVolumesByProvisioner_Fail(t *testing.T) {

}

func TestExtender_constructVolumeFromCSISource_Success(t *testing.T) {

}

func setup() *Extender {
	k, err := k8s.GetFakeKubeClient(namespace, testLogger)
	if err != nil {
		panic(err)
	}

	e := NewExtender(testLogger)
	e.k8sClient = k8s.NewKubeClient(k, testLogger, namespace)

	return e
}
