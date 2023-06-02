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

// Package metrics is for metrics, used in CSI
package metrics

import (
	"time"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	//nolint
	Branch string
	//nolint
	Revision string
)

// BuildInfo used to expose information of the current build
var BuildInfo = prometheus.NewGaugeFunc(
	prometheus.GaugeOpts{
		Name:        "build_info",
		Help:        "A metric with a constant '1' value labeled by version, revision, branch",
		ConstLabels: prometheus.Labels{"version": base.PluginVersion, "revision": Revision, "branch": Branch},
	},
	func() float64 { return 1 },
)

// DbgMetricHoldTime is Dbg metric hold time,
// after that, Dbg metric will be cleaned up in case
// more and more metrics exists.
const DbgMetricHoldTime = 3 * 60
const metricWithCustomLabels = "MetricsWithCustomLabels"

// EventReasonKilling is the delimiter keyword used to distinguish if pod event is for current pod,
// not for same name pod already killed, which is useful to calculate scheduling metrics.
const EventReasonKilling = "Killing"

// EventReasonFailedScheduling is the keyword to find out failed scheduling event, which is useful
// to calculate  scheduling metrics.
const EventReasonFailedScheduling = "FailedScheduling"

// Statistic is a common interface for histogram metrics
type Statistic interface {
	Collect() prometheus.Collector
	EvaluateDuration(labels prometheus.Labels) func()
	EvaluateDurationForMethod(method string) func()
	EvaluateDurationForType(t string) func()
}

// ExtendedDefBuckets is a default buckets used for csi driver
var ExtendedDefBuckets = []float64{.025, .05, .1, .5, 1, 3, 5, 10, 15, 30, 45, 90, 180}

// Metrics is a structure, which encapsulate prometheus histogram structure. It used for volume operation metrics
type Metrics struct {
	OperationsDuration *prometheus.HistogramVec
}

// NewMetrics initializes operations duration metrics
func NewMetrics(opts prometheus.HistogramOpts, labels ...string) *Metrics {
	return &Metrics{OperationsDuration: prometheus.NewHistogramVec(opts, labels)}
}

// EvaluateDuration evaluate duration from start for a given method and put it into histogram
// Receive prometheus.Labels.
func (m *Metrics) EvaluateDuration(labels prometheus.Labels) func() {
	start := time.Now()
	return func() {
		m.OperationsDuration.With(labels).Observe(time.Since(start).Seconds())
	}
}

// EvaluateDurationForMethod of the method call
func (m *Metrics) EvaluateDurationForMethod(method string) func() {
	return m.EvaluateDuration(prometheus.Labels{"method": method})
}

// EvaluateDurationForType evaluate function call with "type" label
func (m *Metrics) EvaluateDurationForType(t string) func() {
	return m.EvaluateDuration(prometheus.Labels{"type": t})
}

// Collect returns prometheus.Collector slice with OperationsDuration histogram
func (m *Metrics) Collect() prometheus.Collector {
	return m.OperationsDuration
}

// StatisticWithCustomLabels is Similar to Statistic but have Custom labels, but StatisticWithCustomLabels supports additional metrics labels,
// this can be useful when you want to create metrics with dynamic labels, which prometheus doesn't support in native
type StatisticWithCustomLabels interface {
	Collect() prometheus.Collector
	EvaluateDurationWithLabel(labels prometheus.Labels) func()
	EvaluateDurationForMethod(method string, labels prometheus.Labels) func()
	EvaluateDurationForType(t string, labels prometheus.Labels) func()
	UpdateValue(value float64, labels prometheus.Labels, clear bool, clearLabels prometheus.Labels)
	Clear(label prometheus.Labels)
}

// MetricWithCustomLabels is a structure, which encapsulate prometheus GaugeVec structure. It used for evalute duration,
// it support support dynamic labels to make it easy to distinguish metric data that has same metric name but different label.
type MetricWithCustomLabels struct {
	GaugeVec *prometheus.GaugeVec
}

// NewMetricsWithCustomLabels initializes MetricsWithCustomLabels
func NewMetricsWithCustomLabels(opts prometheus.GaugeOpts, labels ...string) *MetricWithCustomLabels {
	return &MetricWithCustomLabels{GaugeVec: prometheus.NewGaugeVec(opts, labels)}
}

// Collect returns prometheus.Collector slice with internal GaugeVec
func (m *MetricWithCustomLabels) Collect() prometheus.Collector {
	return m.GaugeVec
}

// EvaluateDurationWithLabel evaluate duration with label
func (m *MetricWithCustomLabels) EvaluateDurationWithLabel(labels prometheus.Labels) func() {
	start := time.Now()
	return func() {
		m.GaugeVec.With(labels).Set(time.Since(start).Seconds())
	}
}

// EvaluateDurationForMethod evaluate duration of the method call with label
func (m *MetricWithCustomLabels) EvaluateDurationForMethod(method string, labels prometheus.Labels) func() {
	labels["source"] = metricWithCustomLabels
	labels["method"] = method
	return m.EvaluateDurationWithLabel(labels)
}

// EvaluateDurationForType evaluate function call with "type" label
func (m *MetricWithCustomLabels) EvaluateDurationForType(t string, labels prometheus.Labels) func() {
	labels["source"] = metricWithCustomLabels
	labels["type"] = t
	return m.EvaluateDurationWithLabel(labels)
}

// UpdateValue update value of metric with specific labels, also support to clear metric by labels
func (m *MetricWithCustomLabels) UpdateValue(value float64, labels prometheus.Labels, clear bool, clearLabels prometheus.Labels) {
	labels["source"] = metricWithCustomLabels
	if clear {
		m.GaugeVec.DeletePartialMatch(clearLabels)
	}
	m.GaugeVec.With(labels).Set(value)
}

// Clear clears metric by labels
func (m *MetricWithCustomLabels) Clear(labels prometheus.Labels) {
	m.GaugeVec.DeletePartialMatch(labels)
}

// Counter is a common interface for counter metrics
type Counter interface {
	Collect() prometheus.Collector
	Add(labels prometheus.Labels)
	Clear(label prometheus.Labels)
}

// CounterWithCustomLabels is a structure, which encapsulate prometheus CounterVec structure. It used for counts something.
// also supports additional metrics labels,
// this can be useful when you want to create metrics with dynamic labels, which prometheus doesn't support in native
type CounterWithCustomLabels struct {
	CounterVec *prometheus.CounterVec
}

// NewCounterWithCustomLabels initializes CounterWithCustomLabels
func NewCounterWithCustomLabels(opts prometheus.CounterOpts, labels ...string) *CounterWithCustomLabels {
	return &CounterWithCustomLabels{CounterVec: prometheus.NewCounterVec(opts, labels)}
}

// Collect returns prometheus.Collector slice with internal CounterVec
func (m *CounterWithCustomLabels) Collect() prometheus.Collector {
	return m.CounterVec
}

// Add adds value to metric with specific labels, also support to clear metric by labels
func (m *CounterWithCustomLabels) Add(labels prometheus.Labels) {
	labels["source"] = "CounterWithCustomLabels"
	m.CounterVec.With(labels).Add(1)
}

// Clear clears metric by labels
func (m *CounterWithCustomLabels) Clear(labels prometheus.Labels) {
	m.CounterVec.DeletePartialMatch(labels)
}
