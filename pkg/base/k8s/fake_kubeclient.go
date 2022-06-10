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
	"errors"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/logger/objects"
)

type contextKey string

const (
	updateFailCtxKey   = contextKey("updateFail")
	updateFailCtxValue = "yes"
	listFailCtxKey     = contextKey("listFail")
	listFailCtxValue   = "yes"
	getFailCtxKey      = contextKey("getFail")
	getFailCtxValue    = "yes"
	deleteFailCtxKey   = contextKey("deleteFail")
	deleteFailCtxValue = "yes"
)

var (
	// UpdateFailCtx raises an error on fakeClient.Update
	UpdateFailCtx = context.WithValue(context.Background(), updateFailCtxKey, updateFailCtxValue)
	// ListFailCtx raises an error on fakeClient.List
	ListFailCtx = context.WithValue(context.Background(), listFailCtxKey, listFailCtxValue)
	// GetFailCtx raises an error on fakeClient.Get
	GetFailCtx = context.WithValue(context.Background(), getFailCtxKey, getFailCtxValue)
	// DeleteFailCtx raises an error on fakeClient.Get
	DeleteFailCtx = context.WithValue(context.Background(), deleteFailCtxKey, deleteFailCtxValue)
)

// NewFakeClientWrapper return new instance of FakeClientWrapper
func NewFakeClientWrapper(client k8sCl.Client, scheme *runtime.Scheme) *FakeClientWrapper {
	return &FakeClientWrapper{client: client, scheme: scheme}
}

// FakeClientWrapper wrapper for k8s fake client
// required because behaviour of real kube client and fake are different
// real client will return resources with Cluster scope even if namespaced request was sent
// fake client doesn't know about scope for resources,
// so it will search for resources with Cluster scope in namespace, which was submitted in request
type FakeClientWrapper struct {
	client k8sCl.Client
	scheme *runtime.Scheme
}

// Get is a wrapper around Get method
func (fkw *FakeClientWrapper) Get(ctx context.Context, key k8sCl.ObjectKey, obj k8sCl.Object) error {
	if ctx.Value(getFailCtxKey) == getFailCtxValue {
		return errors.New("raise error in get")
	}
	if fkw.shouldPatchNS(obj) {
		key = fkw.removeNSFromObjKey(key)
	}
	return fkw.client.Get(ctx, key, obj)
}

// List is a wrapper around List method
func (fkw *FakeClientWrapper) List(ctx context.Context, list k8sCl.ObjectList, opts ...k8sCl.ListOption) error {
	if ctx.Value(listFailCtxKey) == listFailCtxValue {
		return errors.New("raise list error")
	}
	if fkw.shouldPatchNS(list) {
		opts = fkw.removeNSFromListOptions(opts)
	}
	return fkw.client.List(ctx, list, opts...)
}

// Create is a wrapper around Create method
func (fkw *FakeClientWrapper) Create(ctx context.Context, obj k8sCl.Object, opts ...k8sCl.CreateOption) error {
	return fkw.client.Create(ctx, obj, opts...)
}

// Delete is a wrapper around Delete method
func (fkw *FakeClientWrapper) Delete(ctx context.Context, obj k8sCl.Object, opts ...k8sCl.DeleteOption) error {
	if ctx.Value(deleteFailCtxKey) == updateFailCtxValue {
		return errors.New("raise delete error")
	}
	return fkw.client.Delete(ctx, obj, opts...)
}

// Update is a wrapper around Update method
func (fkw *FakeClientWrapper) Update(ctx context.Context, obj k8sCl.Object, opts ...k8sCl.UpdateOption) error {
	if ctx.Value(updateFailCtxKey) == updateFailCtxValue {
		return errors.New("raise update error")
	}
	return fkw.client.Update(ctx, obj, opts...)
}

// Patch is a wrapper around Patch method
func (fkw *FakeClientWrapper) Patch(ctx context.Context, obj k8sCl.Object,
	patch k8sCl.Patch, opts ...k8sCl.PatchOption) error {
	return fkw.client.Patch(ctx, obj, patch, opts...)
}

// DeleteAllOf is a wrapper around DeleteAllOf method
func (fkw *FakeClientWrapper) DeleteAllOf(ctx context.Context, obj k8sCl.Object, opts ...k8sCl.DeleteAllOfOption) error {
	return fkw.client.DeleteAllOf(ctx, obj, opts...)
}

// Status is a wrapper around Status method
func (fkw *FakeClientWrapper) Status() k8sCl.StatusWriter {
	return fkw.client.Status()
}

// Scheme is a wrapper around Scheme method
func (fkw *FakeClientWrapper) Scheme() *runtime.Scheme {
	return fkw.scheme
}

// RESTMapper is a wrapper around RESTMapper
// should return the rest this client is using, but not need in fake implementation
func (fkw *FakeClientWrapper) RESTMapper() meta.RESTMapper {
	return nil
}

func (fkw *FakeClientWrapper) shouldPatchNS(obj runtime.Object) bool {
	gvk, err := apiutil.GVKForObject(obj, fkw.scheme)
	if err != nil {
		return false
	}
	// NS patch shouldn't for namespaced resources
	_, isVolume := obj.(*volumecrd.Volume)
	_, isPVC := obj.(*corev1.PersistentVolume)
	if isVolume || isPVC {
		return false
	}
	return gvk.Group == apiV1.CSICRsGroupVersion
}

func (fkw *FakeClientWrapper) removeNSFromListOptions(opts []k8sCl.ListOption) []k8sCl.ListOption {
	result := make([]k8sCl.ListOption, 0)
	for _, opt := range opts {
		if _, ok := opt.(k8sCl.InNamespace); ok {
			continue
		}
		result = append(result, opt)
	}
	return result
}

func (fkw *FakeClientWrapper) removeNSFromObjKey(key k8sCl.ObjectKey) k8sCl.ObjectKey {
	key.Namespace = ""
	return key
}

// GetFakeKubeClient returns fake KubeClient  for test purposes
// Receives namespace to work
// Returns instance of mocked KubeClient or error if something went wrong
// TODO: test code shouldn't be in base package - https://github.com/dell/csi-baremetal/issues/81
func GetFakeKubeClient(testNs string, logger *logrus.Logger) (*KubeClient, error) {
	scheme, err := PrepareScheme()
	if err != nil {
		return nil, err
	}
	fakeClientWrapper := NewFakeClientWrapper(fake.NewClientBuilder().WithScheme(scheme).Build(), scheme)
	return NewKubeClient(fakeClientWrapper, logger, objects.NewObjectLogger(), testNs), nil
}
