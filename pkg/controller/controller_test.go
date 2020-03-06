package controller

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	coreV1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testID = "someID"
	testNs = "default"

	testCtx       = context.Background()
	testPod1Name  = fmt.Sprintf("%s-testPod1", NodeSvcPodsMask)
	testPod2Name  = fmt.Sprintf("%s-testPod2", NodeSvcPodsMask)
	testPod3Name  = fmt.Sprintf("%s-testPod3", NodeSvcPodsMask)
	testPod4Name  = "SomeName"
	testPod1Ip    = "10.10.10.10"
	testPod2Ip    = "10.10.10.11"
	testPod3Ip    = "NOT AN IP"
	testNode1Name = "node1"
	testNode2Name = "node2"
	testNode3Name = "node3"

	testDriveLocation1 = "drive1-sn"
	testDriveLocation2 = "drive2-sn"
	testDriveLocation3 = "drive3-sn"
	testNode4Name      = "preferredNode"
	// valid pod
	testPod1 = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod1Name, Namespace: testNs},
		Spec:       coreV1.PodSpec{NodeName: testNode1Name},
		Status:     coreV1.PodStatus{PodIP: testPod1Ip},
	}
	// valid pod
	testPod2 = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod2Name, Namespace: testNs},
		Spec:       coreV1.PodSpec{NodeName: testNode2Name},
		Status:     coreV1.PodStatus{PodIP: testPod2Ip},
	}
	// invalid pod, bad endpoint
	testPod3 = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod3Name, Namespace: testNs},
		Spec:       coreV1.PodSpec{NodeName: testNode3Name},
		Status:     coreV1.PodStatus{PodIP: testPod3Ip},
	}
	// invalid pod, bad testID
	testPod4 = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod4Name},
	}

	testVolume = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: "volume.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testID,
			Namespace:         testNs,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:       testID,
			Owner:    "pod",
			Size:     1000,
			Type:     "Type",
			Location: "location",
		},
	}

	testAC1Name = fmt.Sprintf("%s-%s", testNode1Name, strings.ToLower(testDriveLocation1))
	testAC1     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC1Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation1,
			NodeId:   testNode1Name},
	}
	testAC2Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation2))
	testAC2     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC2Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024 * 1024,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation2,
			NodeId:   testNode2Name,
		},
	}
	testAC3Name = fmt.Sprintf("%s-%s", testNode1Name, strings.ToLower(testDriveLocation3))
	testAC3     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC3Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024 * 100,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation3,
			NodeId:   testNode1Name,
		},
	}
)

func TestCSIControllerService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSIControllerService testing suite")
}

