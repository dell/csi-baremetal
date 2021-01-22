package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Statistic is a common interface for histogram metrics
type Statistic interface {
	Collect() prometheus.Collector
	EvaluateDuration(method string, start time.Time)
}

// VolumeMetrics is a structure, which encapsulate prometheus histogram structure. It used for volume operation metrics
type VolumeMetrics struct {
	VolumeOperationsDuration *prometheus.HistogramVec
}

// NewVolumeMetrics initializes volume metrics
func NewVolumeMetrics() *VolumeMetrics {
	vm := &VolumeMetrics{}

	vm.VolumeOperationsDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "volume_operations_duration",
		Help:    "Volume operations methods duration",
		Buckets: prometheus.ExponentialBuckets(0.001, 1.5, 25),
	}, []string{"method"})

	return vm
}

// Collect returns prometheus.Collector slice with volume operations histogram
func (vm *VolumeMetrics) Collect() prometheus.Collector {
	return vm.VolumeOperationsDuration
}

// EvaluateDuration evaluate duration from start for given method and put it into histogram
// Receive method name as a string, start time ad time.Time
func (vm *VolumeMetrics) EvaluateDuration(method string, start time.Time) {
	duration := time.Since(start)
	vm.VolumeOperationsDuration.With(prometheus.Labels{
		"method": method,
	}).Observe(duration.Seconds())
}

// PartitionsMetrics is a structure, which encapsulate prometheus histogram structure. It used for partition operation metrics
type PartitionsMetrics struct {
	PartitionOperationsDuration *prometheus.HistogramVec
}

// NewPartitionsMetrics initializes partitions metrics.
func NewPartitionsMetrics() *PartitionsMetrics {
	pm := &PartitionsMetrics{}

	pm.PartitionOperationsDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "partition_operations_duration",
		Help:    "Partition operations methods duration",
		Buckets: prometheus.ExponentialBuckets(0.001, 1.5, 25),
	}, []string{"method"})

	return pm
}

// Collect returns prometheus.Collector slice with partition operations histogram
func (pm *PartitionsMetrics) Collect() prometheus.Collector {
	return pm.PartitionOperationsDuration
}

// EvaluateDuration evaluate duration from start for given method and put it into histogram
// Receive method name as a string, start time ad time.Time
func (pm *PartitionsMetrics) EvaluateDuration(method string, start time.Time) {
	duration := time.Since(start)
	pm.PartitionOperationsDuration.With(prometheus.Labels{
		"method": method,
	}).Observe(duration.Seconds())
}
