package cache

import (
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/testutils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWrapCache(t *testing.T) {
	volumeID := "volume-0"
	namespace := "test"
	t.Run("Namespace isn't in cache", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(namespace, logrus.New())
		assert.Nil(t, err)
		cache := NewCacheWrapper(kubeClient)
		testutils.CreatePVC(kubeClient, volumeID, namespace)
		volNamespace, err := cache.GetVolumeNamespace(volumeID)
		assert.Equal(t, namespace, volNamespace)
		cacheNamespace, err := cache.Get(volumeID)
		assert.Nil(t, err)
		assert.Equal(t, namespace, cacheNamespace)
	})
	t.Run("Namespace was in cache", func(t *testing.T) {
		kubeClient, err := k8s.GetFakeKubeClient(namespace, logrus.New())
		assert.Nil(t, err)
		cache := NewCacheWrapper(kubeClient)
		cache.Set(volumeID, namespace)
		volNamespace, err := cache.GetVolumeNamespace(volumeID)
		assert.Equal(t, namespace, volNamespace)
	})
}