var _ = Describe("CSIControllerService addition functions", func() {
	var svc *CSIControllerService

	BeforeEach(func() {
		svc = newSvc()
	})

	AfterEach(func() {
		removeAllPods(svc)
		removeAllCrds(svc.k8sclient)
	})

	Context("construct CR scenarios", func() {
		It("Construct AvailableCapacity CRD instance", func() {
			capacity := api.AvailableCapacity{
				Size:     1024 * 1024,
				Type:     api.StorageClass_HDD,
				Location: testDriveLocation1,
				NodeId:   testNode1Name,
			}
			ac := svc.constructAvailableCapacityCR(testAC1Name, &capacity)
			Expect(ac).To(Equal(&testAC1))
		})
	})
	Context("updateCommunicator success scenarios", func() {
		It("updateCommunicator Success", func() {
			createPods(svc, testPod1)
			err := svc.updateCommunicators()
			Expect(err).To(BeNil())
			Expect(len(svc.communicators)).To(Equal(1))
		})
		It("create 3 pods and expect 2 communicators, 1 pod has incompatible testID", func() {
			createPods(svc, testPod1, testPod2, testPod4)
			err := svc.updateCommunicators()
			Expect(err).To(BeNil())
			Expect(len(svc.communicators)).To(Equal(2))
		})
		It("create 3 pods and expect 2 communicators, 1 pod has incompatible pod ip", func() {
			createPods(svc, testPod1, testPod2, testPod3)
			err := svc.updateCommunicators()
			Expect(err).To(BeNil())
			Expect(len(svc.communicators)).To(Equal(2))
		})
	})

	Context("updateCommunicator fail scenarios", func() {
		It("0 communicators were created", func() {
			createPods(svc, testPod4)
			svc.communicators = map[NodeID]api.VolumeManagerClient{}
			err := svc.updateCommunicators()
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(errors.New("unable to initialize communicators")))
		})
	})

	Context("searchAvailableCapacity scenarios", func() {
		It("Found AC with min size on preferred node", func() {
			addAC(svc, &testAC1, &testAC3)
			requiredCapacity := int64(900)
			drive := svc.searchAvailableCapacity(testNode1Name, requiredCapacity, "")
			Expect(testAC1.Spec.Location).To(Equal(drive.Spec.Location))
		})
		It("Found AC on node with maximum ACs (preferred node wasn't provided", func() {
			addAC(svc, &testAC1, &testAC2, &testAC3)    // 2 ACs on node1 and 1 AC on node 3
			requiredCapacity := int64(1024 * 1024 * 50) // expect testAC3
			ac := svc.searchAvailableCapacity("", requiredCapacity, "")
			Expect(ac).ToNot(BeNil())
			Expect(ac.Spec.Location).To(Equal(testAC3.Spec.Location))
		})
		It("Found AC on preferred node with defined storage class", func() {
			addAC(svc, &testAC2)
			requiredCapacity := int64(2000)
			ac := svc.searchAvailableCapacity(testNode2Name, requiredCapacity, "hdd")
			Expect(testAC2.Spec.Location).To(Equal(ac.Spec.Location))
		})
		It("Couldn't find any ac because of requiredCapacity", func() {
			addAC(svc, &testAC1, &testAC2)
			drive := svc.searchAvailableCapacity(testNode1Name, 1024*1024*2048, "")
			Expect(drive).To(BeNil())
		})
		It("Couldn't find any ac because of non-existed preferred node", func() {
			addAC(svc, &testAC1, &testAC2)
			drive := svc.searchAvailableCapacity("node", 1024, "")
			Expect(drive).To(BeNil())
		})
	})

	Context("updateAvailableCapacityCRs scenarios", func() {
		It("Failed to create ac because of GetAvailableCapacity request error", func() {
			mc := &mocks.VolumeMgrClientMock{}
			mc.On("GetAvailableCapacity", &api.AvailableCapacityRequest{
				// Prepare response from NodeService
				NodeId: testNode1Name,
			}).Return(&api.AvailableCapacityResponse{}, errors.New("error during GetAvailableCapacity request"))
			svc.communicators[NodeID(testNode1Name)] = mc
			err := svc.updateAvailableCapacityCRs(context.Background())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not all available capacity were created"))
		})
		It("There are no AC on node, AC CRs shouldn't be created", func() {
			mc := &mocks.VolumeMgrClientMock{}
			response := &api.AvailableCapacityResponse{
				AvailableCapacity: make([]*api.AvailableCapacity, 0),
			}
			mc.On("GetAvailableCapacity", &api.AvailableCapacityRequest{
				// Prepare response from NodeService
				NodeId: testNode4Name,
			}).Return(response, nil)
			svc.communicators[NodeID(testNode4Name)] = mc
			err := svc.updateAvailableCapacityCRs(context.Background())
			Expect(err).To(BeNil())
			acList := accrd.AvailableCapacityList{}
			err = svc.k8sclient.ReadList(context.Background(), &acList)
			Expect(err).To(BeNil())
			Expect(len(acList.Items)).To(Equal(0))
		})
		It("Create one AC CR", func() {
			mc := &mocks.VolumeMgrClientMock{}
			availableCapacity := make([]*api.AvailableCapacity, 0)
			availableCapacity = append(availableCapacity, &api.AvailableCapacity{
				Size:     1000,
				Type:     api.StorageClass_ANY,
				Location: "drive",
				NodeId:   testNode4Name,
			})
			response := &api.AvailableCapacityResponse{
				AvailableCapacity: availableCapacity,
			}
			mc.On("GetAvailableCapacity", &api.AvailableCapacityRequest{
				// Prepare response from NodeService
				NodeId: testNode4Name,
			}).Return(response, nil)
			svc.communicators[NodeID(testNode4Name)] = mc
			err := svc.updateAvailableCapacityCRs(context.Background())
			Expect(err).To(BeNil())
			acList := accrd.AvailableCapacityList{}
			err = svc.k8sclient.ReadList(context.Background(), &acList)
			Expect(err).To(BeNil())
			Expect(len(acList.Items)).To(Equal(1))
		})
	})

	Context("waitVCRStatus scenarios", func() {
		BeforeEach(func() {
			err := svc.k8sclient.CreateCR(context.Background(), &testVolume, testID)
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			err := svc.k8sclient.DeleteCR(context.Background(), &testVolume)
			Expect(err).To(BeNil())
		})

		It("Context was closed", func() {
			ctxT, cancelFn := context.WithTimeout(context.Background(), 10*time.Millisecond)
			reached, statusCode := svc.waitVCRStatus(ctxT, testID)
			cancelFn()
			Expect(reached).To(BeFalse())
			Expect(statusCode).To(Equal(api.OperationalStatus(-1)))
		})
		It("Status was reached", func() {
			ctxT, cancelFn := context.WithTimeout(context.Background(), 1200*time.Millisecond)
			var (
				wg         sync.WaitGroup
				reached    bool
				statusCode api.OperationalStatus
			)
			wg.Add(1)
			go func() {
				reached, statusCode = svc.waitVCRStatus(ctxT, testID, api.OperationalStatus_FailedToCreate)
				cancelFn()
				wg.Done()
			}()
			testV2 := testVolume
			testV2.Spec.Status = api.OperationalStatus_FailedToCreate
			err := svc.k8sclient.UpdateCR(context.Background(), &testV2)
			Expect(err).To(BeNil())
			wg.Wait()
			Expect(reached).To(BeTrue())
			Expect(statusCode).To(Equal(api.OperationalStatus(11))) // 11 - OperationalStatus_FailedToCreate
		})
	})
})

