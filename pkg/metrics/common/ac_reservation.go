/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package common

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/dell/csi-baremetal/pkg/metrics"
)

// ReservationDuration used to collect ReservationHelper methods durations
var ReservationDuration = metrics.NewMetrics(prometheus.HistogramOpts{
	Name:    "ac_reservation_duration",
	Help:    "AvailableCapacity reservation duration",
	Buckets: prometheus.ExponentialBuckets(0.005, 1.5, 10),
}, "method")

// DbgScheduleTotalTime used to collect schedule total time
var DbgScheduleTotalTime = metrics.NewMetricsWithCustomLabels(prometheus.GaugeOpts{
	Name: "schedule_total_time",
	Help: "total schedule time cose for pod",
}, "source", "pod_name")

// DbgScheduleSinceLastTime used to collect schedule interval time
var DbgScheduleSinceLastTime = metrics.NewMetricsWithCustomLabels(prometheus.GaugeOpts{
	Name: "schedule_since_last_time",
	Help: "schedule interval since last time",
}, "source", "pod_name")

// DbgScheduleCounter used to collect schedule totol counts
var DbgScheduleCounter = metrics.NewCounterWithCustomLabels(prometheus.CounterOpts{
	Name: "schedule_counter",
	Help: "schedule counter for pod",
}, "source", "pod_name")

// nolint: gochecknoinits
func init() {
	prometheus.MustRegister(ReservationDuration.Collect())
	prometheus.MustRegister(DbgScheduleTotalTime.Collect())
	prometheus.MustRegister(DbgScheduleSinceLastTime.Collect())
	prometheus.MustRegister(DbgScheduleCounter.Collect())
}
