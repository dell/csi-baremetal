package node

import (
	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
	"errors"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var (
	node *CSINodeService
	ctx  context.Context
)

const (
	nodeID     = "fake-node"
	device     = "/dev/sda1"
	volumeID   = "volume-id"
	targetPath = "/tmp/targetPath"
)

func TestCSINodeService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSIControllerService testing suite")
}

var _ = Describe("CSINodeService NodePublish()", func() {
	var node *CSINodeService
	scImplMock := &sc.ImplementerMock{}

	volumeCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{
				FsType: "xfs",
			},
		},
		AccessMode: &csi.VolumeCapability_AccessMode{
			Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		},
	}

	BeforeEach(func() {
		node = newNodeService()
	})

	Context("NodePublish() success", func() {
		It("Should publish volume", func() {
			scImplMock.On("PrepareVolume", device, targetPath).Return(false, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodePublish() failure", func() {
		It("Should fail with missing volume capabilities", func() {
			req := &csi.NodePublishVolumeRequest{}

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume capability missing in request"))
		})
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodePublishVolumeRequest{
				TargetPath:       targetPath,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodePublishVolumeRequest{
				VolumeId:         volumeID,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})
		It("Should fail with volume cache error", func() {
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)
			node.volumesCache["volume-id"] = nil

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("There is no volume with appropriate VolumeID"))
		})
		It("Should fail with search device by S/N error", func() {
			req := getNodePublishRequest("volume-id-2", targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("unable to find device for drive with S/N"))
		})
		It("Should fail with PrepareVolume() error", func() {
			scImplMock.On("PrepareVolume", device, targetPath).
				Return(false, errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("unable to publish volume"))
		})

		It("Should fail with PrepareVolume() error", func() {
			scImplMock.On("PrepareVolume", device, targetPath).
				Return(true, errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("error"))
		})
	})
})

var _ = Describe("CSINodeService NodeUnPublish()", func() {
	var node *CSINodeService
	scImplMock := &sc.ImplementerMock{}

	BeforeEach(func() {
		node = newNodeService()
	})

	Context("NodeUnPublish() success", func() {
		It("Should unpublish volume", func() {
			scImplMock.On("Unmount", targetPath).Return(nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock

			req := getNodeUnpublishRequest(volumeID, targetPath)

			resp, err := node.NodeUnpublishVolume(context.Background(), req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodeUnPublish() failure", func() {
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodeUnpublishVolumeRequest{
				TargetPath: targetPath,
			}

			resp, err := node.NodeUnpublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodeUnpublishVolumeRequest{
				VolumeId: volumeID,
			}

			resp, err := node.NodeUnpublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})

		It("Should fail with Unmount() error", func() {
			scImplMock.On("Unmount", targetPath).Return(errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeUnpublishRequest(volumeID, targetPath)

			resp, err := node.NodeUnpublishVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Unable to unmount"))
		})
	})
})

func getNodePublishRequest(volumeID, targetPath string, volumeCap csi.VolumeCapability) *csi.NodePublishVolumeRequest {
	return &csi.NodePublishVolumeRequest{
		VolumeId:         volumeID,
		TargetPath:       targetPath,
		VolumeCapability: &volumeCap,
	}
}

func getNodeUnpublishRequest(volumeID, targetPath string) *csi.NodeUnpublishVolumeRequest {
	return &csi.NodeUnpublishVolumeRequest{
		VolumeId:   volumeID,
		TargetPath: targetPath,
	}
}

func newNodeService() *CSINodeService {
	client := mocks.NewMockHWMgrClient(mocks.HwMgrRespDrives)
	executor := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	node = NewCSINodeService(client, nodeID, logrus.New())

	node.VolumeManager.SetExecutor(executor)

	node.volumesCache["volume-id"] = &api.Volume{Id: volumeID, Owner: "test", Location: "hdd1"}
	node.volumesCache["volume-id-2"] = &api.Volume{Id: volumeID, Owner: "test", Location: ""}

	return node
}
