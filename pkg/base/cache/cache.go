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

// Package cache contains common interface for caches with Get, Delete and Set methods and in memory cache implementation
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

// NewMemCache is a constructor for MemCache
func NewMemCache() *MemCache {
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