var _ = Describe("CSIControllerService CreateVolume", func() {
	var svc *CSIControllerService

	BeforeEach(func() {
		svc = newSvc()
	})

	Context("Fail scenarios", func() {
		It("Missing request name", func() {
			req := &csi.CreateVolumeRequest{}
			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume name missing in request"))
		})
		It("Missing volume capabilities", func() {
			req := &csi.CreateVolumeRequest{Name: "some-name-1"}
			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume capabilities missing in request"))
		})
		It("There is no suitable Available Capacity (on all nodes)", func() {
			req := getCreateVolumeRequest("req1", 1024*1024*1024*1024, "")

			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("there is no suitable drive for request"))
		})
		It("There is no suitable Available Capacity (on preferred node)", func() {
			req := getCreateVolumeRequest("req1", 1024*1024*1024*1024, "node1")

			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("there is no suitable drive for request"))
		})
		It("Status FailedToCreate had reached because of createLocalVolume had got error", func() {
			addAC(svc, &testAC1, &testAC2)
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name)
				mc       = &mocks.VolumeMgrClientMock{}
				vol      = &vcrd.Volume{}
			)
			svc.communicators[NodeID(testNode1Name)] = mc
			mc.On("CreateLocalVolume", &api.CreateLocalVolumeRequest{
				PvcUUID:  "req1",
				Capacity: capacity,
				Sc:       api.StorageClass_HDD,
				Location: testDriveLocation1,
			}).Return(&api.CreateLocalVolumeResponse{}, errors.New("error"))
			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "Unable to create volume on local node.")))
			err = svc.k8sclient.ReadCR(context.Background(), "req1", vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.Status).To(Equal(api.OperationalStatus_FailedToCreate))
		})
		It("Volume CR creation timeout expired", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)

			req := getCreateVolumeRequest(uuid, capacity, testNode4Name)
			mc := &mocks.VolumeMgrClientMock{}
			svc.communicators[NodeID(testNode1Name)] = mc

			err := svc.k8sclient.CreateCR(context.Background(), &vcrd.Volume{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name:              uuid,
					Namespace:         "default",
					CreationTimestamp: k8smetav1.Time{Time: time.Now().Add(time.Duration(-100) * time.Minute)},
				},
				Spec: api.Volume{
					Id:     req.GetName(),
					Size:   1024 * 60,
					Owner:  testNode1Name,
					Status: api.OperationalStatus_Creating,
				}}, req.GetName())
			Expect(err).To(BeNil())

			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			v := vcrd.Volume{}
			err = svc.k8sclient.ReadCR(testCtx, req.GetName(), &v)
			Expect(err).To(BeNil())
			Expect(v.Spec.Status).To(Equal(api.OperationalStatus_FailedToCreate))
		})
	})

	Context("Success scenarios", func() {
		It("Volume is created successfully", func() {
			addAC(svc, &testAC1, &testAC2)
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name)
				mc       = &mocks.VolumeMgrClientMock{}
				vol      = &vcrd.Volume{}
			)
			svc.communicators[NodeID(testNode1Name)] = mc
			mc.On("CreateLocalVolume", &api.CreateLocalVolumeRequest{
				PvcUUID:  "req1",
				Capacity: capacity,
				Sc:       api.StorageClass_HDD,
				Location: testDriveLocation1,
			}).Return(&api.CreateLocalVolumeResponse{Ok: true}, nil)
			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())

			err = svc.k8sclient.ReadCR(context.Background(), "req1", vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.Status).To(Equal(api.OperationalStatus_Created))
		})
		It("Volume CR has already existed", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)

			req := getCreateVolumeRequest(uuid, capacity, testNode4Name)
			mc := &mocks.VolumeMgrClientMock{}
			svc.communicators[NodeID(testNode1Name)] = mc

			err := svc.k8sclient.CreateCR(context.Background(), &vcrd.Volume{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name:              uuid,
					Namespace:         "default",
					CreationTimestamp: k8smetav1.Time{Time: time.Now()},
				},
				Spec: api.Volume{
					Id:     req.GetName(),
					Size:   1024 * 60,
					Owner:  testNode1Name,
					Status: api.OperationalStatus_Created,
				}}, req.GetName())
			Expect(err).To(BeNil())

			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
			Expect(resp.Volume.VolumeId).To(Equal(uuid))
			Expect(resp.Volume.CapacityBytes).To(Equal(int64(1024 * 60)))
		})
	})
})

