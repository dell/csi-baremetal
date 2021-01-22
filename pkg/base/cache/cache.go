package cache

import (
	"errors"
	"sync"
)

// Interface contains methods for caches
type Interface interface {
	Get(key string) (string, error)
	Set(key, object string)
	Delete(key string)
}

// MemCache is an implementation of Interface
type MemCache struct {
	items map[string]string
	sync.RWMutex
}

// NewBaseCache is a constructor for MemCache
func NewBaseCache() *MemCache {
	return &MemCache{
		items: make(map[string]string),
	}
}

// Get return item from MemCache using given key or error in case of missing key
func (c *MemCache) Get(key string) (string, error) {
	c.Lock()
	defer c.Unlock()

	item, ok := c.items[key]
	if !ok {
		return "", errors.New("item wasn't found")
	}
	return item, nil
}

// Set adds object to MemCache with given key
func (c *MemCache) Set(key, object string) {
	c.Lock()
	defer c.Unlock()

	c.items[key] = object
}

// Delete deletes object from MemCache with given key
func (c *MemCache) Delete(key string) {
	c.Lock()
	defer c.Unlock()

	delete(c.items, key)
}
