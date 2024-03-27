package mocks

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// MockManager is a mock implementation of manager.Manager interface from controller-runtime
type MockManager struct {
	ctrl.Manager
	mock.Mock
}

// GetConfig is mock implementation of GetConfig method from cluster.Cluster interface
func (m *MockManager) GetConfig() *rest.Config {
	args := m.Mock.Called()
	return args.Get(0).(*rest.Config)
}

// GetScheme is mock implementation of GetScheme method from cluster.Cluster interface
func (m *MockManager) GetScheme() *runtime.Scheme {
	args := m.Mock.Called()
	return args.Get(0).(*runtime.Scheme)
}

// GetClient is mock implementation of GetClient method from cluster.Cluster interface
func (m *MockManager) GetClient() client.Client {
	args := m.Mock.Called()
	return args.Get(0).(client.Client)
}

// GetFieldIndexer is mock implementation of GetFieldIndexer method from cluster.Cluster interface
func (m *MockManager) GetFieldIndexer() client.FieldIndexer {
	args := m.Mock.Called()
	return args.Get(0).(client.FieldIndexer)
}

// GetCache is mock implementation of GetCache method from cluster.Cluster interface
func (m *MockManager) GetCache() cache.Cache {
	args := m.Mock.Called()
	return args.Get(0).(cache.Cache)
}

// GetRESTMapper is mock implementation of GetRESTMapper method from cluster.Cluster interface
func (m *MockManager) GetRESTMapper() meta.RESTMapper {
	args := m.Mock.Called()
	return args.Get(0).(meta.RESTMapper)
}

// GetControllerOptions is mock implementation of GetControllerOptions method from manager.Manager interface
func (m *MockManager) GetControllerOptions() config.Controller {
	args := m.Mock.Called()
	return args.Get(0).(config.Controller)
}

// GetLogger is mock implementation of GetLogger method from manager.Manager interface
func (m *MockManager) GetLogger() logr.Logger {
	args := m.Mock.Called()
	return args.Get(0).(logr.Logger)
}

// Add is mock implementation of Add method from manager.Manager interface
func (m *MockManager) Add(manager.Runnable) error {
	args := m.Mock.Called()
	return args.Error(0)
}

// MockCache is a mock implementation of cache.Cache interface from controller-runtime
type MockCache struct {
	client.Reader
	cache.Informers
	mock.Mock
}

// Get is mock implementation of Get method from cache.Cache interface
func (c *MockCache) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	args := c.Mock.Called(ctx, key, obj, opts)
	return args.Error(0)
}

// List is mock implementation of List method from cache.Cache interface
func (c *MockCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := c.Mock.Called(ctx, list, opts)
	return args.Error(0)
}

// GetInformer is mock implementation of GetInformer method from cache.Cache interface
func (c *MockCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	args := c.Mock.Called(ctx, obj, opts)
	return args.Get(0).(cache.Informer), args.Error(1)
}
