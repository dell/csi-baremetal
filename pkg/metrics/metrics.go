package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type Statistic interface {
	Collect() []prometheus.Collector
	EvaluateDuration(method string, start time.Time)
}

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

func (vm *VolumeMetrics) Collect() []prometheus.Collector {
	return []prometheus.Collector{vm.VolumeOperationsDuration}
}

func (vm *VolumeMetrics) EvaluateDuration(method string, start time.Time) {
	duration := time.Since(start)
	vm.VolumeOperationsDuration.With(prometheus.Labels{
		"method": method,
	}).Observe(duration.Seconds())
}

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

func (pm *PartitionsMetrics) Collect() []prometheus.Collector {
	return []prometheus.Collector{pm.PartitionOperationsDuration}
}

func (pm *PartitionsMetrics) EvaluateDuration(method string, start time.Time) {
	duration := time.Since(start)
	pm.PartitionOperationsDuration.With(prometheus.Labels{
		"method": method,
	}).Observe(duration.Seconds())
}
