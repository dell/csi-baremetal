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
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/logger"
	"github.com/dell/csi-baremetal/pkg/base/logger/objects"
	"github.com/dell/csi-baremetal/pkg/scheduler/util"
	coreV1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// CSISchedulerPlugin is a plugin that does placement decision based on information in AC CRD
type CSISchedulerPlugin struct {
	frameworkHandle framework.Handle
	schedulerUtils  *util.SchedulerUtils
}

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "CSISchedulerPlugin"
)

// please refer to https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/ for details
// Filter plugin
var _ framework.FilterPlugin = &CSISchedulerPlugin{}

// Score plugin
//var _ framework.ScorePlugin = &CSISchedulerPlugin{}

// Reserve plugin
//var _ framework.ReservePlugin = &CSISchedulerPlugin{}

// Unreserve plugin
//var _ framework.UnreservePlugin = &CSISchedulerPlugin{}

// Name returns name of plugin
func (c CSISchedulerPlugin) Name() string {
	return Name
}

// New initializes a new plugin and returns it.
func New(configuration runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	logger, _ := logger.InitLogger("", logger.DebugLevel)
	stopCH := ctrl.SetupSignalHandler()
	k8sClient, err := k8s.GetK8SClient()
	if err != nil {
		logger.Fatal(err)
	}
	kubeClient := k8s.NewKubeClient(k8sClient, logger, objects.NewObjectLogger(), "default")
	kubeCache, err := k8s.InitKubeCache(stopCH, logger,
		&coreV1.PersistentVolumeClaim{},
		&storageV1.StorageClass{},
		&volumecrd.Volume{})
	featureConf := featureconfig.NewFeatureConfig()
	featureConf.Update(featureconfig.FeatureNodeIDFromAnnotation, true)
	featureConf.Update(featureconfig.FeatureExternalAnnotationForNode, false)
	schedulerUtils, error := util.NewSchedulerUtils(logger, kubeClient, kubeCache, "csi-baremetal", featureConf, "", "")
	if error != nil {
		logger.Fatalf("Fail to create extender: %v", err)
	}
	sp := &CSISchedulerPlugin{
		frameworkHandle: handle,
		schedulerUtils:  schedulerUtils,
	}
	return sp, nil
}

// Filter filters out nodes which don't have ACs match to PVCs
func (c *CSISchedulerPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	klog.V(2).Infof("CSISchedulerPlugin Filer")
	if !c.schedulerUtils.FilterPlugin(ctx, pod, nodeInfo.Node()) {
		framework.NewStatus(framework.UnschedulableAndUnresolvable, "inadequate storage capacity")
	}
	return nil
}

// Score does balancing across the nodes for better performance. Nodes with more ACs should have highest scores
//func (c CSISchedulerPlugin) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
//	panic("implement me")
//}

//// ScoreExtensions returns a ScoreExtensions interface if it implements one, or nil if does not.
//func (c CSISchedulerPlugin) ScoreExtensions() framework.ScoreExtensions {
//	panic("implement me")
//}

// Reserve does reservation of ACs
//func (c CSISchedulerPlugin) Reserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
//	panic("implement me")
//}

// Unreserve un-reserver ACs
//func (c CSISchedulerPlugin) Unreserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
//	panic("implement me")
//}
