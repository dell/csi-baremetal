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

package plugin

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
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
func (c CSISchedulerPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *schedulernodeinfo.NodeInfo) *framework.Status {
	panic("implement me")
}

// Score does balancing across the nodes for better performance. Nodes with more ACs should have highest scores
func (c CSISchedulerPlugin) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
	panic("implement me")
}

// ScoreExtensions returns a ScoreExtensions interface if it implements one, or nil if does not.
func (c CSISchedulerPlugin) ScoreExtensions() framework.ScoreExtensions {
	panic("implement me")
}

// Reserve does reservation of ACs
func (c CSISchedulerPlugin) Reserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	panic("implement me")
}

// Unreserve un-reserver ACs
func (c CSISchedulerPlugin) Unreserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
	panic("implement me")
}
