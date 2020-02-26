package controller

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	coreV1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	testDriveLocation1 = "drive"
	testDriveLocation2 = "drive1-sn1"
	testDriveLocation3 = "drive2"
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
		TypeMeta:   k8smetav1.TypeMeta{Kind: "Volume", APIVersion: "volume.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testID, Namespace: testNs},
		Spec: api.Volume{
			Id:       testID,
			Owner:    "pod",
			Size:     1000,
			Type:     "Type",
			Location: "location",
		},
	}

	testAC = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testID, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation1,
			NodeId:   testNode1Name},
	}
	testAC2 = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testID, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation2,
			NodeId:   testNode4Name,
		},
	}

	testAC3 = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testID, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024 * 2,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation3,
			NodeId:   testNode4Name,
		},
	}
)

func TestCSIControllerService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CSIControllerService testing suite")
}

var _ = Describe("CSIControllerService manipulations with CRD", func() {
	var svc *CSIControllerService

	BeforeEach(func() {
		svc = newSvc()
	})

	AfterEach(func() {
		removeAllCrds(svc)
	})

	Context("Create and read CRDs (volume and AC)", func() {
		It("Should create and read Volume CRD", func() {
			err := svc.CreateCRD(testCtx, &testVolume, testID)
			Expect(err).To(BeNil())
			rVolume := &vcrd.Volume{}
			err = svc.ReadCRD(testCtx, testID, rVolume)
			Expect(err).To(BeNil())
			Expect(rVolume.ObjectMeta.Name).To(Equal(testID))
		})

		It("Should create and read Available Capacity CRD", func() {
			err := svc.CreateCRD(testCtx, &testAC, testID)
			Expect(err).To(BeNil())
			rAC := &accrd.AvailableCapacity{}
			err = svc.ReadCRD(testCtx, testID, rAC)
			Expect(err).To(BeNil())
			Expect(rAC.ObjectMeta.Name).To(Equal(testID))
		})

		It("Should read volumes CRD List", func() {
			err := svc.CreateCRD(context.Background(), &testVolume, testID)
			Expect(err).To(BeNil())

			vList := &vcrd.VolumeList{}
			err = svc.ReadListCRD(context.Background(), vList)
			Expect(err).To(BeNil())
			Expect(len(vList.Items)).To(Equal(1))
			Expect(vList.Items[0].Namespace).To(Equal(testNs))
		})

		It("Try to read CRD that doesn't exist", func() {
			name := "notexistingcrd"
			ac := accrd.AvailableCapacity{}
			err := svc.ReadCRD(testCtx, name, &ac)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("\"%s\" not found", name)))
		})

		It("Construct AvailableCapacity CRD instance", func() {
			ac := &api.AvailableCapacity{
				Size:     1024 * 1024,
				Type:     api.StorageClass_HDD,
				Location: testDriveLocation1,
				NodeId:   testNode1Name,
			}
			crd := svc.constructAvailableCapacityCRD(testID, ac)
			Expect(crd).To(Equal(&testAC))
		})
	})

	Context("Update Available Capacity instance", func() {
		It("Should update successfully", func() {
			err := svc.CreateCRD(testCtx, &testAC, testID)
			Expect(err).To(BeNil())

			newSize := int64(1024 * 105)
			testAC.Spec.Size = newSize

			err = svc.UpdateCRD(testCtx, &testAC)
			Expect(err).To(BeNil())
			Expect(testAC.Spec.Size).To(Equal(newSize))

			acCopy := testAC.DeepCopy()
			err = svc.Update(testCtx, &testAC)
			Expect(err).To(BeNil())
			Expect(&testAC).To(Equal(acCopy))
		})

		It("Update should fail", func() {

		})
	})
})

