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

// BaseCache is an implementation of Interface
type BaseCache struct {
	items map[string]string
	sync.RWMutex
}

// NewBaseCache is a constructor for BaseCache
func NewBaseCache() *BaseCache {
	return &BaseCache{
		items: make(map[string]string),
	}
}

// Get return item from BaseCache using given key or error in case of missing key
func (c *BaseCache) Get(key string) (string, error) {
	c.Lock()
	defer c.Unlock()

	item, ok := c.items[key]
	if !ok {
		return "", errors.New("item wasn't found")
	}
	return item, nil
}

// Set adds object to BaseCache with given key
func (c *BaseCache) Set(key, object string) {
	c.Lock()
	defer c.Unlock()

	c.items[key] = object
}

// Delete deletes object from BaseCache with given key
func (c *BaseCache) Delete(key string) {
	c.Lock()
	defer c.Unlock()

	delete(c.items, key)
}
