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

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMemCache(t *testing.T) {
	memCache := NewMemCache()
	assert.Equal(t, 0, len(memCache.items))
}

func TestMemCache_Set(t *testing.T) {
	memCache := NewMemCache()
	memCache.Set("test", "test")

	assert.Equal(t, "test", memCache.items["test"])
}

func TestMemCache_Get(t *testing.T) {
	memCache := NewMemCache()
	memCache.Set("test", "test")

	value, err := memCache.Get("test")
	assert.Nil(t, err)
	assert.Equal(t, value, memCache.items["test"])

	_, err = memCache.Get("unknown_key")
	assert.NotNil(t, err)
}

func TestMemCache_Delete(t *testing.T) {
	memCache := NewMemCache()
	memCache.Set("test", "test")

	assert.Equal(t, "test", memCache.items["test"])

	memCache.Delete("test")
	assert.Equal(t, "", memCache.items["test"])
}
