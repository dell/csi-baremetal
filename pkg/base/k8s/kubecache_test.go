/*
Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestKubeCache_GetK8SCache(t *testing.T) {
	config := &rest.Config{}
	origFun := ctrl.GetConfigOrDie

	defer func() {
		ctrl.GetConfigOrDie = origFun
	}()

	ctrl.GetConfigOrDie = func() *rest.Config {
		return config
	}

	cache, err := GetK8SCache()
	assert.Nil(t, err)
	assert.NotNil(t, cache)
}
