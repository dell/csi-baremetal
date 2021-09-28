/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8s

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"
	crApiutil "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// GetK8SCache returns k8s cache
func GetK8SCache() (cache.Cache, error) {
	config := ctrl.GetConfigOrDie()
	scheme, err := PrepareScheme()
	if err != nil {
		return nil, err
	}
	// Create the mapper provider
	mapper, err := crApiutil.NewDynamicRESTMapper(config)
	if err != nil {
		return nil, err
	}

	return cache.New(config, cache.Options{
		Scheme: scheme,
		Mapper: mapper,
	})
}

// KubeCache is a wrapper for controller-runtime cache
type KubeCache struct {
	k8sCl.Reader
	log *logrus.Entry
}

// ReadCR CRReader implementation
func (k KubeCache) ReadCR(ctx context.Context, name string, namespace string, obj runtime.Object) error {
	return k.Get(ctx, k8sCl.ObjectKey{Name: name, Namespace: namespace}, obj)
}

// ReadList CRReader implementation
func (k KubeCache) ReadList(ctx context.Context, obj runtime.Object) error {
	return k.List(ctx, obj)
}

// NewKubeCache is the constructor for KubeCache struct
// Receives basic reader from controller-runtime, logrus logger
// Returns an instance of KubeCache struct
func NewKubeCache(reader k8sCl.Reader, logger *logrus.Logger) *KubeCache {
	return &KubeCache{
		Reader: reader,
		log:    logger.WithField("component", "KubeClient"),
	}
}

// InitKubeCache creates and starts KubeCache,
// if objects passed the function will block until cache synced for these objects
func InitKubeCache(logger *logrus.Logger, stopCH <-chan struct{}, objects ...runtime.Object) (*KubeCache, error) {
	k8sCache, err := GetK8SCache()
	if err != nil {
		logger.Errorf("fail to create cache for kubernetes resources, error: %v", err)
		return nil, err
	}
	for _, obj := range objects {
		// TODO get rid of TODO context https://github.com/dell/csi-baremetal/issues/556
		_, err := k8sCache.GetInformer(context.TODO(), obj)
		if err != nil {
			logger.Errorf("fail to get cache informer for CR, error: %v", err)
			return nil, err
		}
	}
	// start cache
	go func() {
		// cache implementation we use newer returns err
		_ = k8sCache.Start(stopCH)
	}()

	k8sCache.WaitForCacheSync(stopCH)

	return NewKubeCache(k8sCache, logger), nil
}
