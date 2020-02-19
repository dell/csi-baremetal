package controller

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"github.com/sirupsen/logrus"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	v12 "k8s.io/api/core/v1"
	v13core "k8s.io/api/core/v1"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testName = "someID"
	testNs   = "default"

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
	testPod1 = &v12.Pod{
		ObjectMeta: v13.ObjectMeta{Name: testPod1Name, Namespace: testNs},
		Spec:       v12.PodSpec{NodeName: testNode1Name},
		Status:     v12.PodStatus{PodIP: testPod1Ip},
	}
	// valid pod
	testPod2 = &v12.Pod{
		ObjectMeta: v13.ObjectMeta{Name: testPod2Name, Namespace: testNs},
		Spec:       v12.PodSpec{NodeName: testNode2Name},
		Status:     v12.PodStatus{PodIP: testPod2Ip},
	}
	// invalid pod, bad endpoint
	testPod3 = &v12.Pod{
		ObjectMeta: v13.ObjectMeta{Name: testPod3Name, Namespace: testNs},
		Spec:       v12.PodSpec{NodeName: testNode3Name},
		Status:     v12.PodStatus{PodIP: testPod3Ip},
	}
	// invalid pod, bad testName
	testPod4 = &v12.Pod{
		ObjectMeta: v13.ObjectMeta{Name: testPod4Name},
	}

	testVolume = vcrd.Volume{
		TypeMeta:   v13.TypeMeta{Kind: "Volume", APIVersion: "volume.dell.com/v1"},
		ObjectMeta: v13.ObjectMeta{Name: testName, Namespace: testNs},
		Spec: api.Volume{
			Id:       testName,
			Owner:    "pod",
			Size:     1000,
			Type:     "Type",
			Location: "location",
		},
	}

	testAC = accrd.AvailableCapacity{
		TypeMeta:   v13.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: v13.ObjectMeta{Name: testName, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation1,
			NodeId:   testNode1Name},
	}
	testAC2 = accrd.AvailableCapacity{
		TypeMeta:   v13.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: v13.ObjectMeta{Name: testName, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:     1024 * 1024,
			Type:     api.StorageClass_HDD,
			Location: testDriveLocation2,
			NodeId:   testNode4Name,
		},
	}

	testAC3 = accrd.AvailableCapacity{
		TypeMeta:   v13.TypeMeta{Kind: "AvailableCapacity", APIVersion: "availablecapacity.dell.com/v1"},
		ObjectMeta: v13.ObjectMeta{Name: testName, Namespace: testNs},
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
			err := svc.CreateCRD(testCtx, &testVolume, testNs, testName)
			Expect(err).To(BeNil())
			rVolume := &vcrd.Volume{}
			err = svc.ReadCRD(testCtx, testName, testNs, rVolume)
			Expect(err).To(BeNil())
			Expect(rVolume.ObjectMeta.Name).To(Equal(testName))
		})

		It("Should create and read Available Capacity CRD", func() {
			err := svc.CreateCRD(testCtx, &testAC, testNs, testName)
			Expect(err).To(BeNil())
			rAC := &accrd.AvailableCapacity{}
			err = svc.ReadCRD(testCtx, testName, testNs, rAC)
			Expect(err).To(BeNil())
			Expect(rAC.ObjectMeta.Name).To(Equal(testName))
		})

		It("Should read volumes CRD List", func() {
			err := svc.CreateCRD(context.Background(), &testVolume, testNs, testName)
			Expect(err).To(BeNil())

			vList := &vcrd.VolumeList{}
			err = svc.ReadListCRD(context.Background(), testNs, vList)
			Expect(err).To(BeNil())
			Expect(len(vList.Items)).To(Equal(1))
			Expect(vList.Items[0].Namespace).To(Equal(testNs))
		})

		It("Try to read CRD that doesn't exist", func() {
			name := "notexistingcrd"
			ac := accrd.AvailableCapacity{}
			err := svc.ReadCRD(testCtx, name, testNs, &ac)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("\"%s\" not found", name)))
		})

		It("Construct Volume CRD instance", func() {
			vol := &api.Volume{
				Id:       testName,
				Owner:    "pod",
				Size:     1000,
				Type:     "Type",
				Location: "location",
			}
			crd := svc.constructVolumeCRD(testNs, vol)
			Expect(equals(crd.Spec, testVolume)).To(BeTrue())

		})

		It("Construct AvailableCapacity CRD instance", func() {
			ac := &api.AvailableCapacity{
				Size:     1024 * 1024,
				Type:     api.StorageClass_HDD,
				Location: testDriveLocation1,
				NodeId:   testNode1Name,
			}
			crd := svc.constructAvailableCapacityCRD(testName, testNs, ac)
			Expect(crd).To(Equal(&testAC))
		})
	})

	Context("Update Available Capacity instance", func() {
		It("Should update successfully", func() {
			err := svc.CreateCRD(testCtx, &testAC, testNs, testName)
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
	})

	Context("updateCommunicator success scenarios", func() {
		It("updateCommunicator Success", func() {
			createPods(svc, testPod1)
			err := svc.updateCommunicators()
			Expect(err).To(BeNil())
			Expect(len(svc.communicators)).To(Equal(1))
		})

		It("create 3 pods and expect 2 communicators, 1 pod has incompatible testName", func() {
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
	Context("updateAvailableCapacity scenarios", func() {
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

		It("Communicator on CreateLocalVolume request returns error", func() {
			addAC(svc)
			var (
				//ctxT, _ = context.WithTimeout(context.Background(), 5 * time.Second)
				uuid     = "uuid-1234"
				node1    = "node1"
				capacity = int64(1024 * 53)
				req      = &csi.CreateVolumeRequest{
					Name:               uuid,
					CapacityRange:      &csi.CapacityRange{RequiredBytes: capacity},
					VolumeCapabilities: make([]*csi.VolumeCapability, 0),
					AccessibilityRequirements: &csi.TopologyRequirement{
						Preferred: []*csi.Topology{
							{
								Segments: map[string]string{"baremetal-csi/nodeid": node1},
							},
						},
					},
				}
				mc = &mocks.VolumeMgrClientMock{}
			)
			svc.communicators[NodeID(node1)] = mc
			mc.On("CreateLocalVolume", &api.CreateLocalVolumeRequest{
				PvcUUID:  uuid,
				Capacity: capacity,
				Sc:       "hdd",
				Location: testDriveLocation1,
			}).Return(&api.CreateLocalVolumeResponse{}, errors.New("error"))
			_, err := svc.CreateVolume(context.Background(), req)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "Unable to create volume on local node.")))
		})
	})

	Context("Success scenarios", func() {
		It("Volume is created successfully", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)
			req := getCreateVolumeRequest(uuid, capacity, testNode4Name)
			mc := &mocks.VolumeMgrClientMock{}
			svc.communicators[NodeID(testNode4Name)] = mc
			// Prepare response from NodeService
			addAC(svc)
			mc.On("CreateLocalVolume", &api.CreateLocalVolumeRequest{
				PvcUUID:  uuid,
				Capacity: capacity,
				Sc:       "hdd",
				Location: testDriveLocation2,
			}).Return(&api.CreateLocalVolumeResponse{Drive: testDriveLocation2, Capacity: capacity, Ok: true}, nil)
			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).NotTo(BeNil())
			Expect(resp.Volume.VolumeId).To(Equal(uuid))
			//Expect(resp.Volume.CapacityBytes).To(Equal(capacity))
			Expect(err).To(BeNil())
			volumeFromCache, ok := svc.volumeCache.items[VolumeID(uuid)]
			Expect(ok).To(BeTrue())
			Expect(volumeFromCache.Spec.Owner).To(Equal(testNode4Name))
			vCrd := &vcrd.Volume{}
			err = svc.ReadCRD(context.Background(), uuid, testNs, vCrd)
			Expect(err).To(BeNil())
		})

		It("Volume is found in cache", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)

			req := getCreateVolumeRequest(uuid, capacity, testNode4Name)
			mc := &mocks.VolumeMgrClientMock{}
			svc.communicators[NodeID(testNode4Name)] = mc

			_ = svc.volumeCache.addVolumeToCache(&vcrd.Volume{Spec: api.Volume{
				Id:     req.GetName(),
				Size:   1024 * 60,
				Owner:  testNode4Name,
				Status: api.OperationalStatus_Created,
			}}, req.GetName())

			resp, err := svc.CreateVolume(context.Background(), req)
			Expect(resp).NotTo(BeNil())
			Expect(resp.Volume.VolumeId).To(Equal(uuid))
			Expect(resp.Volume.CapacityBytes).To(Equal(int64(1024 * 60)))
			Expect(err).To(BeNil())
		})
	})
})