var _ = Describe("CSIControllerService DeleteVolume", func() {
	var (
		svc  *CSIControllerService
		node = "node1"
		uuid = "uuid-1234"
	)

	BeforeEach(func() {
		svc = newSvc()
		// prepare crd
		err := svc.k8sclient.CreateCR(context.Background(), &vcrd.Volume{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      uuid,
				Namespace: "default",
			},
			Spec: api.Volume{
				Id:    uuid,
				Owner: node,
			}}, uuid)
		Expect(err).To(BeNil())
	})

	Context("Fail scenarios", func() {

		It("Request doesn't contain volume ID", func() {
			dreq := &csi.DeleteVolumeRequest{}
			resp, err := svc.DeleteVolume(context.Background(), dreq)
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.InvalidArgument, "Volume ID must be provided")))
		})

		It("Communicator on DeleteLocalVolume returns error or false", func() {
			mc := &mocks.VolumeMgrClientMock{}
			// prepare communicator
			svc.communicators[NodeID(node)] = mc
			dlReq := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}
			dlResp := &api.DeleteLocalVolumeResponse{Ok: false}
			mc.On("DeleteLocalVolume", dlReq).Return(dlResp, errors.New("error")).Times(1)

			resp, err := svc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Errorf(codes.Internal, "unable to delete volume on node %s", node)))

			// second time DeleteLocalVolume will return error nil, but ok is false
			mc.On("DeleteLocalVolume", dlReq).Return(dlResp, nil).Times(1)
			resp, err = svc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "response for delete local volume is not ok")))
		})

		It("DeleteLocalVolume doesn't return local volume", func() {
			mc := &mocks.VolumeMgrClientMock{}
			// prepare communicator
			svc.communicators[NodeID(node)] = mc
			dlReq := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}

			dlResp := &api.DeleteLocalVolumeResponse{Ok: true, Volume: nil}
			mc.On("DeleteLocalVolume", dlReq).Return(dlResp, nil).Times(1)

			resp, err := svc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "Unable to delete volume from node")))
		})
	})

	Context("Success scenarios", func() {
		It("Volume CRD isn't found, consider that volume was removed", func() {
			vID := "some-id"
			dreq := &csi.DeleteVolumeRequest{VolumeId: vID}
			resp, err := svc.DeleteVolume(context.Background(), dreq)
			Expect(resp).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
		It("Volume was delete successful", func() {
			mc := &mocks.VolumeMgrClientMock{}
			// prepare communicator
			svc.communicators[NodeID(node)] = mc
			dlReq := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}

			localVolume := api.Volume{
				Id:       uuid,
				Owner:    node,
				Location: testDriveLocation1,
			}

			dlResp := &api.DeleteLocalVolumeResponse{Ok: true, Volume: &localVolume}
			mc.On("DeleteLocalVolume", dlReq).Return(dlResp, nil).Times(1)

			// create volume crd to delete
			volumeCrd := &vcrd.Volume{
				TypeMeta: k8smetav1.TypeMeta{
					Kind:       "Volume",
					APIVersion: "volume.dell.com/v1",
				},
				ObjectMeta: k8smetav1.ObjectMeta{
					Name:      localVolume.Id,
					Namespace: svc.k8sclient.Namespace,
				},
				Spec: localVolume,
			}
			_ = svc.k8sclient.CreateCR(testCtx, volumeCrd, uuid)

			resp, err := svc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())
			acList := accrd.AvailableCapacityList{}
			err = svc.k8sclient.ReadList(context.Background(), &acList)
			Expect(err).To(BeNil())
			Expect(len(acList.Items)).To(Equal(1)) // expect that one AC will appear
		})
	})
})

