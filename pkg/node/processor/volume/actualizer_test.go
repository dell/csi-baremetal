package volume

import (
	"context"
	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/mocks"
	mockprov "github.com/dell/csi-baremetal/pkg/mocks/provisioners"
	"github.com/dell/csi-baremetal/pkg/node"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

var (
	testCtx = context.Background()

	volumeCR = vcrd.Volume{
		TypeMeta: v1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: v1.ObjectMeta{
			Name:      testVolume,
			Namespace: testNs,
		},
		Spec: api.Volume{
			Id:           testVolume,
			Size:         1024 * 1024 * 1024 * 150,
			StorageClass: apiV1.StorageClassHDD,
			Location:     testLocation,
			CSIStatus:    apiV1.Creating,
			NodeId:       nodeID,
			Mode:         apiV1.ModeFS,
			Type:         string(fs.XFS),
		},
	}

	pod = coreV1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      testVolume,
			Namespace: testNs,
		},
	}
)

const (
	testNs   = "test-ns"
	nodeID   = "test-node-id"
	nodeName = "test-node-name"

	testVolume   = "test-volume"
	testLocation = "test-location"
)

func TestVolumeActualizer_OwnerPodsAreRemoved(t *testing.T) {

	t.Run("owner exists", func(t *testing.T) {
		var (
			a   = newActualizer()
			vol = volumeCR.DeepCopy()
			po  = pod.DeepCopy()
		)

		vol.Spec.Owners = []string{po.GetName()}
		err := a.client.Create(testCtx, vol)
		assert.Nil(t, err)

		err = a.client.Create(testCtx, po)
		assert.Nil(t, err)

		isRemoved := a.ownerPodsAreRemoved(testCtx, vol)
		assert.False(t, isRemoved)
	})

	t.Run("owner removed", func(t *testing.T) {
		var (
			a   = newActualizer()
			vol = volumeCR.DeepCopy()
			po  = pod.DeepCopy()
		)

		vol.Spec.Owners = []string{po.GetName()}
		err := a.client.Create(testCtx, vol)
		assert.Nil(t, err)

		isRemoved := a.ownerPodsAreRemoved(testCtx, vol)
		assert.True(t, isRemoved)
	})

	t.Run("failed to get pod", func(t *testing.T) {
		var (
			a   = newActualizer()
			vol = volumeCR.DeepCopy()
			po  = pod.DeepCopy()
		)

		vol.Spec.Owners = []string{po.GetName()}
		err := a.client.Create(testCtx, vol)
		assert.Nil(t, err)

		isRemoved := a.ownerPodsAreRemoved(k8s.GetFailCtx, vol)
		assert.False(t, isRemoved)
	})
}

func TestVolumeActualizer_Handle(t *testing.T) {

	t.Run("change Mount status", func(t *testing.T) {
		var (
			a   = newActualizer()
			vol = volumeCR.DeepCopy()
		)

		vol.Spec.CSIStatus = apiV1.Published
		vol.Spec.Mounted = true
		err := a.client.Create(testCtx, vol)
		assert.Nil(t, err)

		eventRecorder := new(mocks.NoOpRecorder)
		a.eventRecorder = eventRecorder

		partitionPath := "/partition/path/for/volume1"
		prov := mockprov.MockProvisioner{}
		a.vmgr.SetProvisioners(map[p.VolumeType]p.Provisioner{
			p.DriveBasedVolumeType: &prov,
		})
		prov.On("GetVolumePath", &vol.Spec).Return(partitionPath, nil)

		fs := mockprov.MockFsOpts{}
		a.vmgr.SetFSOps(&fs)
		fs.On("IsMounted", partitionPath).Return(false, nil)

		a.Handle(testCtx)

		resVol := &vcrd.Volume{}
		err = a.client.Get(testCtx, client.ObjectKey{Name: testVolume, Namespace: testNs}, resVol)
		assert.Nil(t, err)
		assert.False(t, resVol.Spec.Mounted)

		assert.Equal(t, 1, len(eventRecorder.Calls))
		assert.Equal(t, eventing.VolumeUnexpectedMount, eventRecorder.Calls[0].Event)
	})
}

func newActualizer() *actualizer {
	testLogger := logrus.New()
	testLogger.SetLevel(logrus.DebugLevel)

	client := mocks.NewMockDriveMgrClient(mocks.DriveMgrRespDrives)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}

	e := command.NewExecutor(testLogger)
	volumeManager := node.NewVolumeManager(client, e, testLogger, kubeClient, kubeClient, new(mocks.NoOpRecorder), nodeID, nodeName)

	return &actualizer{
		client:        kubeClient,
		nodeID:        nodeID,
		eventRecorder: new(mocks.NoOpRecorder),
		vmgr:          volumeManager,
		log:           testLogger.WithField("component", "actualizer-test"),
	}
}
