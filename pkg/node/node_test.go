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

package node

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	csibmnodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/csibmnode/common"
	"github.com/dell/csi-baremetal/pkg/mocks"
	mockProv "github.com/dell/csi-baremetal/pkg/mocks/provisioners"
	p "github.com/dell/csi-baremetal/pkg/node/provisioners"
	"github.com/dell/csi-baremetal/pkg/testutils"
)

var (
	node   *CSINodeService
	prov   *mockProv.MockProvisioner
	fsOps  *mockProv.MockFsOpts
	volOps *mocks.VolumeOperationsMock
)

func setVariables() {
	node = newNodeService()
	prov = &mockProv.MockProvisioner{}
	fsOps = &mockProv.MockFsOpts{}
	volOps = &mocks.VolumeOperationsMock{}
	node.provisioners = map[p.VolumeType]p.Provisioner{
		p.DriveBasedVolumeType: prov,
		p.LVMBasedVolumeType:   prov,
	}
	node.fsOps = fsOps
	node.svc = volOps
}

func TestCSINodeService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSIControllerService testing suite")
}

var _ = Describe("CSINodeService NodePublish()", func() {
	BeforeEach(func() {
		setVariables()
	})

	Context("NodePublish() success", func() {
		It("Should publish volume", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *testVolumeCap)
			req.VolumeContext[PodNameKey] = testPodName

			fsOps.On("PrepareAndPerformMount",
				req.GetStagingTargetPath(), req.GetTargetPath(), true).
				Return(nil)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())

			// check owner appearance
			volumeCR := &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, testV1ID, volumeCR)
			Expect(err).To(BeNil())
			//Expect(volumeCR.Spec.Owners[0]).To(Equal(testPodName))

			// publish again such volume
			resp, err = node.NodePublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())

			// check owner appearance
			volumeCR = &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, testV1ID, volumeCR)
			Expect(err).To(BeNil())
			//Expect(len(volumeCR.Spec.Owners)).To(Equal(1))
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
				VolumeCapability: testVolumeCap,
			}

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing target path", func() {
			req := &csi.NodePublishVolumeRequest{
				VolumeId:         testV1ID,
				VolumeCapability: testVolumeCap,
			}

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})
		It("Should fail with missing stage path", func() {
			req := &csi.NodePublishVolumeRequest{
				VolumeId:         testV1ID,
				VolumeCapability: testVolumeCap,
				TargetPath:       targetPath,
			}

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Staging Path missing in request"))
		})
		It("Should fail, because Volume has failed status", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *testVolumeCap)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.Failed
			err := node.k8sClient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("Should fail, because of volume CR isn't exist", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *testVolumeCap)
			err := node.k8sClient.DeleteCR(testCtx, &testVolumeCR1)
			Expect(err).To(BeNil())

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			// not a good ide to check error message. better to validate error code.
			Expect(err.Error()).To(ContainSubstring("Unable to find volume"))
		})
		It("Should fail, because of PrepareAndPerformMount failed", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *testVolumeCap)

			fsOps.On("PrepareAndPerformMount",
				req.GetStagingTargetPath(), req.GetTargetPath(), true).
				Return(errors.New("error mount"))

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("mount error"))
		})
	})
})

