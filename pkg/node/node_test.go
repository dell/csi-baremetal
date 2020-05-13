package node

import (
	"errors"
	"fmt"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/health/grpc_health_v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils"
	ph "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/linuxutils/partitionhelper"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/testutils"
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
			scImplMock.On("Mount", stagePath, targetPath, []string{"--bind"}).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
		It("Target path already mounted", func() {
			scImplMock.On("CreateTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", targetPath).Return(true, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodePublish() failure", func() {
		It("Should fail with missing volume capabilities", func() {
			req := &csi.NodePublishVolumeRequest{}

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume capability missing in request"))
		})
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodePublishVolumeRequest{
				TargetPath:       targetPath,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodePublishVolumeRequest{
				VolumeId:         testV1ID,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})
		It("Should fail with missing stage path", func() {
			req := &csi.NodePublishVolumeRequest{
				VolumeId:         testV1ID,
				VolumeCapability: volumeCap,
				TargetPath:       targetPath,
			}

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Staging Path missing in request"))
		})
		It("Should fail, because Volume has failed status", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.Failed
			err := node.k8sclient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("Should fail, because of volume CR isn't exist", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			err := node.k8sclient.DeleteCR(testCtx, &testVolumeCR1)
			Expect(err).To(BeNil())

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			// not a good ide to check error message. better to validate error code.
			Expect(err.Error()).To(ContainSubstring("Unable to find volume"))
		})
		It("Should fail with search device by S/N error", func() {
			req := getNodePublishRequest(testV3ID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("Should fail with IsMountError error", func() {
			scImplMock.On("CreateTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", targetPath).Return(false, errors.New("error")).Times(1)
			scImplMock.On("DeleteTargetPath", targetPath).Return(nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("failed to publish volume: mount error"))
		})
		It("Should fail with CreateTargetPath error", func() {
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			scImplMock.On("CreateTargetPath", targetPath).Return(errors.New("error")).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("failed to publish volume: mount error"))
		})
		It("Should fail with Mount error", func() {
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			scImplMock.On("CreateTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("DeleteTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("Mount", stagePath, targetPath, []string{"--bind"}).Return(errors.New("error")).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("failed to publish volume: mount error"))
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
			scImplMock.On("CreateTargetPath", stagePath).Return(nil).Times(1)
			scImplMock.On("Mount", device, stagePath, []string{""}).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", stagePath).Return(false, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(testV1ID, *volumeCap)

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
		It("VolumeReady status", func() {
			scImplMock.On("Mount", device, stagePath, []string(nil)).Return(nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(testV1ID, *volumeCap)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.VolumeReady
			err := node.k8sclient.UpdateCR(testCtx, &vol1)

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodeStage() failure", func() {
		It("Should fail with missing volume capabilities", func() {
			req := &csi.NodeStageVolumeRequest{}

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume capability missing in request"))
		})
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodeStageVolumeRequest{
				StagingTargetPath: stagePath,
				VolumeCapability:  volumeCap,
			}

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing stage path", func() {
			req := &csi.NodeStageVolumeRequest{
				VolumeId:         testV1ID,
				VolumeCapability: volumeCap,
			}

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Stage Path missing in request"))
		})
		It("Should fail, because of volume CR isn't exist", func() {
			req := getNodeStageRequest(testV1ID, *volumeCap)
			err := node.k8sclient.DeleteCR(testCtx, &testVolumeCR1)
			Expect(err).To(BeNil())

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("No volume with ID " + testV1ID + " found on node"))
		})
		It("Should fail with search device by S/N error", func() {
			req := getNodeStageRequest(testV3ID, *volumeCap)

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("Should fail with IsMountError error", func() {
			scImplMock.On("CreateTargetPath", stagePath).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", stagePath).Return(false, errors.New("error")).Times(1)
			scImplMock.On("DeleteTargetPath", stagePath).Return(nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(testV1ID, *volumeCap)

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("failed to stage volume"))
		})
		It("Should fail with CreateTargetPath error", func() {
			scImplMock.On("IsMountPoint", stagePath).Return(false, nil).Times(1)
			scImplMock.On("CreateTargetPath", stagePath).Return(errors.New("error")).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(testV1ID, *volumeCap)

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to stage volume"))
		})
		It("Should fail with Mount error", func() {
			scImplMock.On("IsMountPoint", stagePath).Return(false, nil).Times(1)
			scImplMock.On("DeleteTargetPath", stagePath).Return(nil).Times(1)
			scImplMock.On("CreateTargetPath", stagePath).Return(nil).Times(1)
			scImplMock.On("Mount", device, stagePath, []string{""}).Return(errors.New("error")).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeStageRequest(testV1ID, *volumeCap)

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to stage volume"))
		})
		It("Should fail, because Volume has failed status", func() {
			req := getNodeStageRequest(testV1ID, *volumeCap)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.Failed
			err := node.k8sclient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
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

			req := getNodeUnpublishRequest(testV1ID, targetPath)

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
		It("Should succeeded, because Volume has more than 1 owners", func() {
			req := getNodeUnpublishRequest(testV1ID, targetPath)
			scImplMock.On("Unmount", targetPath).Return(nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock

			vol1 := testVolumeCR1
			vol1.Spec.Owners = []string{"pod-1", "pod-2"}
			err := node.k8sclient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})

	})

	Context("NodeUnPublish() failure", func() {
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodeUnpublishVolumeRequest{
				TargetPath: targetPath,
			}

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodeUnpublishVolumeRequest{
				VolumeId: testV1ID,
			}

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})

		It("Should fail with Unmount() error", func() {
			scImplMock.On("Unmount", targetPath).Return(errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeUnpublishRequest(testV1ID, targetPath)

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Unable to unmount"))
		})
		It("Should failed, because Volume has failed status", func() {
			req := getNodeUnpublishRequest(testV1ID, targetPath)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.Failed
			err := node.k8sclient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
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

			req := getNodeUnstageRequest(testV1ID, stagePath)

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})
	})

	Context("NodeUnPublish() failure", func() {
		It("Should fail with missing VolumeId", func() {
			req := &csi.NodeUnstageVolumeRequest{
				StagingTargetPath: stagePath,
			}

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})

		It("Should fail with missing target path", func() {
			req := &csi.NodeUnstageVolumeRequest{
				VolumeId: testV1ID,
			}

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Stage Path missing in request"))
		})

		It("Should fail with Unmount() error", func() {
			scImplMock.On("Unmount", targetPath).Return(errors.New("error")).Times(1)

			node.scMap[SCName("hdd")] = scImplMock
			req := getNodeUnstageRequest(testV1ID, targetPath)

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Unable to unmount"))
		})

		It("Should failed, because Volume has failed status", func() {
			req := getNodeUnstageRequest(testV1ID, targetPath)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.Failed
			err := node.k8sclient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
		})
	})
})

var _ = Describe("CSINodeService NodeGetInfo()", func() {
	It("Should return topology key with Node ID", func() {
		node := newNodeService()

		resp, err := node.NodeGetInfo(testCtx, &csi.NodeGetInfoRequest{})
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

		resp, err := node.NodeGetCapabilities(testCtx, &csi.NodeGetCapabilitiesRequest{})
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

var _ = Describe("CSINodeService Check()", func() {
	It("Should return serving", func() {
		node := newNodeService()
		node.initialized = true

		resp, err := node.Check(testCtx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		Expect(resp.Status).To(Equal(grpc_health_v1.HealthCheckResponse_SERVING))
	})
	It("Should return not serving", func() {
		node := newNodeService()

		resp, err := node.Check(testCtx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		Expect(resp.Status).To(Equal(grpc_health_v1.HealthCheckResponse_NOT_SERVING))
	})
})

var _ = Describe("CSINodeService InlineVolumes", func() {
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

	Context("Volume Context with inline volumes", func() {
		It("Fail to parse volume context", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			req.StagingTargetPath = ""
			req.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true1"
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("Should create inline volume", func() {
			scImplMock.On("CreateTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("Mount", device, targetPath, []string{""}).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			req.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true"
			req.VolumeContext["size"] = "50Gi"
			err := testutils.AddAC(node.k8sclient, &testAC1, &testAC2)
			Expect(err).To(BeNil())
			go testutils.VolumeReconcileImitation(node.svc, testV1ID, apiV1.Created)
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})

		It("Should fail to create inline volume", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			req.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true"
			req.VolumeContext["size"] = "50Gi"
			go testutils.VolumeReconcileImitation(node.svc, testV1ID, apiV1.Failed)
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("Should fail with missing size", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			req.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true"
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("Missing Stage path", func() {
			scImplMock.On("CreateTargetPath", targetPath).Return(nil).Times(1)
			scImplMock.On("Mount", device, targetPath, []string{""}).Return(nil).Times(1)
			scImplMock.On("IsMountPoint", targetPath).Return(false, nil).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			req.StagingTargetPath = ""
			req.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true"
			req.VolumeContext["size"] = "50Gi"
			err := testutils.AddAC(node.k8sclient, &testAC1, &testAC2)
			Expect(err).To(BeNil())
			go testutils.VolumeReconcileImitation(node.svc, testV1ID, apiV1.Created)
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
		})

		It("Should fail publish inline volume", func() {
			scImplMock.On("CreateTargetPath", targetPath).Return(fmt.Errorf("error")).Times(1)
			node.scMap[SCName("hdd")] = scImplMock
			req := getNodePublishRequest(testV1ID, targetPath, *volumeCap)
			req.VolumeContext["csi.storage.k8s.io/ephemeral"] = "true"
			req.VolumeContext["size"] = "50Gi"
			err := testutils.AddAC(node.k8sclient, &testAC1, &testAC2)
			Expect(err).To(BeNil())
			go testutils.VolumeReconcileImitation(node.svc, testV1ID, apiV1.Created)
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
	})
})

func getNodePublishRequest(volumeID, targetPath string, volumeCap csi.VolumeCapability) *csi.NodePublishVolumeRequest {
	return &csi.NodePublishVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stagePath,
		TargetPath:        targetPath,
		VolumeCapability:  &volumeCap,
		VolumeContext:     make(map[string]string),
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
	// todo get rid of code duplicates
	// create map of commands which must be mocked
	cmds := make(map[string]mocks.CmdOut)
	// list of all devices
	cmds[lsblkAllDevicesCmd] = mocks.CmdOut{Stdout: mocks.LsblkTwoDevicesStr}
	// list partitions of specific device
	cmds[lsblkSingleDeviceCmd] = mocks.CmdOut{Stdout: mocks.LsblkListPartitionsStr}
	partUUID := fmt.Sprintf(fmt.Sprintf(ph.GetPartitionUUIDCmdTmpl, "/dev/sda", "1"))
	cmds[partUUID] = mocks.CmdOut{Stdout: "Partition unique GUID: volume-id"}
	executor := mocks.NewMockExecutor(cmds)
	kubeClient, err := base.GetFakeKubeClient(testNs)
	if err != nil {
		panic(err)
	}
	node := NewCSINodeService(client, nodeID, logrus.New(), kubeClient)
	node.VolumeManager.SetExecutor(executor)
	node.linuxUtils = linuxutils.NewLinuxUtils(executor, node.log.Logger)

	node.drivesCache[disk1.UUID] = &drivecrd.Drive{
		TypeMeta: v1.TypeMeta{
			Kind:       "Drive",
			APIVersion: apiV1.APIV1Version,
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
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      disk2.UUID,
			Namespace: "default",
		},
		Spec: disk2,
	}

	addVolumeCRs(node.k8sclient, testVolumeCR1, testVolumeCR2, testVolumeCR3)

	return node
}

func addVolumeCRs(k8sClient *base.KubeClient, volumes ...vcrd.Volume) {
	for _, v := range volumes {
		if err := k8sClient.CreateCR(context.Background(), v.Name, &v); err != nil {
			panic(err)
		}
	}
}
