package volume

import (
	"context"
	"github.com/dell/csi-baremetal/pkg/node/actions/volume/unstage/errors"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/eventing"
)

const ctxTimeout = 30 * time.Second

type actualizer struct {
	client                      client.Client
	unstageVolumeActionsFactory unstageVolumeActionsFactory
	// kubernetes node ID
	nodeID        string
	eventRecorder eventRecorder
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

		// If volume is published and all of its owner pods are removed - then this volume becomes orphaned.
		// This situation is possible in case of CSI driver is unregistered within kubelet before pod is terminated:
		// in this case kubelet manage physical volume mounts by itself and skip volume stage/publish operations over CSI drivers.
		// So in this case volume CR should be actualized by ourselves.
		if volumes.Items[i].Spec.CSIStatus == v1.Published && a.ownerPodsAreRemoved(ctx, &volumes.Items[i]) {
			// performing volume unstage actions appliance
			switch err := a.unstageVolumeActionsFactory.CreateUnstageVolumeActions(&volumes.Items[i],
				volumes.Items[i].Spec.GlobalMountPath,
			).Apply(ctx); err.(type) {
			case nil:
				volumes.Items[i].Spec.CSIStatus = v1.Created
				break

			case errors.UnmountVolumeError:
				a.log.Errorf("failed to unmount Volume '%s', error: '%v'", volumes.Items[i].GetName(), err)
				continue

			case errors.RestoreWBTError:
				a.log.Errorf("Unable to restore WBT value for volume %s: %v", volumes.Items[i].Name, err)
				a.eventRecorder.Eventf(&volumes.Items[i], eventing.WBTValueSetFailed,
					"Unable to restore WBT value for volume %s", volumes.Items[i].Name)
				continue

			default:
				a.log.Errorf("Unknown error occurred during volume unstage actions appliance %s: %v", volumes.Items[i].Name, err)
				continue
			}

			if err := a.client.Update(ctx, &volumes.Items[i]); err != nil {
				a.log.Errorf("failed to actualize Volume %s: %s", volumes.Items[i].GetName(), err.Error())
				continue
			}
			a.log.Infof("Volume %s was successfully actualized", volumes.Items[i].GetName())
		}
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
func NewVolumeActualizer(client client.Client, unstageVolumeActionsFactory unstageVolumeActionsFactory,
	nodeID string, log *logrus.Entry,
) processor {
	return &actualizer{
		client:                      client,
		unstageVolumeActionsFactory: unstageVolumeActionsFactory,
		nodeID:                      nodeID,
		log:                         log,
	}
}