var _ = Describe("CSIControllerService DeleteVolume", func() {
	var (
		svc      *CSIControllerService
		node     = "node1"
		uuid     = "uuid-1234"
		capacity = int64(1024 * 42)
	)

	BeforeEach(func() {
		svc = newSvc()
	})

	Context("Fail scenarios", func() {
		It("Request doesn't contain volume ID", func() {
			dreq := &csi.DeleteVolumeRequest{}
			resp, err := svc.DeleteVolume(context.Background(), dreq)
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.InvalidArgument, "Volume ID must be provided")))
		})

		It("Volume isn't found in cache", func() {
			vID := "some-id"
			dreq := &csi.DeleteVolumeRequest{VolumeId: vID}
			resp, err := svc.DeleteVolume(context.Background(), dreq)
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(fmt.Errorf("unable to find volume with ID %s in cache", vID)))
		})

		It("Communicator on DeleteLocalVolume returns error or false", func() {
			mc := &mocks.VolumeMgrClientMock{}
			// prepare communicator
			svc.communicators[NodeID(node)] = mc
			dlReq := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}
			dlResp := &api.DeleteLocalVolumeResponse{Ok: false}
			mc.On("DeleteLocalVolume", dlReq).Return(dlResp, errors.New("error")).Times(1)

			// prepare cache
			_ = svc.volumeCache.addVolumeToCache(&vcrd.Volume{
				Spec: api.Volume{
					Id:    uuid,
					Owner: node,
					Size:  capacity,
				}}, uuid)

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

			// prepare cache
			_ = svc.volumeCache.addVolumeToCache(&vcrd.Volume{
				Spec: api.Volume{
					Id:    uuid,
					Owner: node,
					Size:  capacity,
				}}, uuid)

			resp, err := svc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "Unable to delete volume from node")))
		})
	})

	Context("Success scenarios", func() {
		It("Volume was delete successful", func() {
			mc := &mocks.VolumeMgrClientMock{}
			// prepare communicator
			svc.communicators[NodeID(node)] = mc
			dlReq := &api.DeleteLocalVolumeRequest{PvcUUID: uuid}

			localVolume := api.Volume{
				Id:       uuid,
				Owner:    node,
				Size:     capacity,
				Location: testDriveLocation1,
			}

			dlResp := &api.DeleteLocalVolumeResponse{Ok: true, Volume: &localVolume}
			mc.On("DeleteLocalVolume", dlReq).Return(dlResp, nil).Times(1)

			// prepare cache
			_ = svc.volumeCache.addVolumeToCache(&vcrd.Volume{Spec: localVolume}, uuid)

			// create volume crd to delete
			volumeCrd := svc.constructVolumeCRD("default", &localVolume)
			_ = svc.CreateCRD(testCtx, volumeCrd, "default", uuid)

			resp, err := svc.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())
			Expect(len(svc.volumeCache.items)).To(Equal(0))
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

// compare api.Volume with volume crd
func equals(volume api.Volume, volume2 vcrd.Volume) bool {
	return volume.Id == volume2.Spec.Id &&
		volume.Status == volume2.Spec.Status &&
		volume.Health == volume2.Spec.Health &&
		volume.Location == volume2.Spec.Location &&
		volume.Type == volume2.Spec.Type &&
		volume.Mode == volume2.Spec.Mode &&
		volume.Size == volume2.Spec.Size &&
		volume.Owner == volume2.Spec.Owner
}

// create provided pods via client from provided svc
func createPods(s *CSIControllerService, pods ...*v12.Pod) {
	for _, pod := range pods {
		err := s.Create(context.Background(), pod)
		if err != nil {
			Fail(fmt.Sprintf("uable to create pod %s", pod.Name))
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

	err = v12.AddToScheme(scheme)
	if err != nil {
		os.Exit(1)
	}

	err = accrd.AddToSchemeAvailableCapacity(scheme)
	if err != nil {
		panic(err)
	}

	nSvc := NewControllerService(fake.NewFakeClientWithScheme(scheme), logrus.New())
	return nSvc
}

// remove all pods via client from provided svc
func removeAllPods(s *CSIControllerService) {
	pods := v13core.PodList{}
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

	if err = s.ReadListCRD(testCtx, testNs, vList); err != nil {
		Fail(fmt.Sprintf("unable to read volume crds list: %v", err))
	}

	if err = s.ReadListCRD(testCtx, testNs, acList); err != nil {
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
	return &csi.CreateVolumeRequest{
		Name:               name,
		CapacityRange:      &csi.CapacityRange{RequiredBytes: cap},
		VolumeCapabilities: make([]*csi.VolumeCapability, 0),
		AccessibilityRequirements: &csi.TopologyRequirement{
			Preferred: []*csi.Topology{
				{
					Segments: map[string]string{"baremetal-csi/nodeid": preferredNode},
				},
			},
		},
	}
}
