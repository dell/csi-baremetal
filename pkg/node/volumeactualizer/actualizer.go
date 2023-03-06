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

// Package volumeactualizer implements Actualizer
package volumeactualizer

import (
	"context"
	"time"

	apiV1 "github.com/dell/csi-baremetal/api/v1"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/polling"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/node"
)

const ctxTimeout = 30 * time.Second

type eventRecorder interface {
	Eventf(object runtime.Object, event *eventing.EventDescription, messageFmt string, args ...interface{})
}

// Actualizer is a polling loop fix volume status stucked in PUBLISHED.
// if all pods of a volume is deleted, when node is offline,
// K8s runtime will clean up volume directory and will not call CSI Unpublish interface.
type Actualizer struct {
	client client.Client
	// kubernetes node ID
	nodeID        string
	eventRecorder eventRecorder
	vmgr          *node.VolumeManager
	log           *logrus.Logger
}

// handle is polling loop handler
func (a *Actualizer) handle(ctx context.Context) {
	ctx, cancelFn := context.WithTimeout(ctx, ctxTimeout)
	defer cancelFn()

	volumes := &volumecrd.VolumeList{}
	if err := a.client.List(ctx, volumes); err != nil {
		a.log.Errorf("failed to get Volume List: %s", err.Error())
		return
	}

	for i := 0; i < len(volumes.Items); i++ {
		if volumes.Items[i].Spec.NodeId != a.nodeID {
			// if volume belongs to another node - then just skip it
			continue
		}

		// perform only for volumes in PUBLISHED or VOLUME_READY state
		if volumes.Items[i].Spec.CSIStatus != apiV1.Published && volumes.Items[i].Spec.CSIStatus != apiV1.VolumeReady {
			continue
		}

		isRemoved := a.ownerPodsAreRemoved(ctx, &volumes.Items[i])

		if !isRemoved {
			continue
		}
		a.eventRecorder.Eventf(&volumes.Items[i], eventing.VolumeStatusActualized,
			"Volume CSIStatus is unexpected. Status: %t. Real: %t. Owner pod removed: %t",
			volumes.Items[i].Spec.CSIStatus, apiV1.Published, isRemoved)

		volumes.Items[i].Spec.CSIStatus = apiV1.Published

		if err := a.client.Update(ctx, &volumes.Items[i]); err != nil {
			a.log.Errorf("failed to actualize Volume %s: %s", volumes.Items[i].GetName(), err.Error())
			continue
		}
		a.log.Debugf("Volume %s was successfully actualized", volumes.Items[i].GetName())
	}
}

func (a *Actualizer) ownerPodsAreRemoved(ctx context.Context, volume *volumecrd.Volume) bool {
	ownerPods := volume.Spec.GetOwners()

	pod := &corev1.Pod{}
	for i := 0; i < len(ownerPods); i++ {
		err := a.client.Get(ctx, client.ObjectKey{Name: ownerPods[i], Namespace: volume.Namespace}, pod)
		if err != nil && !k8serrors.IsNotFound(err) {
			a.log.Errorf("failed to get pod %s in %s namespace: %s", ownerPods[i], volume.Namespace, err.Error())
			return false
		}

		// Check if pod was deleted
		if k8serrors.IsNotFound(err) {
			a.log.Infof("Pod %s with Volume %s in %s ns was removed", ownerPods[i], volume.Namespace, volume.GetName())
			continue
		}

		// In case either of owner's pods have not deleted - just return false
		return false
	}

	return true
}

// Start polling
func (a *Actualizer) Start(ctx context.Context, dur time.Duration) {
	polling.NewTimer(dur).Start(ctx, a.handle)
}

// NewVolumeActualizer creates new Volume actualizer
func NewVolumeActualizer(client client.Client, nodeID string, eventRecorder eventRecorder,
	vmgr *node.VolumeManager, log *logrus.Logger) *Actualizer {
	return &Actualizer{
		client:        client,
		nodeID:        nodeID,
		eventRecorder: eventRecorder,
		vmgr:          vmgr,
		log:           log,
	}
}
