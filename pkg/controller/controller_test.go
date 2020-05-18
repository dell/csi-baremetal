package controller

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	crdV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	vcrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/k8s"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/sc"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/testutils"
)

var (
	testLogger = logrus.New()
	testID     = "someID"
	testNs     = "default"

	testCtx       = context.Background()
	testPod1Name  = fmt.Sprintf("%s-testPod1", NodeSvcPodsMask)
	testPod2Name  = fmt.Sprintf("%s-testPod2", NodeSvcPodsMask)
	testPod3Name  = "SomeName"
	testPod1Ip    = "10.10.10.10"
	testPod2Ip    = "10.10.10.11"
	testNode1Name = "node1"
	testNode2Name = "node2"

	testDriveLocation1 = "drive1-sn"
	testDriveLocation2 = "drive2-sn"
	testDriveLocation4 = "drive4-sn"
	testNode4Name      = "preferredNode"
	// valid pod
	testReadyPod1 = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod1Name, Namespace: testNs},
		Spec:       coreV1.PodSpec{NodeName: testNode1Name},
		Status: coreV1.PodStatus{
			PodIP: testPod1Ip,
			ContainerStatuses: []coreV1.ContainerStatus{
				{
					Name:  "hwmgr",
					Ready: true,
				},
				{
					Name:  "node",
					Ready: true,
				},
				{
					Name:  "sidecar",
					Ready: true,
				},
			},
		},
	}
	// invalid pod, not all containers are ready
	testUnreadyPod2 = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod2Name, Namespace: testNs},
		Spec:       coreV1.PodSpec{NodeName: testNode2Name},
		Status: coreV1.PodStatus{
			PodIP: testPod2Ip,
			ContainerStatuses: []coreV1.ContainerStatus{
				{
					Name:  "hwmgr",
					Ready: true,
				},
				{
					Name:  "node",
					Ready: false,
				},
				{
					Name:  "sidecar",
					Ready: true,
				},
			},
		},
	}

	// invalid pod, bad testID
	testPod3 = &coreV1.Pod{
		ObjectMeta: k8smetav1.ObjectMeta{Name: testPod3Name},
	}

	testVolume = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testID,
			Namespace:         testNs,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:       testID,
			NodeId:   "pod",
			Size:     1000,
			Type:     string(sc.XFS),
			Location: "location",
			Mode:     apiV1.ModeFS,
		},
	}

	testAC1Name = fmt.Sprintf("%s-%s", testNode1Name, strings.ToLower(testDriveLocation1))
	testAC1     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC1Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:         1024 * 1024 * 1024,
			StorageClass: apiV1.StorageClassHDD,
			Location:     testDriveLocation1,
			NodeId:       testNode1Name},
	}
	testAC2Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation2))
	testAC2     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC2Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:         1024 * 1024 * 1024 * 1024,
			StorageClass: apiV1.StorageClassHDD,
			Location:     testDriveLocation2,
			NodeId:       testNode2Name,
		},
	}
	testAC3Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation4))
	testAC3     = accrd.AvailableCapacity{
		TypeMeta:   k8smetav1.TypeMeta{Kind: "AvailableCapacity", APIVersion: crdV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{Name: testAC3Name, Namespace: testNs},
		Spec: api.AvailableCapacity{
			Size:         1024 * 1024 * 1024 * 100,
			StorageClass: apiV1.StorageClassHDDLVG,
			Location:     testDriveLocation4,
			NodeId:       testNode2Name,
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

	Context("WaitNodeServices scenarios", func() {
		It("success scenario when there is ready Node pod", func() {
			createPods(svc, testReadyPod1, testUnreadyPod2)

			res := svc.WaitNodeServices()
			Expect(res).To(BeTrue())
		})

		It("failed scenario when there is no ready Node pod", func() {
			createPods(svc, testUnreadyPod2)

			res := svc.WaitNodeServices()
			Expect(res).To(BeFalse())
		})

		It("failed scenario when there is no Node pod", func() {
			createPods(svc, testPod3)

			res := svc.WaitNodeServices()
			Expect(res).To(BeFalse())
		})

	})
})

var _ = Describe("CSIControllerService CreateVolume", func() {
	var controller *CSIControllerService

	BeforeEach(func() {
		controller = newSvc()
	})

	Context("Fail scenarios", func() {
		It("Missing request name", func() {
			req := &csi.CreateVolumeRequest{}
			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume name missing in request"))
		})
		It("Missing volume capabilities", func() {
			req := &csi.CreateVolumeRequest{Name: "some-name-1"}
			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("Volume capabilities missing in request"))
		})
		It("There is no suitable Available Capacity (on all nodes)", func() {
			req := getCreateVolumeRequest("req1", 1024*1024*1024*1024, "")

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("there is no suitable drive for request"))
		})
		It("Status Failed was set in Volume CR", func() {
			err := testutils.AddAC(controller.k8sclient, &testAC1, &testAC2)
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name)
				vol      = &vcrd.Volume{}
			)

			go testutils.VolumeReconcileImitation(controller.svc, "req1", crdV1.Failed)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "Unable to create volume")))
			err = controller.k8sclient.ReadCR(context.Background(), "req1", vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.CSIStatus).To(Equal(crdV1.Failed))
		})
		It("Volume CR creation timeout expired", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)

			req := getCreateVolumeRequest(uuid, capacity, testNode4Name)

			err := controller.k8sclient.CreateCR(context.Background(), req.GetName(), &vcrd.Volume{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name:              uuid,
					Namespace:         "default",
					CreationTimestamp: k8smetav1.Time{Time: time.Now().Add(time.Duration(-100) * time.Minute)},
				},
				Spec: api.Volume{
					Id:        req.GetName(),
					Size:      1024 * 60,
					NodeId:    testNode1Name,
					CSIStatus: crdV1.Creating,
				}})
			Expect(err).To(BeNil())

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			v := vcrd.Volume{}
			err = controller.k8sclient.ReadCR(testCtx, req.GetName(), &v)
			Expect(err).To(BeNil())
			Expect(v.Spec.CSIStatus).To(Equal(crdV1.Failed))
		})
	})

	Context("Success scenarios", func() {
		It("Volume is created successfully", func() {
			err := testutils.AddAC(controller.k8sclient, &testAC1, &testAC2)
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name)
				vol      = &vcrd.Volume{}
			)

			go testutils.VolumeReconcileImitation(controller.svc, "req1", crdV1.Created)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())

			err = controller.k8sclient.ReadCR(context.Background(), "req1", vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.CSIStatus).To(Equal(crdV1.Created))
		})
		It("Volume CR has already exists", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)

			req := getCreateVolumeRequest(uuid, capacity, testNode4Name)

			err := controller.k8sclient.CreateCR(context.Background(), req.GetName(), &vcrd.Volume{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name:              uuid,
					Namespace:         "default",
					CreationTimestamp: k8smetav1.Time{Time: time.Now()},
				},
				Spec: api.Volume{
					Id:        req.GetName(),
					Size:      1024 * 60,
					NodeId:    testNode1Name,
					CSIStatus: crdV1.Created,
				}})
			Expect(err).To(BeNil())

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
			Expect(resp.Volume.VolumeId).To(Equal(uuid))
			Expect(resp.Volume.CapacityBytes).To(Equal(int64(1024 * 60)))
		})
	})
})

