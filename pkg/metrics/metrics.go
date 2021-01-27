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

	"github.com/prometheus/client_golang/prometheus"
)

// Statistic is a common interface for histogram metrics
type Statistic interface {
	Collect() prometheus.Collector
	EvaluateDuration(labels prometheus.Labels) func()
	EvaluateDurationForMethod(method string) func()
}

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

// Collect returns prometheus.Collector slice with OperationsDuration histogram
func (m *Metrics) Collect() prometheus.Collector {
	return m.OperationsDuration
}
