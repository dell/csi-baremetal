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

// DbgCreateVolumeDuration used to collect ducation of Controller.CreateVolume
var DbgCreateVolumeDuration = metrics.NewMetricsWithCustomLabels(prometheus.GaugeOpts{
	Name: "controller_create_volume_duration_seconds",
	Help: "duration of the controller createVolume",
}, "source", "method", "volume_name")

// nolint: gochecknoinits
func init() {
	prometheus.MustRegister(DbgCreateVolumeDuration.Collect())
}