var _ = Describe("CSINodeService NodeStage()", func() {
	BeforeEach(func() {
		setVariables()
	})

	Context("NodeStage() success", func() {
		It("Should stage volume", func() {
			// testVolume2 has Create status
			req := getNodeStageRequest(testVolume2.Id, *testVolumeCap)
			partitionPath := "/partition/path/for/volume1"
			prov.On("GetVolumePath", testVolume2).Return(partitionPath, nil)
			fsOps.On("PrepareAndPerformMount",
				partitionPath, req.GetStagingTargetPath(), false).
				Return(nil)

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
			// check volume CR status
			volumeCR := &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, testVolume1.Id, volumeCR)
			Expect(err).To(BeNil())
			Expect(volumeCR.Spec.CSIStatus).To(Equal(apiV1.VolumeReady))
		})
		It("Should stage, volume CR with VolumeReady status", func() {
			req := getNodeStageRequest(testVolume1.Id, *testVolumeCap)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.VolumeReady
			err := node.k8sClient.UpdateCR(testCtx, &vol1)

			partitionPath := "/partition/path/for/volume1"
			prov.On("GetVolumePath", vol1.Spec).Return(partitionPath, nil)
			fsOps.On("PrepareAndPerformMount",
				partitionPath, req.GetStagingTargetPath(), false).
				Return(nil)

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
				VolumeCapability:  testVolumeCap,
			}

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume ID missing in request"))
		})
		It("Should fail with missing stage path", func() {
			req := &csi.NodeStageVolumeRequest{
				VolumeId:         testV1ID,
				VolumeCapability: testVolumeCap,
			}

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Stage Path missing in request"))
		})
		It("Should fail, because of volume CR isn't exist", func() {
			req := getNodeStageRequest(testV1ID, *testVolumeCap)
			err := node.k8sClient.DeleteCR(testCtx, &testVolumeCR1)
			Expect(err).To(BeNil())

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
		It("Should fail because partition path wasn't found", func() {
			req := getNodeStageRequest(testVolume1.Id, *testVolumeCap)
			prov.On("GetVolumePath", testVolume1).
				Return("", errors.New("GetVolumePath error"))

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("partition error"))
			Expect(status.Code(err)).To(Equal(codes.Internal))
		})
		It("Failed because PrepareAndPerformMount had failed", func() {
			req := getNodeStageRequest(testVolume2.Id, *testVolumeCap)
			partitionPath := "/partition/path/for/volume1"
			prov.On("GetVolumePath", testVolume2).Return(partitionPath, nil)
			fsOps.On("PrepareAndPerformMount",
				partitionPath, req.GetStagingTargetPath(), false).
				Return(errors.New("PrepareAndPerformMount error"))

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to stage volume"))
		})
		It("Should fail with Mount error, volume CR has VolumeReady status", func() {
			req := getNodeStageRequest(testVolume1.Id, *testVolumeCap)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.VolumeReady
			err := node.k8sClient.UpdateCR(testCtx, &vol1)

			partitionPath := "/partition/path/for/volume1"
			prov.On("GetVolumePath", vol1.Spec).Return(partitionPath, nil)
			fsOps.On("PrepareAndPerformMount",
				partitionPath, req.GetStagingTargetPath(), false).
				Return(errors.New("mount error"))

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("mount error"))
		})
		It("Should fail, because Volume has failed status", func() {
			req := getNodeStageRequest(testV1ID, *testVolumeCap)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.Failed
			err := node.k8sClient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodeStageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
	})
})

