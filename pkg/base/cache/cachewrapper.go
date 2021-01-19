package cache

import (
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

// WrapCache wrapper for BaseCache interface
type WrapCache struct {
	Interface
	k8sClient *k8s.KubeClient
}

// NewCacheWrapper is a constructor for WrapCache
func NewCacheWrapper(k8sClient *k8s.KubeClient) WrapCache {
	return WrapCache{
		Interface: NewBaseCache(),
		k8sClient: k8sClient,
	}
}

// GetVolumeNamespace returns namespace of volume with given id
// Receives volume id
// Returns namespace as a string and error
func (cw *WrapCache) GetVolumeNamespace(volumeID string) (string, error) {
	namespace, err := cw.Get(volumeID)
	if err != nil {
		if namespace, err = cw.k8sClient.GetVolumeNamespace(volumeID); err != nil {
			return "", err
		}
		if namespace != "" {
			cw.Set(volumeID, namespace)
		}
	}
	return namespace, nil
}
