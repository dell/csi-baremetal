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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	vm := NewMetrics(prometheus.HistogramOpts{
		Name: "partition_operations_duration",
		Help: "partition operations methods duration",
	})
	rVM := vm.Collect()
	assert.Equal(t, vm.OperationsDuration, rVM)

	vm2 := NewMetricsWithCustomLabels(prometheus.GaugeOpts{
		Name: "test",
		Help: "test",
	}, "source", "pod_name")
	rVM2 := vm2.Collect()
	assert.Equal(t, vm2.GaugeVec, rVM2)

	assert.NotNil(t, vm2.EvaluateDuration(prometheus.Labels{"pod_name": "test_pod"}))
	assert.NotNil(t, vm2.EvaluateDurationForMethod("testMethod", prometheus.Labels{"pod_name": "test_pod"}))
	assert.NotNil(t, vm2.EvaluateDurationForType("testType", prometheus.Labels{"pod_name": "test_pod"}))
	assert.NotPanics(t, func() { vm2.UpdateValue(10.0, prometheus.Labels{"pod_name": "test_pod"}, false, prometheus.Labels{}) })
	assert.NotPanics(t, func() { vm2.Clear(prometheus.Labels{"pod_name": "test_pod"}) })

	vm3 := NewCounterWithCustomLabels(prometheus.CounterOpts{
		Name: "test counter",
		Help: "test counter",
	}, "source", "pod_name")
	rVM3 := vm3.Collect()
	assert.Equal(t, vm3.CounterVec, rVM3)
	assert.NotPanics(t, func() { vm3.Add(prometheus.Labels{"pod_name": "test_pod"}, false, prometheus.Labels{}) })
	assert.NotPanics(t, func() { vm3.Clear(prometheus.Labels{"pod_name": "test_pod"}) })
}