var _ = Describe("CSIControllerService addition functions", func() {
	var svc *CSIControllerService

	BeforeEach(func() {
		svc = newSvc()
	})

	AfterEach(func() {
		removeAllPods(svc)
		removeAllCrds(svc)
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
		It("Found ac with drive-node1 id", func() {
			addAC(svc)
			requiredCapacity := int64(900)
			drive := svc.searchAvailableCapacity(testNode1Name, requiredCapacity)
			Expect(testDriveLocation1).To(Equal(drive.Spec.Location))
		})
		It("Found ac with preferredNode-drive1-sn1 id", func() {
			addAC(svc)
			requiredCapacity := int64(2000)
			drive := svc.searchAvailableCapacity(testNode4Name, requiredCapacity)
			Expect(testDriveLocation2).To(Equal(drive.Spec.Location))

		})
		It("Couldn't find any ac because of requiredCapacity", func() {
			addAC(svc)
			drive := svc.searchAvailableCapacity(testNode1Name, 1024*1024*2)
			Expect(drive).To(BeNil())
		})
		It("Couldn't find any ac because of preferred node", func() {
			addAC(svc)
			drive := svc.searchAvailableCapacity("node", 1024)
			Expect(drive).To(BeNil())
		})
		It("Choose preferred node", func() {
			addAC(svc)
			err := svc.availableCapacityCache.Create(&testAC3, testNode4Name, testDriveLocation3)
			Expect(err).To(BeNil())
			drive := svc.searchAvailableCapacity("", 1024)
			Expect(*drive).To(Equal(testAC2))
		})
		It("No available capacity", func() {
			drive := svc.searchAvailableCapacity("", 1024)
			Expect(drive).To(BeNil())
			drive = svc.searchAvailableCapacity(testNode4Name, 1024)
			Expect(drive).To(BeNil())
		})
	})
	Context("updateAvailableCapacityCache scenarios", func() {
		It("Failed to create ac because of GetAvailableCapacity request error", func() {
			mc := &mocks.VolumeMgrClientMock{}
			mc.On("GetAvailableCapacity", &api.AvailableCapacityRequest{
				// Prepare response from NodeService
				NodeId: testNode4Name,
			}).Return(&api.AvailableCapacityResponse{}, errors.New("error during GetAvailableCapacity request"))
			svc.communicators[NodeID(testNode4Name)] = mc
			err := svc.updateAvailableCapacityCache(context.Background())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not all available capacity were created"))
		})
		It("Create empty cache", func() {
			mc := &mocks.VolumeMgrClientMock{}
			response := &api.AvailableCapacityResponse{
				AvailableCapacity: make([]*api.AvailableCapacity, 0),
			}
			mc.On("GetAvailableCapacity", &api.AvailableCapacityRequest{
				// Prepare response from NodeService
				NodeId: testNode4Name,
			}).Return(response, nil)
			svc.communicators[NodeID(testNode4Name)] = mc
			err := svc.updateAvailableCapacityCache(context.Background())
			Expect(err).To(BeNil())
			Expect(len(svc.availableCapacityCache.items)).To(Equal(0))
		})
		It("Create cache with 1 ac", func() {
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
			err := svc.updateAvailableCapacityCache(context.Background())
			Expect(err).To(BeNil())
			Expect(len(svc.availableCapacityCache.items)).To(Equal(1))
		})
	})
	Context("waitVCRDStatus scenarios", func() {
		BeforeEach(func() {
			err := svc.CreateCRD(context.Background(), &testVolume, testID)
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			err := svc.DeleteCRD(context.Background(), &testVolume)
			Expect(err).To(BeNil())
		})

		It("Context was closed", func() {
			ctxT, cancelFn := context.WithTimeout(context.Background(), 10*time.Millisecond)
			reached, statusCode := svc.waitVCRDStatus(ctxT, testID)
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
				reached, statusCode = svc.waitVCRDStatus(ctxT, testID, api.OperationalStatus_FailedToCreate)
				cancelFn()
				wg.Done()
			}()
			testV2 := testVolume
			testV2.Spec.Status = api.OperationalStatus_FailedToCreate
			err := svc.UpdateCRD(context.Background(), &testV2)
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
			addAC(svc)
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
				Sc:       "hdd",
				Location: testDriveLocation1,
			}).Return(&api.CreateLocalVolumeResponse{}, errors.New("error"))
			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "Unable to create volume on local node.")))
			err = svc.ReadCRD(context.Background(), "req1", vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.Status).To(Equal(api.OperationalStatus_FailedToCreate))
		})
	})

	Context("Success scenarios", func() {
		It("Volume is created successfully", func() {
			addAC(svc)
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
				Sc:       "hdd",
				Location: testDriveLocation1,
			}).Return(&api.CreateLocalVolumeResponse{Ok: true}, nil)
			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())

			err = svc.ReadCRD(context.Background(), "req1", vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.Status).To(Equal(api.OperationalStatus_Created))
		})
		//
		//		It("Volume is found in cache", func() {
		//			uuid := "uuid-1234"
		//			capacity := int64(1024 * 42)
		//
		//			req := getCreateVolumeRequest(uuid, capacity, testNode4Name)
		//			mc := &mocks.VolumeMgrClientMock{}
		//			svc.communicators[NodeID(testNode4Name)] = mc
		//
		//			_ = svc.volumeCache.addVolumeToCache(&vcrd.Volume{Spec: api.Volume{
		//				Id:     req.GetName(),
		//				Size:   1024 * 60,
		//				Owner:  testNode4Name,
		//				Status: api.OperationalStatus_Created,
		//			}}, req.GetName())
		//
		//			resp, err := svc.CreateVolume(context.Background(), req)
		//			Expect(resp).NotTo(BeNil())
		//			Expect(resp.Volume.VolumeId).To(Equal(uuid))
		//			Expect(resp.Volume.CapacityBytes).To(Equal(int64(1024 * 60)))
		//			Expect(err).To(BeNil())
		//		})
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
		err := svc.CreateCRD(context.Background(), &vcrd.Volume{
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

	AfterEach(func() {
		removeAllCrds(svc)
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
					Namespace: svc.namespace,
				},
				Spec: localVolume,
			}
			_ = svc.CreateCRD(testCtx, volumeCrd, uuid)

			resp, err := svc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())
			Expect(len(svc.availableCapacityCache.items)).To(Equal(1))
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
		err := s.Create(context.Background(), pod)
		if err != nil {
			Fail(fmt.Sprintf("uable to create pod %s, error: %v", pod.Name, err))
		}
	}
}

