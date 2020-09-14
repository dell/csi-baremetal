package plugin

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

// CSISchedulerPlugin is a plugin that does placement decision based on information in AC CRD
type CSISchedulerPlugin struct {
	frameworkHandle framework.FrameworkHandle
}

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "CSISchedulerPlugin"
)

// please refer to https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/ for details
// Filter plugin
var _ framework.FilterPlugin = &CSISchedulerPlugin{}

// Score plugin
var _ framework.ScorePlugin = &CSISchedulerPlugin{}

// Reserve plugin
var _ framework.ReservePlugin = &CSISchedulerPlugin{}

// Unreserve plugin
var _ framework.UnreservePlugin = &CSISchedulerPlugin{}

// Name returns name of plugin
func (c CSISchedulerPlugin) Name() string {
	return Name
}

// New initializes a new plugin and returns it.
func New(configuration *runtime.Unknown, handle framework.FrameworkHandle) (framework.Plugin, error) {
	sp := &CSISchedulerPlugin{frameworkHandle: handle}
	return sp, nil
}

// Filter filters out nodes which don't have ACs match to PVCs
func (c CSISchedulerPlugin) Filter(pc *framework.PluginContext, pod *v1.Pod, nodeName string) *framework.Status {
	panic("implement me")
}

// Score does balancing across the nodes for better performance. Nodes with more ACs should have highest scores
func (c CSISchedulerPlugin) Score(pc *framework.PluginContext, p *v1.Pod, nodeName string) (int, *framework.Status) {
	panic("implement me")
}

// Reserve does reservation of ACs
func (c CSISchedulerPlugin) Reserve(pc *framework.PluginContext, p *v1.Pod, nodeName string) *framework.Status {
	panic("implement me")
}

// Unreserve un-reserver ACs
func (c CSISchedulerPlugin) Unreserve(pc *framework.PluginContext, p *v1.Pod, nodeName string) {
	panic("implement me")
}