var _ = Describe("CSIControllerService DeleteVolume", func() {
	var (
		controller *CSIControllerService
		node       = "node1"
		uuid       = "uuid-1234"
	)

	BeforeEach(func() {
		controller = newSvc()
		// prepare crd
		println("BEFORE EACH CREATE CR")
		err := controller.k8sclient.CreateCR(context.Background(), uuid, &vcrd.Volume{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      uuid,
				Namespace: testNs,
			},
			TypeMeta: k8smetav1.TypeMeta{
				Kind:       "Volume",
				APIVersion: crdV1.APIV1Version,
			},
			Spec: api.Volume{
				Id:     uuid,
				NodeId: node,
			}})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		removeAllCrds(controller.k8sclient)
	})

	Context("Fail scenarios", func() {

		It("Request doesn't contain volume ID", func() {
			dreq := &csi.DeleteVolumeRequest{}
			resp, err := controller.DeleteVolume(context.Background(), dreq)
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.InvalidArgument, "Volume ID must be provided")))
		})
		It("Node service mark volume as FailToRemove", func() {
			var (
				volumeCrd = &vcrd.Volume{}
				err       error
			)
			// create volume crd to delete
			err = controller.k8sclient.CreateCR(testCtx, uuid, volumeCrd)
			Expect(err).To(BeNil())

			go testutils.VolumeReconcileImitation(controller.svc, volumeCrd.Spec.Id, crdV1.Failed)

			resp, err := controller.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})

			Expect(resp).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Unable to delete volume"))

			err = controller.k8sclient.ReadCR(context.Background(), uuid, volumeCrd)
			Expect(err).To(BeNil())
			Expect(volumeCrd.Spec.CSIStatus).To(Equal(crdV1.Failed))
		})
	})

	Context("Success scenarios", func() {
		It("Volume CRD isn't found, consider that volume was removed", func() {
			vID := "some-id"
			dreq := &csi.DeleteVolumeRequest{VolumeId: vID}
			resp, err := controller.DeleteVolume(context.Background(), dreq)
			Expect(resp).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
		It("Volume is deleted successful, sc HDD", func() {
			var (
				volumeCrd = &vcrd.Volume{
					TypeMeta: k8smetav1.TypeMeta{
						Kind:       "Volume",
						APIVersion: crdV1.APIV1Version,
					},
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      uuid,
						Namespace: controller.k8sclient.Namespace,
					},
					Spec: api.Volume{
						Id:       uuid,
						NodeId:   node,
						Location: testDriveLocation1,
					},
				}
				err error
			)
			// create volume crd to delete
			err = controller.k8sclient.CreateCR(testCtx, uuid, volumeCrd)
			Expect(err).To(BeNil())

			go testutils.VolumeReconcileImitation(controller.svc, volumeCrd.Spec.Id, crdV1.Removed)

			resp, err := controller.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())

			err = controller.k8sclient.ReadCR(context.Background(), uuid, volumeCrd)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
		It("Volume is deleted successful, sc HDDLVG and AC size is increased", func() {
			removeAllCrds(controller.k8sclient)                    // remove CRs that was created in BeforeEach()
			err := testutils.AddAC(controller.k8sclient, &testAC3) // create AC CR, expect that size of that AC will be increased
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 101)
				volume   = api.Volume{
					Id:           uuid,
					NodeId:       testNode2Name,
					Location:     testDriveLocation4, // testAC4
					Size:         capacity,
					StorageClass: apiV1.StorageClassHDDLVG,
				}
				volumeCrd = vcrd.Volume{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      uuid,
						Namespace: controller.k8sclient.Namespace,
					},
					Spec: volume,
				}
			)
			// create volume CR that should be deleted (created in BeforeEach)
			err = controller.k8sclient.CreateCR(testCtx, uuid, &volumeCrd)
			Expect(err).To(BeNil())

			go testutils.VolumeReconcileImitation(controller.svc, volumeCrd.Spec.Id, crdV1.Removed)

			resp, err := controller.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())

			//// check that there are no any volume CR (was removed)
			vList := vcrd.VolumeList{}
			err = controller.k8sclient.ReadList(testCtx, &vList)
			Expect(err).To(BeNil())
			Expect(len(vList.Items)).To(Equal(0))
			// check that AC size was increased on capacity
			acList := accrd.AvailableCapacityList{}
			err = controller.k8sclient.ReadList(context.Background(), &acList)
			Expect(err).To(BeNil())
			Expect(len(acList.Items)).To(Equal(1)) // expect that amount of AC was not increased
			Expect(acList.Items[0].Spec.Size - capacity).To(Equal(testAC3.Spec.Size))
		})
		It("Volume is deleted successful, LVG AC recreated", func() {
			removeAllCrds(controller.k8sclient) // remove CRs that was created in BeforeEach()
			fullLVGsizeVolume := testVolume
			fullLVGsizeVolume.Spec.StorageClass = apiV1.StorageClassHDDLVG

			// create volume CR that should be deleted
			err := controller.k8sclient.CreateCR(testCtx, testID, &fullLVGsizeVolume)
			Expect(err).To(BeNil())

			go testutils.VolumeReconcileImitation(controller.svc, fullLVGsizeVolume.Spec.Id, crdV1.Removed)

			resp, err := controller.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: testID})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())

			// check that there are no any volume CR (was removed)
			vList := vcrd.VolumeList{}
			err = controller.k8sclient.ReadList(testCtx, &vList)
			Expect(err).To(BeNil())
			Expect(len(vList.Items)).To(Equal(0))
			// check that AC size still not exist
			acList := accrd.AvailableCapacityList{}
			err = controller.k8sclient.ReadList(context.Background(), &acList)
			Expect(err).To(BeNil())
			Expect(len(acList.Items)).To(Equal(0))
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

// create and instance of CSIControllerService with scheme for working with CRD
func newSvc() *CSIControllerService {
	kubeclient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	nSvc := NewControllerService(kubeclient, testLogger)
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
		Name:          name,
		CapacityRange: &csi.CapacityRange{RequiredBytes: cap},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType:     string(sc.XFS),
						MountFlags: nil,
					},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
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
func removeAllCrds(s *k8s.KubeClient) {
	var (
		vList  = &vcrd.VolumeList{}
		acList = &accrd.AvailableCapacityList{}
		err    error
	)

	println("Removing all CRs")
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
	println("CRs were removed")
}