// add available capacity to svc cache
func addAC(s *CSIControllerService) {
	err := s.availableCapacityCache.Create(&testAC, testNode1Name, testDriveLocation1)
	if err != nil {
		Fail(fmt.Sprintf("uable to create ac %s, %s", testNode1Name, testDriveLocation1))
	}
	err = s.availableCapacityCache.Create(&testAC2, testNode4Name, testDriveLocation2)
	if err != nil {
		Fail(fmt.Sprintf("unable to create ac %s, %s", testNode4Name, testDriveLocation2))
	}
}

// create and instance of CSIControllerService with scheme for working with CRD
func newSvc() *CSIControllerService {
	scheme := runtime.NewScheme()
	err := vcrd.AddToScheme(scheme)
	if err != nil {
		os.Exit(1)
	}

	err = coreV1.AddToScheme(scheme)
	if err != nil {
		os.Exit(1)
	}

	err = accrd.AddToSchemeAvailableCapacity(scheme)
	if err != nil {
		panic(err)
	}

	nSvc := NewControllerService(fake.NewFakeClientWithScheme(scheme), logrus.New(), testNs)
	return nSvc
}

// remove all pods via client from provided svc
func removeAllPods(s *CSIControllerService) {
	pods := coreV1.PodList{}
	err := s.List(context.Background(), &pods, k8sclient.InNamespace(testNs))
	if err != nil {
		Fail(fmt.Sprintf("unable to get pods list: %v", err))
	}
	for _, pod := range pods.Items {
		err = s.Delete(context.Background(), &pod)
		if err != nil {
			Fail(fmt.Sprintf("unable to delete pod: %v", err))
		}
	}
}

// remove all crds (volume and ac)
func removeAllCrds(s *CSIControllerService) {
	var (
		vList  = &vcrd.VolumeList{}
		acList = &accrd.AvailableCapacityList{}
		err    error
	)

	if err = s.ReadListCRD(testCtx, vList); err != nil {
		Fail(fmt.Sprintf("unable to read volume crds list: %v", err))
	}

	if err = s.ReadListCRD(testCtx, acList); err != nil {
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
