package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8Client is a mock implementation of client.Client interface from controller-runtime
type K8Client struct {
	client.Reader
	client.Writer
	client.StatusClient
	mock.Mock
}

// Get is mock implementation of Get method from client.Reader interface
func (k *K8Client) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	args := k.Mock.Called(ctx, key, obj)
	return args.Error(0)
}

// List is mock implementation of List method from client.Reader interface
func (k *K8Client) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	args := k.Mock.Called(ctx, list, opts)
	return args.Error(0)
}

// Create is mock implementation of Create method from client.Writer interface
func (k *K8Client) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	args := k.Mock.Called(ctx, obj, opts)
	return args.Error(0)
}

// Delete is mock implementation of Delete method from client.Writer interface
func (k *K8Client) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	args := k.Mock.Called(ctx, obj, opts)
	return args.Error(0)
}

// Update is mock implementation of Update method from client.Writer interface
func (k *K8Client) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	args := k.Mock.Called(ctx, obj, opts)
	return args.Error(0)
}

// Patch is mock implementation of Patch method from client.Writer interface
func (k *K8Client) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := k.Mock.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

// DeleteAllOf is mock implementation of DeleteAllOf method from client.Writer interface
func (k *K8Client) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	args := k.Mock.Called(ctx, obj, opts)
	return args.Error(0)
}
