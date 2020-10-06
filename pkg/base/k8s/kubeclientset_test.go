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
	"k8s.io/client-go/rest"
	"testing"
)

// Since we run test not in k8s, it will always fail
// It ain't much but it's honest work.
func TestGetK8SClientset(t *testing.T) {
	_, err := GetK8SClientset()
	if err != rest.ErrNotInCluster {
		t.Errorf("GetK8SClientset() error got = %v, want %v", err, rest.ErrNotInCluster)
	}
}
