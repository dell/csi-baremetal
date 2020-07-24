package scheduler

import (
	"fmt"
	"testing"

	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	k8sV1 "k8s.io/api/core/v1"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

var (
	testLogger = logrus.New()

	testSizeStr     = "10G"
	testStorageType = "HDD"
	testCSIVolume   = &k8sV1.CSIVolumeSource{
		Driver:           fmt.Sprintf("%s-hdd", pluginNameMask),
		VolumeAttributes: map[string]string{base.SizeKey: testSizeStr, base.StorageTypeKey: testStorageType},
	}
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
	e := setup()
	curr, err := e.constructVolumeFromCSISource(testCSIVolume)
	assert.Nil(t, err)
	expectedSize, err := util.StrToBytes(testSizeStr)
	assert.Nil(t, err)
	expectedVolume := &genV1.Volume{StorageClass: testStorageType, Size: expectedSize, Ephemeral: true}
	assert.Equal(t, expectedVolume, curr)

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
