package node

import (
	"errors"
	"fmt"
	"google.golang.org/grpc/health/grpc_health_v1"
	"testing"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
)

var (
	node *CSINodeService
	ctx  = context.Background()
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

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodePublish() failure", func() {
		It("Should fail with missing volume capabilities", func() {
			req := &csi.NodePublishVolumeRequest{}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume capability missing in request"))
		})
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodePublishVolumeRequest{
				TargetPath:       targetPath,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodePublishVolumeRequest{
				VolumeId:         volumeID,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})
		It("Should fail with volume cache error", func() {
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)
			delete(node.volumesCache, volumeID)

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("There is no volume with appropriate VolumeID"))
		})
		It("Should fail with search device by S/N error", func() {
			req := getNodePublishRequest("volume-id-3", targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("unable to find device for drive with S/N"))
		})
		It("Should fail with PrepareVolume() error", func() {
			scImplMock.On("PrepareVolume", device, targetPath).
				Return(false, errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)
			node.volumesCache["volume-id"] = &api.Volume{
				Id:       volumeID,
				Owner:    "test",
				Location: "hdd1",
			}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(fmt.Sprintf("failed to publish volume %s", volumeID)))
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

			resp, err := node.NodeUnpublishVolume(ctx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodeUnPublish() failure", func() {
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodeUnpublishVolumeRequest{
				TargetPath: targetPath,
			}

			resp, err := node.NodeUnpublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodeUnpublishVolumeRequest{
				VolumeId: volumeID,
			}

			resp, err := node.NodeUnpublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})

		It("Should fail with Unmount() error", func() {
			scImplMock.On("Unmount", targetPath).Return(errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeUnpublishRequest(volumeID, targetPath)

			resp, err := node.NodeUnpublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Unable to unmount"))
		})
	})
})

var _ = Describe("CSINodeService NodeGetInfo()", func() {
	It("Should return topology key with Node ID", func() {
		node := newNodeService()

		resp, err := node.NodeGetInfo(ctx, &csi.NodeGetInfoRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		val, ok := resp.AccessibleTopology.Segments["baremetal-csi/nodeid"]
		Expect(ok).To(BeTrue())
		Expect(val).To(Equal(nodeID))
	})
})

/*
func (s *CSINodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	ll := s.log.WithFields(logrus.Fields{
		"method": "NodeGetInfo",
	})

	topology := csi.Topology{
		Segments: map[string]string{
			"baremetal-csi/nodeid": s.NodeID,
		},
	}

	ll.Infof("NodeGetInfo created topology: %v", topology)

	return &csi.NodeGetInfoResponse{
		NodeId:             s.NodeID,
		AccessibleTopology: &topology,
	}, nil
}
*/

var _ = Describe("CSINodeService Check()", func() {
	It("Should return serving", func() {
		node := newNodeService()

		resp, err := node.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		Expect(resp.Status).To(Equal(grpc_health_v1.HealthCheckResponse_SERVING))
	})
	It("Should return  not serving", func() {
		node := newNodeService()
		node.drivesCache = make(map[string]*api.Drive)
		resp, err := node.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		Expect(resp.Status).To(Equal(grpc_health_v1.HealthCheckResponse_NOT_SERVING))
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
	node.volumesCache["volume-id-3"] = &api.Volume{Id: volumeID, Owner: "test", Location: "hdd3"}

	node.drivesCache["disks-1"] = &api.Drive{SerialNumber: "hdd1", Size: 1024 * 1024 * 1024 * 500}
	node.drivesCache["disks-2"] = &api.Drive{SerialNumber: "hdd2", Size: 1024 * 1024 * 1024 * 200}
	node.drivesCache["disks-3"] = &api.Drive{SerialNumber: "hdd3", Type: api.DriveType_HDD}

	return node
}
