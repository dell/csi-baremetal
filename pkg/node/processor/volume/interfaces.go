package volume

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/node/actions"
)

type processor interface {
	Handle(ctx context.Context)
}

type unstageVolumeActionsFactory interface {
	CreateUnstageVolumeActions(volume *volumecrd.Volume, stagingTargetPath string) actions.Actions
}

type eventRecorder interface {
	Eventf(object runtime.Object, event *eventing.EventDescription, messageFmt string, args ...interface{})
}
