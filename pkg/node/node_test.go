package node

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"errors"
	"fmt"

	"github.com/google/uuid"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"google.golang.org/grpc/health/grpc_health_v1"

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
	volumeid2  = "volume-id-2"
	volumeid3  = "volume-id-3"
	targetPath = "/tmp/targetPath"
	stagePath  = "/tmp/stagePath"
)

var (
	disk1 = api.Drive{UUID: uuid.New().String(), SerialNumber: "hdd1", Size: 1024 * 1024 * 1024 * 500, NodeId: nodeID}
	disk2 = api.Drive{UUID: uuid.New().String(), SerialNumber: "hdd2", Size: 1024 * 1024 * 1024 * 200, NodeId: nodeID}
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
			scImplMock.On("CreateTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("BindMount", stagePath, targetPath, false).Return(nil).Times(1)
			scImplMock.On("DeleteTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
		It("Target path already mounted", func() {
			scImplMock.On("IsMountPoint", targetPath).Return(true, nil).Times(1)
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
		It("Should fail with missing stage path", func() {
			req := &csi.NodePublishVolumeRequest{
				VolumeId:         volumeID,
				VolumeCapability: volumeCap,
				TargetPath:       targetPath,
			}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Stage path missing in request"))
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
		})
		It("Should fail with IsMountError error", func() {
			scImplMock.On("IsMountPoint", targetPath).Return(false, errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)
			node.volumesCache["volume-id"] = &api.Volume{
				Id:       volumeID,
				Owner:    "test",
				Location: disk1.UUID,
			}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(fmt.Sprintf("failed to publish volume %s", volumeID)))
		})
		It("Should fail with CreateTargetPath error", func() {
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			scImplMock.On("CreateTargetPath", targetPath).Return(errors.New("error")).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)
			node.volumesCache["volume-id"] = &api.Volume{
				Id:       volumeID,
				Owner:    "test",
				Location: disk1.UUID,
			}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(fmt.Sprintf("failed to publish volume %s", volumeID)))
		})
		It("Should fail with BindMount error", func() {
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			scImplMock.On("CreateTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("BindMount", stagePath, targetPath, false).Return(errors.New("error")).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(volumeID, targetPath, *volumeCap)
			node.volumesCache["volume-id"] = &api.Volume{
				Id:       volumeID,
				Owner:    "test",
				Location: disk1.UUID,
			}

			resp, err := node.NodePublishVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(fmt.Sprintf("failed to publish volume %s", volumeID)))
		})
	})
})

