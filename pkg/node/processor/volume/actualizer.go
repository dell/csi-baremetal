package volume

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/node"
)

const ctxTimeout = 30 * time.Second

type actualizer struct {
	client client.Client
	// kubernetes node ID
	nodeID        string
	eventRecorder eventRecorder
	vmgr          *node.VolumeManager
	log           *logrus.Entry
}

func (a *actualizer) Handle(ctx context.Context) {
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

		path, err := a.vmgr.GetProvisionerForVolume(&volumes.Items[i].Spec).GetVolumePath(&volumes.Items[i].Spec)
		if err != nil {
			a.log.Errorf("failed to get volume path: %s", err.Error())
			return
		}

		isMounted, err := a.vmgr.GetFSOps().IsMounted(path)
		if err != nil {
			a.log.Errorf("failed to check mounttt point: %s", err.Error())
			return
		}

		if volumes.Items[i].Spec.Mounted == isMounted {
			continue
		}

		isRemoved := a.ownerPodsAreRemoved(ctx, &volumes.Items[i])

		a.eventRecorder.Eventf(&volumes.Items[i], eventing.VolumeUnexpectedMount,
			"Volume mount status is unexpected. Status: %t. Real: %t. Owner pod removed: %t",
			volumes.Items[i].Spec.Mounted, isMounted, isRemoved)

		volumes.Items[i].Spec.Mounted = isMounted

		if err := a.client.Update(ctx, &volumes.Items[i]); err != nil {
			a.log.Errorf("failed to actualize Volume %s: %s", volumes.Items[i].GetName(), err.Error())
			continue
		}
		a.log.Debugf("Volume %s was successfully actualized", volumes.Items[i].GetName())
	}
}

func (a *actualizer) ownerPodsAreRemoved(ctx context.Context, volume *volumecrd.Volume) bool {
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

// NewVolumeActualizer creates new Volume actualizer
func NewVolumeActualizer(client client.Client, nodeID string, eventRecorder eventRecorder,
	vmgr *node.VolumeManager, log *logrus.Entry) processor {
	return &actualizer{
		client:        client,
		nodeID:        nodeID,
		eventRecorder: eventRecorder,
		vmgr:          vmgr,
		log:           log,
	}
}