var _ = Describe("CSIControllerService ControllerGetCapabilities", func() {
	It("Should return right capabilities", func() {
		var (
			caps                      *csi.ControllerGetCapabilitiesResponse
			err                       error
			expectedCapabilitiesTypes = []csi.ControllerServiceCapability_RPC_Type{
				csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
				csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
			}
		)

		svc := newSvc()

		caps, err = svc.ControllerGetCapabilities(context.Background(), &csi.ControllerGetCapabilitiesRequest{})
		Expect(err).To(BeNil())
		Expect(len(caps.Capabilities)).To(Equal(2))

		currentCapabilitiesTypes := make([]csi.ControllerServiceCapability_RPC_Type, len(caps.Capabilities))
		for i := 0; i < len(caps.Capabilities); i++ {
			currentCapabilitiesTypes[i] = caps.Capabilities[i].GetRpc().GetType()
		}
		Expect(expectedCapabilitiesTypes).To(ConsistOf(currentCapabilitiesTypes))
	})
})

// create provided pods via client from provided svc
func createPods(s *CSIControllerService, pods ...*coreV1.Pod) {
	for _, pod := range pods {
		err := s.k8sclient.Create(context.Background(), pod)
		if err != nil {
			Fail(fmt.Sprintf("uable to create pod %s, error: %v", pod.Name, err))
		}
	}
}

// add available capacity to svc cache
func addAC(s *CSIControllerService, acs ...*accrd.AvailableCapacity) {
	for _, ac := range acs {
		if err := s.k8sclient.CreateCR(context.Background(), ac, ac.Name); err != nil {
			Fail(fmt.Sprintf("uable to create ac %s, error: %v", ac.Name, err))
		}
	}
}

// create and instance of CSIControllerService with scheme for working with CRD
func newSvc() *CSIControllerService {
	kubeclient, err := base.GetFakeKubeClient(testNs)
	if err != nil {
		panic(err)
	}
	nSvc := NewControllerService(kubeclient, logrus.New())
	return nSvc
}

// remove all pods via client from provided svc
func removeAllPods(s *CSIControllerService) {
	pods := coreV1.PodList{}
	err := s.k8sclient.List(context.Background(), &pods, k8sclient.InNamespace(testNs))
	if err != nil {
		Fail(fmt.Sprintf("unable to get pods list: %v", err))
	}
	for _, pod := range pods.Items {
		err = s.k8sclient.Delete(context.Background(), &pod)
		if err != nil {
			Fail(fmt.Sprintf("unable to delete pod: %v", err))
		}
	}
}

// return CreateVolumeRequest based on provided parameters
func getCreateVolumeRequest(name string, cap int64, preferredNode string) *csi.CreateVolumeRequest {
	req := &csi.CreateVolumeRequest{
		Name:               name,
		CapacityRange:      &csi.CapacityRange{RequiredBytes: cap},
		VolumeCapabilities: make([]*csi.VolumeCapability, 0),
	}

	if preferredNode != "" {
		req.AccessibilityRequirements = &csi.TopologyRequirement{
			Preferred: []*csi.Topology{
				{
					Segments: map[string]string{"baremetal-csi/nodeid": preferredNode},
				},
			},
		}
	}
	return req
}

// remove all crds (volume and ac)
func removeAllCrds(s *base.KubeClient) {
	var (
		vList  = &vcrd.VolumeList{}
		acList = &accrd.AvailableCapacityList{}
		err    error
	)

	if err = s.ReadList(testCtx, vList); err != nil {
		Fail(fmt.Sprintf("unable to read volume crds list: %v", err))
	}

	if err = s.ReadList(testCtx, acList); err != nil {
		Fail(fmt.Sprintf("unable to read available capacity crds list: %v", err))
	}

	// remove all volume crds
	for _, obj := range vList.Items {
		if err = s.Delete(testCtx, &obj); err != nil {
			Fail(fmt.Sprintf("unable to delete volume crd: %v", err))
		}
	}

	// remove all ac crds
	for _, obj := range acList.Items {
		if err = s.Delete(testCtx, &obj); err != nil {
			Fail(fmt.Sprintf("unable to delete ac crd: %v", err))
		}
	}
}