var _ = Describe("CSINodeService NodeUnPublish()", func() {
	BeforeEach(func() {
		setVariables()
	})

	Context("NodeUnPublish() success", func() {
		It("Should unpublish volume and change volume CR status", func() {
			req := getNodeUnpublishRequest(testV1ID, targetPath)
			fsOps.On("UnmountWithCheck", req.GetTargetPath()).Return(nil)

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
			// check volume CR status
			volumeCR := &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, testV1ID, volumeCR)
			Expect(err).To(BeNil())
			Expect(volumeCR.Spec.CSIStatus).To(Equal(apiV1.VolumeReady))
		})
		//It("Should unpublish volume and don't change volume CR status", func() {
		//	req := getNodeUnpublishRequest(testV1ID, targetPath)
		//	vol1 := testVolumeCR1
		//	vol1.Spec.Owners = []string{"pod-1", "pod-2"}
		//	vol1.Spec.CSIStatus = apiV1.Published
		//	err := node.k8sClient.UpdateCR(testCtx, &vol1)
		//	Expect(err).To(BeNil())
		//	fsOps.On("UnmountWithCheck", req.GetTargetPath()).Return(nil)
		//
		//	resp, err := node.NodeUnpublishVolume(testCtx, req)
		//	Expect(resp).NotTo(BeNil())
		//	Expect(err).To(BeNil())
		//	// check volume CR status
		//	volumeCR := &vcrd.Volume{}
		//	err = node.k8sClient.ReadCR(testCtx, testV1ID, volumeCR)
		//	Expect(err).To(BeNil())
		//	Expect(volumeCR.Spec.CSIStatus).To(Equal(apiV1.Published))
		//})

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
			req := &csi.NodeUnpublishVolumeRequest{VolumeId: testV1ID}

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("Target Path missing in request"))
		})

		It("Should fail with UnmountWithCheck() error", func() {
			req := getNodeUnpublishRequest(testV1ID, targetPath)
			fsOps.On("UnmountWithCheck", req.GetTargetPath()).
				Return(errors.New("Unmount error"))

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("unmount error"))
		})
		It("Should failed, because Volume CR wasn't found", func() {
			req := getNodeUnpublishRequest("unexisted-volume", targetPath)

			resp, err := node.NodeUnpublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
	})
})
var _ = Describe("CSINodeService NodeUnStage()", func() {
	BeforeEach(func() {
		setVariables()
	})

	Context("NodeUnStage() success", func() {
		It("Should unstage volume", func() {
			req := getNodeUnstageRequest(testV1ID, stagePath)
			fsOps.On("UnmountWithCheck", req.GetStagingTargetPath()).Return(nil)

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
			// check owners and CSI status
			volumeCR := &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, testV1ID, volumeCR)
			Expect(err).To(BeNil())
			//Expect(volumeCR.Spec.Owners).To(BeNil())
			Expect(volumeCR.Spec.CSIStatus).To(Equal(apiV1.Created))
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
		It("Should fail because Volume CR wasn't found", func() {
			req := getNodeUnstageRequest("sone-none-existing-UUID", stagePath)
			resp, err := node.NodeUnstageVolume(testCtx, req)

			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
		})
		It("Should fail with UnmountWithCheck() error", func() {
			req := getNodeUnstageRequest(testV1ID, stagePath)
			fsOps.On("UnmountWithCheck", req.GetStagingTargetPath()).
				Return(errors.New("error"))

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			// check owners and CSI status
			volumeCR := &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, testV1ID, volumeCR)
			Expect(err).To(BeNil())
			//Expect(volumeCR.Spec.Owners).To(BeNil())
			Expect(volumeCR.Spec.CSIStatus).To(Equal(apiV1.Failed))
		})

		It("Should failed, because Volume has failed status", func() {
			req := getNodeUnstageRequest(testV1ID, targetPath)
			vol1 := testVolumeCR1
			vol1.Spec.CSIStatus = apiV1.Failed
			err := node.k8sClient.UpdateCR(testCtx, &vol1)
			Expect(err).To(BeNil())

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(status.Code(err)).To(Equal(codes.FailedPrecondition))
		})
	})

	Context("NodeUnStage() concurrent requests", func() {
		It("Should unstage volume one time", func() {
			req := getNodeUnstageRequest(testV1ID, stagePath)
			secondUnstageErr := make(chan error)
			// UnmountWithCheck should only once respond with no error
			fsOps.On("UnmountWithCheck", req.GetStagingTargetPath()).Return(nil).Run(func(_ mock.Arguments) {
				go func() {
					_, err := node.NodeUnstageVolume(testCtx, req)
					secondUnstageErr <- err
				}()
				// make call blocking call
				time.Sleep(10 * time.Millisecond)
			}).Once()
			// on later calls it will respond error
			fsOps.On("UnmountWithCheck", req.GetStagingTargetPath()).
				Return(fmt.Errorf("%s not mounted", req.GetStagingTargetPath()))

			resp, err := node.NodeUnstageVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())

			// check concurrent call error
			err = <-secondUnstageErr
			Expect(err).To(BeNil())

			// check owners and CSI status
			volumeCR := &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, testV1ID, volumeCR)
			Expect(err).To(BeNil())
			//Expect(volumeCR.Spec.Owners).To(BeNil())
			Expect(volumeCR.Spec.CSIStatus).To(Equal(apiV1.Created))
		})
	})
})
var _ = Describe("CSINodeService NodeGetInfo()", func() {
	It("Should return topology key with Node ID", func() {
		node := newNodeService()

		resp, err := node.NodeGetInfo(testCtx, &csi.NodeGetInfoRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		val, ok := resp.AccessibleTopology.Segments[csibmnodeconst.NodeIDAnnotationKey]
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
	BeforeEach(func() {
		setVariables()
	})

	Context("Volume Context with inline volumes", func() {
		It("Fail to parse volume context", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *testVolumeCap)
			req.StagingTargetPath = ""
			req.VolumeContext[EphemeralKey] = "true1"
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("Should create inline volume", func() {
			req := getNodePublishRequest(testVolume1.Id, targetPath, *testVolumeCap)
			req.VolumeContext[EphemeralKey] = "true"
			req.VolumeContext[base.SizeKey] = "50Gi"
			req.VolumeContext[PodNameKey] = testPodName
			err := testutils.AddAC(node.k8sClient, &testAC1, &testAC2)
			Expect(err).To(BeNil())

			var (
				createdVolCR = testVolumeCR1
				srcPath      = "/some/path"
			)

			createdVolCR.Spec.CSIStatus = apiV1.Created
			err = node.k8sClient.UpdateCR(testCtx, &createdVolCR)
			Expect(err).To(BeNil())

			volOps.On("CreateVolume", mock.Anything, mock.Anything).Return(&createdVolCR.Spec, nil)
			prov.On("GetVolumePath", createdVolCR.Spec).Return(srcPath, nil)
			fsOps.On("PrepareAndPerformMount", srcPath, req.GetTargetPath(), false).Return(nil)

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
			// check volume CR status and owners
			volumeCR := &vcrd.Volume{}
			err = node.k8sClient.ReadCR(testCtx, createdVolCR.Name, volumeCR)
			Expect(err).To(BeNil())

			Expect(volumeCR.Spec.CSIStatus).To(Equal(apiV1.Published))
			//Expect(volumeCR.Spec.Owners[0]).To(Equal(testPodName))
		})
		It("Should fail to create inline volume in CreateVolume step", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *testVolumeCap)
			req.VolumeContext[EphemeralKey] = "true"
			req.VolumeContext[base.SizeKey] = "50Gi"

			var emptyVol *api.Volume
			volOps.On("CreateVolume", mock.Anything, mock.Anything).
				Return(emptyVol, errors.New("error"))

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
		It("Should fail to create inline volume in GetVolumePath step", func() {
			req := getNodePublishRequest(testVolume1.Id, targetPath, *testVolumeCap)
			req.VolumeContext[EphemeralKey] = "true"
			req.VolumeContext[base.SizeKey] = "50Gi"
			req.VolumeContext[PodNameKey] = testPodName
			err := testutils.AddAC(node.k8sClient, &testAC1, &testAC2)
			Expect(err).To(BeNil())

			var (
				createdVolCR = testVolumeCR1
				srcPath      = "/some/path"
			)

			createdVolCR.Spec.CSIStatus = apiV1.Created
			err = node.k8sClient.UpdateCR(testCtx, &createdVolCR)
			Expect(err).To(BeNil())

			volOps.On("CreateVolume", mock.Anything, mock.Anything).Return(&createdVolCR.Spec, nil)
			prov.On("GetVolumePath", createdVolCR.Spec).Return(srcPath, errors.New("error"))

			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})

		It("Should fail with missing size", func() {
			req := getNodePublishRequest(testV1ID, targetPath, *testVolumeCap)
			req.VolumeContext[EphemeralKey] = "true"
			resp, err := node.NodePublishVolume(testCtx, req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
		})
	})
})

var _ = Describe("CSINodeService Probe()", func() {
	It("Should success", func() {
		node := newNodeService()
		node.initialized = true

		resp, err := node.Probe(testCtx, &csi.ProbeRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		Expect(resp.Ready.Value).To(Equal(true))
	})
	It("Should failed", func() {
		node := newNodeService()
		node.livenessCheck = &DummyLivenessHelper{false}
		resp, err := node.Probe(testCtx, &csi.ProbeRequest{})
		Expect(err).To(BeNil())
		Expect(resp).ToNot(BeNil())
		Expect(resp.Ready.Value).To(Equal(false))
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
	client := mocks.NewMockDriveMgrClient(mocks.DriveMgrRespDrives)
	kubeClient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	node := NewCSINodeService(client, nodeID, testLogger, kubeClient,
		new(mocks.NoOpRecorder), featureconfig.NewFeatureConfig())

	driveCR1 := node.k8sClient.ConstructDriveCR(disk1.UUID, disk1)
	driveCR2 := node.k8sClient.ConstructDriveCR(disk2.UUID, disk2)
	addDriveCRs(node.k8sClient, driveCR1, driveCR2)
	addVolumeCRs(node.k8sClient, testVolumeCR1, testVolumeCR2, testVolumeCR3)

	return node
}

func addVolumeCRs(k8sClient *k8s.KubeClient, volumes ...vcrd.Volume) {
	for _, v := range volumes {
		if err := k8sClient.CreateCR(context.Background(), v.Name, &v); err != nil {
			panic(err)
		}
	}
}