var _ = Describe("CSINodeService NodeStage()", func() {
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

	Context("NodeStage() success", func() {
		It("Should stage volume", func() {
			scImplMock.On("PrepareVolume", device, stagePath).Return(false, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(volumeID, *volumeCap)

			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
		It("Ready to remove status", func() {
			scImplMock.On("BindMount", device, stagePath, true).Return(nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(volumeID, *volumeCap)
			node.setVolumeStatus(volumeID, api.OperationalStatus_ReadyToRemove)
			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodeStage() failure", func() {
		It("Should fail with missing volume capabilities", func() {
			req := &csi.NodeStageVolumeRequest{}

			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume capability missing in request"))
		})
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodeStageVolumeRequest{
				StagingTargetPath: stagePath,
				VolumeCapability:  volumeCap,
			}

			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing stage path", func() {
			req := &csi.NodeStageVolumeRequest{
				VolumeId:         volumeID,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Stage Path missing in request"))
		})
		It("Should fail with volume cache error", func() {
			req := getNodeStageRequest(volumeID, *volumeCap)
			delete(node.volumesCache, volumeID)

			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("There is no volume with appropriate VolumeID"))
		})
		It("Should fail with search device by S/N error", func() {
			req := getNodeStageRequest("volume-id-3", *volumeCap)

			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("Should fail with PrepareVolume() error", func() {
			scImplMock.On("PrepareVolume", device, stagePath).
				Return(false, errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(volumeID, *volumeCap)
			node.volumesCache["volume-id"] = &api.Volume{
				Id:       volumeID,
				Owner:    "test",
				Location: disk1.UUID,
			}

			resp, err := node.NodeStageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(fmt.Sprintf("failed to stage volume %s", volumeID)))
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

var _ = Describe("CSINodeService NodeUnStage()", func() {
	var node *CSINodeService
	scImplMock := &sc.ImplementerMock{}

	BeforeEach(func() {
		node = newNodeService()
	})

	Context("NodeUnStage() success", func() {
		It("Should unstage volume", func() {
			scImplMock.On("Unmount", stagePath).Return(nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock

			req := getNodeUnstageRequest(volumeID, stagePath)

			resp, err := node.NodeUnstageVolume(ctx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodeUnPublish() failure", func() {
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: stagePath,
			}

			resp, err := node.NodeUnstageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodeUnstageVolumeRequest{
				VolumeId: volumeID,
			}

			resp, err := node.NodeUnstageVolume(ctx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Stage Path missing in request"))
		})

		It("Should fail with Unmount() error", func() {
			scImplMock.On("Unmount", targetPath).Return(errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeUnstageRequest(volumeID, targetPath)

			resp, err := node.NodeUnstageVolume(ctx, req)
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

var _ = Describe("CSINodeService NodeGetCapabilities()", func() {
	It("Should return STAGE_UNSTAGE_VOLUME capabilities", func() {
		node := newNodeService()

		resp, err := node.NodeGetCapabilities(ctx, &csi.NodeGetCapabilitiesRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		capabilities := resp.GetCapabilities()
		expectedCapability := &csi.NodeServiceCapability_Rpc{
			Rpc: &csi.NodeServiceCapability_RPC{
				Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
			},
		}
		Expect(len(capabilities)).To(Equal(1))
		Expect(capabilities[0].Type).To(Equal(expectedCapability))
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
		node.drivesCache = make(map[string]*drivecrd.Drive)
		resp, err := node.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		Expect(resp.Status).To(Equal(grpc_health_v1.HealthCheckResponse_NOT_SERVING))
	})
})

func getNodePublishRequest(volumeID, targetPath string, volumeCap csi.VolumeCapability) *csi.NodePublishVolumeRequest {
	return &csi.NodePublishVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stagePath,
		TargetPath:        targetPath,
		VolumeCapability:  &volumeCap,
	}
}

func getNodeStageRequest(volumeID string, volumeCap csi.VolumeCapability) *csi.NodeStageVolumeRequest {
	return &csi.NodeStageVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stagePath,
		VolumeCapability:  &volumeCap,
	}
}

func getNodeUnpublishRequest(volumeID, targetPath string) *csi.NodeUnpublishVolumeRequest {
	return &csi.NodeUnpublishVolumeRequest{
		VolumeId:   volumeID,
		TargetPath: targetPath,
	}
}

func getNodeUnstageRequest(volumeID, stagePath string) *csi.NodeUnstageVolumeRequest {
	return &csi.NodeUnstageVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stagePath,
	}
}

func newNodeService() *CSINodeService {
	client := mocks.NewMockHWMgrClient(mocks.HwMgrRespDrives)
	executor := mocks.NewMockExecutor(map[string]mocks.CmdOut{base.LsblkCmd: {Stdout: mocks.LsblkTwoDevicesStr}})
	kubeClient, err := base.GetFakeKubeClient(testNs)
	if err != nil {
		panic(err)
	}
	node = NewCSINodeService(client, nodeID, logrus.New(), kubeClient)

	node.VolumeManager.SetExecutor(executor)

	node.drivesCache[disk1.UUID] = &drivecrd.Drive{
		TypeMeta: v1.TypeMeta{
			Kind:       "Drive",
			APIVersion: "drive.dell.com/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      disk1.UUID,
			Namespace: "default",
		},
		Spec: disk1,
	}
	node.drivesCache[disk2.UUID] = &drivecrd.Drive{
		TypeMeta: v1.TypeMeta{
			Kind:       "Drive",
			APIVersion: "drive.dell.com/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      disk2.UUID,
			Namespace: "default",
		},
		Spec: disk2,
	}
	node.volumesCache["volume-id"] = &api.Volume{Id: volumeID, Owner: "test", Location: disk1.UUID}
	node.volumesCache["volume-id-2"] = &api.Volume{Id: volumeid2, Owner: "test", Location: ""}
	node.volumesCache["volume-id-3"] = &api.Volume{Id: volumeid3, Owner: "test", Location: "hdd3"}
	return node
}
