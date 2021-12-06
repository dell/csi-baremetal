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

package controller

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	vcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base/cache"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/common"
	csibmnodeconst "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
	"github.com/dell/csi-baremetal/pkg/mocks"
	"github.com/dell/csi-baremetal/pkg/testutils"
)

var (
	testLogger = logrus.New()
	testID     = "someID"
	testNs     = "default"
	testApp    = "app"
	testPod    = "pod"

	testCtx       = context.Background()
	testNode1Name = "node1"
	testNode2Name = "node2"

	testDriveLocation1 = "drive1-sn"
	testDriveLocation2 = "drive2-sn"
	testDriveLocation4 = "drive4-sn"
	testNode4Name      = "preferredNode"

	testVolume = vcrd.Volume{
		TypeMeta: k8smetav1.TypeMeta{Kind: "Volume", APIVersion: apiV1.APIV1Version},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:              testID,
			Namespace:         testNs,
			CreationTimestamp: k8smetav1.Time{Time: time.Now()},
		},
		Spec: api.Volume{
			Id:       testID,
			NodeId:   "pod",
			Size:     1000,
			Type:     string(fs.XFS),
			Location: "location",
			Mode:     apiV1.ModeFS,
		},
	}

	testAC1Name = fmt.Sprintf("%s-%s", testNode1Name, strings.ToLower(testDriveLocation1))
	testAC1     = accrd.AvailableCapacity{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: testAC1Name,
		},
		Spec: api.AvailableCapacity{
			Size:         1024 * 1024 * 1024,
			StorageClass: apiV1.StorageClassHDD,
			Location:     testDriveLocation1,
			NodeId:       testNode1Name,
		},
	}
	testPVC1Name = testAC1Name + "-PVC"
	testPVC1     = v1.PersistentVolumeClaim{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      testPVC1Name,
			Namespace: testNs,
			Labels: map[string]string{
				k8s.AppLabelKey: testApp,
			},
		},
	}
	testACR1Name = testAC1Name + "-reservation"
	testACR1     = acrcrd.AvailableCapacityReservation{
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       "AvailableCapacityReservation",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: testAC1Name,
		},
		Spec: api.AvailableCapacityReservation{
			Namespace: testNs,
			Status:    apiV1.ReservationConfirmed,
			NodeRequests: &api.NodeRequests{
				Requested: []string{testNode1Name},
				Reserved:  []string{testNode1Name},
			},
			ReservationRequests: []*api.ReservationRequest{
				&api.ReservationRequest{
					CapacityRequest: &api.CapacityRequest{
						Name:         testPVC1Name,
						Size:         1024 * 1024 * 1024,
						StorageClass: apiV1.StorageClassHDD,
					},
					Reservations: []string{testAC1Name},
				},
			},
		},
	}
	testAC2Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation2))
	testAC2     = accrd.AvailableCapacity{
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      testAC2Name,
			Namespace: testNs,
		},
		Spec: api.AvailableCapacity{
			Size:         1024 * 1024 * 1024 * 1024,
			StorageClass: apiV1.StorageClassHDD,
			Location:     testDriveLocation2,
			NodeId:       testNode2Name,
		},
	}
	testAC3Name = fmt.Sprintf("%s-%s", testNode2Name, strings.ToLower(testDriveLocation4))
	testAC3     = accrd.AvailableCapacity{
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: apiV1.APIV1Version,
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      testAC3Name,
			Namespace: testNs,
		},
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
		It("Reservation not found", func() {
			req := getCreateVolumeRequest("req1", 1024*1024*1024*1024, "", "testClaim", false, false)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(status.Code(err)).To(Equal(codes.NotFound))
			Expect(err.Error()).To(ContainSubstring("Reservation testClaim not found"))
		})
		It("Available Capacity not found", func() {
			err := controller.k8sclient.Create(testCtx, testPVC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.CreateCR(testCtx, testACR1.Name, testACR1.DeepCopy())
			Expect(err).To(BeNil())

			req := getCreateVolumeRequest("req1", 1024*1024*1024*1024, "", testPVC1Name, false, false)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
		It("Status Failed was set in Volume CR", func() {
			err := testutils.AddAC(controller.k8sclient, testAC1.DeepCopy(), testAC2.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.Create(testCtx, testPVC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.CreateCR(testCtx, testACR1.Name, testACR1.DeepCopy())
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name, testPVC1Name, false, false)
				vol      = &vcrd.Volume{}
			)

			go testutils.VolumeReconcileImitation(controller.k8sclient, "req1", testNs, apiV1.Failed)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(status.Error(codes.Internal, "Unable to create volume")))
			err = controller.k8sclient.ReadCR(context.Background(), "req1", testNs, vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.CSIStatus).To(Equal(apiV1.Failed))
		})
		It("Volume CR creation timeout expired", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)

			req := getCreateVolumeRequest(uuid, capacity, testNode4Name, testPVC1Name, false, false)

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
					CSIStatus: apiV1.Creating,
				}})
			Expect(err).To(BeNil())

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).ToNot(BeNil())
			v := vcrd.Volume{}
			err = controller.k8sclient.ReadCR(testCtx, req.GetName(), "default", &v)
			Expect(err).To(BeNil())
			Expect(v.Spec.CSIStatus).To(Equal(apiV1.Failed))
		})
	})

	Context("Success scenarios", func() {
		It("Volume is created successfully", func() {
			err := testutils.AddAC(controller.k8sclient, testAC1.DeepCopy(), testAC2.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.Create(testCtx, testPVC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.CreateCR(testCtx, testACR1.Name, testACR1.DeepCopy())
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name, testPVC1Name, false, false)
				vol      = &vcrd.Volume{}
			)

			go testutils.VolumeReconcileImitation(controller.k8sclient, "req1", testNs, apiV1.Created)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())

			err = controller.k8sclient.ReadCR(context.Background(), "req1", testNs, vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.CSIStatus).To(Equal(apiV1.Created))
			Expect(vol.Spec.Mode).To(Equal(apiV1.ModeFS))
			Expect(vol.Labels[k8s.AppLabelKey]).To(Equal(testApp))
		})
		It("Volume is created successfully (Block)", func() {
			err := testutils.AddAC(controller.k8sclient, testAC1.DeepCopy(), testAC2.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.Create(testCtx, testPVC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.CreateCR(testCtx, testACR1.Name, testACR1.DeepCopy())
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name, testPVC1Name, true, false)
				vol      = &vcrd.Volume{}
			)

			go testutils.VolumeReconcileImitation(controller.k8sclient, "req1", testNs, apiV1.Created)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())

			err = controller.k8sclient.ReadCR(context.Background(), "req1", testNs, vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.CSIStatus).To(Equal(apiV1.Created))
			Expect(vol.Spec.Mode).To(Equal(apiV1.ModeRAW))
			Expect(vol.Labels[k8s.AppLabelKey]).To(Equal(testApp))
		})
		It("Volume is created successfully (Block, Part)", func() {
			err := testutils.AddAC(controller.k8sclient, testAC1.DeepCopy(), testAC2.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.Create(testCtx, testPVC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.CreateCR(testCtx, testACR1.Name, testACR1.DeepCopy())
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name, testPVC1Name, true, true)
				vol      = &vcrd.Volume{}
			)

			go testutils.VolumeReconcileImitation(controller.k8sclient, "req1", testNs, apiV1.Created)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())

			err = controller.k8sclient.ReadCR(context.Background(), "req1", testNs, vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.CSIStatus).To(Equal(apiV1.Created))
			Expect(vol.Spec.Mode).To(Equal(apiV1.ModeRAWPART))
			Expect(vol.Labels[k8s.AppLabelKey]).To(Equal(testApp))
		})
		It("Volume CR has already exists", func() {
			uuid := "uuid-1234"
			capacity := int64(1024 * 42)

			req := getCreateVolumeRequest(uuid, capacity, testNode4Name, testPVC1Name, false, false)
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
					CSIStatus: apiV1.Created,
				}})
			Expect(err).To(BeNil())

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(resp).NotTo(BeNil())
			Expect(err).To(BeNil())
			Expect(resp.Volume.VolumeId).To(Equal(uuid))
			Expect(resp.Volume.CapacityBytes).To(Equal(int64(1024 * 60)))
		})
		It("Volume is created with supported option", func() {
			err := testutils.AddAC(controller.k8sclient, testAC1.DeepCopy(), testAC2.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.Create(testCtx, testPVC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.CreateCR(testCtx, testACR1.Name, testACR1.DeepCopy())
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name, testPVC1Name, false, false, "noatime")
				vol      = &vcrd.Volume{}
			)

			go testutils.VolumeReconcileImitation(controller.k8sclient, "req1", testNs, apiV1.Created)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(err).To(BeNil())
			Expect(resp).ToNot(BeNil())

			err = controller.k8sclient.ReadCR(context.Background(), "req1", testNs, vol)
			Expect(err).To(BeNil())
			Expect(vol.Spec.CSIStatus).To(Equal(apiV1.Created))
			Expect(vol.Spec.Mode).To(Equal(apiV1.ModeFS))
			Expect(vol.Labels[k8s.AppLabelKey]).To(Equal(testApp))
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
				APIVersion: apiV1.APIV1Version,
			},
			Spec: api.Volume{
				Id:     uuid,
				NodeId: node,
			}})
		Expect(err).To(BeNil())
		println("DONE")
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
		It("Node service mark volume as Failed", func() {
			var (
				volumeID  = "volume-id-2222"
				volumeCrd = &vcrd.Volume{}
				err       error
			)
			// create volume crd to delete
			volumeCrd = controller.k8sclient.ConstructVolumeCR(volumeID, testNs, testApp, api.Volume{Id: volumeID, CSIStatus: apiV1.Created})
			err = controller.k8sclient.CreateCR(testCtx, volumeID, volumeCrd)
			Expect(err).To(BeNil())
			fillCache(controller, volumeID, testNs)

			go testutils.VolumeReconcileImitation(controller.k8sclient, volumeCrd.Spec.Id, testNs, apiV1.Failed)
			resp, err := controller.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: volumeID})

			Expect(resp).To(BeNil())
			Expect(status.Code(err)).To(Equal(codes.Internal))

			err = controller.k8sclient.ReadCR(context.Background(), volumeID, testNs, volumeCrd)
			Expect(err).To(BeNil())
			Expect(volumeCrd.Spec.CSIStatus).To(Equal(apiV1.Failed))
		})
		It("Volume is created with unsupported option", func() {
			err := testutils.AddAC(controller.k8sclient, testAC1.DeepCopy(), testAC2.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.Create(testCtx, testPVC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.CreateCR(testCtx, testACR1.Name, testACR1.DeepCopy())
			Expect(err).To(BeNil())
			var (
				capacity = int64(1024 * 53)
				req      = getCreateVolumeRequest("req1", capacity, testNode1Name, testPVC1Name, false, false, "someOpt1", "someOpt2")
			)

			go testutils.VolumeReconcileImitation(controller.k8sclient, "req1", testNs, apiV1.Created)

			resp, err := controller.CreateVolume(context.Background(), req)
			Expect(err).ToNot(BeNil())
			Expect(resp).To(BeNil())
		})
	})

	Context("Success scenarios", func() {
		It("Volume CRD isn't found, consider that volume was removed", func() {
			vID := "some-id"
			dreq := &csi.DeleteVolumeRequest{VolumeId: vID}
			fillCache(controller, vID, testNs)
			resp, err := controller.DeleteVolume(context.Background(), dreq)
			Expect(resp).ToNot(BeNil())
			Expect(err).To(BeNil())
		})
		It("Volume is deleted successful, sc HDD", func() {
			var (
				namespace = controller.k8sclient.Namespace
				volumeID  = "volume-id-1111"
				volumeCrd = &vcrd.Volume{
					TypeMeta: k8smetav1.TypeMeta{
						Kind:       "Volume",
						APIVersion: apiV1.APIV1Version,
					},
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      volumeID,
						Namespace: namespace,
					},
					Spec: api.Volume{
						Id:        volumeID,
						NodeId:    node,
						Location:  testDriveLocation1,
						CSIStatus: apiV1.Created,
					},
				}
				err error
			)
			// create volume crd to delete
			err = controller.k8sclient.CreateCR(testCtx, volumeID, volumeCrd)
			Expect(err).To(BeNil())

			fillCache(controller, volumeCrd.Spec.Id, namespace)

			go testutils.VolumeReconcileImitation(controller.k8sclient, volumeCrd.Spec.Id, namespace, apiV1.Removed)
			resp, err := controller.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: volumeID})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())

			err = controller.k8sclient.ReadCR(context.Background(), volumeID, namespace, volumeCrd)
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
					CSIStatus:    apiV1.Created,
				}
				volumeCrd = vcrd.Volume{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      uuid,
						Namespace: controller.k8sclient.Namespace,
					},
					Spec: volume,
				}
				logicalVolumeGroup = api.LogicalVolumeGroup{
					Name:       testDriveLocation4,
					Node:       testNode2Name,
					Locations:  []string{testDriveLocation4},
					VolumeRefs: []string{uuid},
					Status:     apiV1.Creating,
					Size:       capacity,
				}
				lvgCR = lvgcrd.LogicalVolumeGroup{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      testDriveLocation4,
						Namespace: controller.k8sclient.Namespace,
					},
					Spec: logicalVolumeGroup,
				}
			)
			// create volume CR that should be deleted (created in BeforeEach)
			err = controller.k8sclient.CreateCR(testCtx, uuid, &volumeCrd)
			Expect(err).To(BeNil())

			// create LogicalVolumeGroup CR
			err = controller.k8sclient.CreateCR(testCtx, uuid, &lvgCR)
			Expect(err).To(BeNil())

			lvgCRs := &lvgcrd.LogicalVolumeGroupList{}
			err = controller.k8sclient.ReadList(testCtx, lvgCRs)
			Expect(err).To(BeNil())
			fillCache(controller, volumeCrd.Spec.Id, volumeCrd.Namespace)
			go testutils.VolumeReconcileImitation(controller.k8sclient, volumeCrd.Spec.Id, volumeCrd.Namespace, apiV1.Removed)
			resp, err := controller.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: uuid})
			Expect(resp).To(Equal(&csi.DeleteVolumeResponse{}))
			Expect(err).To(BeNil())

			//// check that there are no any volume CR (was removed)
			vList := vcrd.VolumeList{}
			err = controller.k8sclient.ReadList(testCtx, &vList)
			Expect(err).To(BeNil())
			Expect(len(vList.Items)).To(Equal(0))
		})
		It("Volume is deleted successful, LogicalVolumeGroup AC recreated", func() {
			removeAllCrds(controller.k8sclient) // remove CRs that was created in BeforeEach()
			fullLVGsizeVolume := testVolume
			fullLVGsizeVolume.Spec.StorageClass = apiV1.StorageClassHDDLVG
			fullLVGsizeVolume.Spec.CSIStatus = apiV1.Created

			// create volume CR that should be deleted
			err := controller.k8sclient.CreateCR(testCtx, testID, &fullLVGsizeVolume)
			Expect(err).To(BeNil())

			fillCache(controller, fullLVGsizeVolume.Spec.Id, fullLVGsizeVolume.Namespace)
			go testutils.VolumeReconcileImitation(controller.k8sclient, fullLVGsizeVolume.Spec.Id, fullLVGsizeVolume.Namespace, apiV1.Removed)
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
				csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
			}
		)

		svc := newSvc()

		caps, err = svc.ControllerGetCapabilities(context.Background(), &csi.ControllerGetCapabilitiesRequest{})
		Expect(err).To(BeNil())
		Expect(len(caps.Capabilities)).To(Equal(len(expectedCapabilitiesTypes)))

		currentCapabilitiesTypes := make([]csi.ControllerServiceCapability_RPC_Type, len(caps.Capabilities))
		for i := 0; i < len(caps.Capabilities); i++ {
			currentCapabilitiesTypes[i] = caps.Capabilities[i].GetRpc().GetType()
		}
		Expect(expectedCapabilitiesTypes).To(ConsistOf(currentCapabilitiesTypes))
	})
})

var _ = Describe("CSIControllerService health check", func() {
	It("Should failed health check", func() {
		svc := newSvc()
		check, err := svc.Check(testCtx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(check.Status).To(Equal(grpc_health_v1.HealthCheckResponse_NOT_SERVING))
	})
	It("Should success health check", func() {
		svc := newSvc()
		//To avoid error with state monitor getPodToNodeList function, because state monitor works in background of controller service
		err := svc.k8sclient.Create(testCtx,
			&v1.Node{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name: testNode2Name,
				},
				Status: v1.NodeStatus{
					Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}},
					Addresses: []v1.NodeAddress{
						{Type: v1.NodeHostName, Address: testNode1Name},
					},
				},
			},
		)
		Expect(err).To(BeNil())
		err = svc.k8sclient.Create(testCtx, &v1.Pod{
			TypeMeta: k8smetav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      "csi-baremetal-node",
				Namespace: "default",
			},
			Spec: v1.PodSpec{NodeName: testNode1Name},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{{Ready: true}},
			}})

		Expect(err).To(BeNil())
		check, err := svc.Check(testCtx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(check.Status).To(Equal(grpc_health_v1.HealthCheckResponse_SERVING))
	})
	It("Should failed health check, pod is unready", func() {
		svc := newSvc()
		//To avoid error with state monitor getPodToNodeList function, because state monitor works in background of controller service
		err := svc.k8sclient.Create(testCtx,
			&v1.Node{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name: testNode2Name,
				},
				Status: v1.NodeStatus{
					Addresses: []v1.NodeAddress{
						{Type: v1.NodeHostName, Address: testNode1Name},
					},
				},
			},
		)
		Expect(err).To(BeNil())
		err = svc.k8sclient.Create(testCtx, &v1.Pod{
			TypeMeta: k8smetav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      "baremetal-csi-node-0",
				Namespace: "default",
			},
			Spec: v1.PodSpec{NodeName: testNode1Name},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{{Ready: false}},
			}})
		Expect(err).To(BeNil())

		check, err := svc.Check(testCtx, &grpc_health_v1.HealthCheckRequest{})
		Expect(err).To(BeNil())
		Expect(check.Status).To(Equal(grpc_health_v1.HealthCheckResponse_NOT_SERVING))
	})
})

var _ = Describe("CSIControllerService ControllerExpandVolume", func() {
	var (
		controller *CSIControllerService
		node       = "node1"
		uuid       = "uuid-1234"
	)

	BeforeEach(func() {
		controller = newSvc()
		err := controller.k8sclient.CreateCR(context.Background(), uuid, &vcrd.Volume{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      uuid,
				Namespace: testNs,
			},
			TypeMeta: k8smetav1.TypeMeta{
				Kind:       "Volume",
				APIVersion: apiV1.APIV1Version,
			},
			Spec: api.Volume{
				Id:           uuid,
				NodeId:       node,
				CSIStatus:    apiV1.Created,
				Size:         1000,
				StorageClass: apiV1.StorageClassHDDLVG,
				Location:     testAC1.Spec.Location,
			}})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		removeAllCrds(controller.k8sclient)
	})

	Context("Fail scenarios", func() {
		It("Request doesn't contain volume ID", func() {
			req := &csi.ControllerExpandVolumeRequest{}
			resp, err := controller.ControllerExpandVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.InvalidArgument, "Volume name missing in request")))
		})
		It("Volume doesn't exist", func() {
			req := &csi.ControllerExpandVolumeRequest{VolumeId: "unknown", VolumeCapability: &csi.VolumeCapability{}}
			resp, err := controller.ControllerExpandVolume(context.Background(), req)
			Expect(resp).To(BeNil())
			Expect(err).To(Equal(status.Error(codes.NotFound, "Volume doesn't exist")))
		})
		It("Node service mark volume as Failed", func() {
			var (
				volumeCrd = &vcrd.Volume{}
				err       error
				capacity  = int64(1024)
			)
			err = controller.k8sclient.CreateCR(testCtx, testAC1Name, testAC1.DeepCopy())
			Expect(err).To(BeNil())
			// create volume crd to delete
			err = controller.k8sclient.ReadCR(testCtx, uuid, testNs, volumeCrd)
			Expect(err).To(BeNil())
			fillCache(controller, uuid, testNs)
			go testutils.VolumeReconcileImitation(controller.k8sclient, uuid, testNs, apiV1.Failed)
			resp, err := controller.ControllerExpandVolume(context.Background(),
				&csi.ControllerExpandVolumeRequest{
					VolumeId:         uuid,
					VolumeCapability: &csi.VolumeCapability{},
					CapacityRange:    &csi.CapacityRange{RequiredBytes: capacity},
				})

			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())
			fmt.Println(err.Error())
			err = controller.k8sclient.ReadCR(context.Background(), uuid, testNs, volumeCrd)
			Expect(err).To(BeNil())
			Expect(volumeCrd.Spec.CSIStatus).To(Equal(apiV1.Failed))
		})
		It("Expand failed", func() {
			var (
				err      error
				capacity = int64(1024)
			)
			volMock := mocks.VolumeOperationsMock{}
			volMock.On("ExpandVolume", mock.Anything, mock.Anything, mock.Anything).
				Return(errors.New("error"))
			controller.svc = &volMock
			resp, err := controller.ControllerExpandVolume(context.Background(),
				&csi.ControllerExpandVolumeRequest{
					VolumeId:         uuid,
					VolumeCapability: &csi.VolumeCapability{},
					CapacityRange:    &csi.CapacityRange{RequiredBytes: capacity},
				})

			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())

		})
		It("WaitStatus failed", func() {
			var (
				volumeCrd = &vcrd.Volume{}
				err       error
				capacity  = int64(1024)
			)
			err = controller.k8sclient.ReadCR(testCtx, uuid, testNs, volumeCrd)
			Expect(err).To(BeNil())

			volMock := mocks.VolumeOperationsMock{}
			volMock.On("ExpandVolume", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			volMock.On("WaitStatus", mock.Anything, mock.Anything, mock.Anything).
				Return(errors.New("error"))
			volMock.On("UpdateCRsAfterVolumeExpansion", mock.Anything, mock.Anything, mock.Anything)
			controller.svc = &volMock
			resp, err := controller.ControllerExpandVolume(context.Background(),
				&csi.ControllerExpandVolumeRequest{
					VolumeId:         uuid,
					VolumeCapability: &csi.VolumeCapability{},
					CapacityRange:    &csi.CapacityRange{RequiredBytes: capacity},
				})

			Expect(resp).To(BeNil())
			Expect(err).NotTo(BeNil())

		})
	})

	Context("Success scenarios", func() {
		It("Volume has the same size", func() {
			var (
				volumeCrd = &vcrd.Volume{}
				err       error
				capacity  = int64(1024)
			)
			// create volume crd to delete
			err = controller.k8sclient.ReadCR(testCtx, uuid, testNs, volumeCrd)
			Expect(err).To(BeNil())
			volumeCrd.Spec.Size = capacityplanner.AlignSizeByPE(capacity)
			err = controller.k8sclient.UpdateCR(testCtx, volumeCrd)
			Expect(err).To(BeNil())
			resp, err := controller.ControllerExpandVolume(context.Background(),
				&csi.ControllerExpandVolumeRequest{
					VolumeId:         uuid,
					VolumeCapability: &csi.VolumeCapability{},
					CapacityRange:    &csi.CapacityRange{RequiredBytes: capacity},
				})
			Expect(resp).ToNot(BeNil())
			Expect(err).To(BeNil())
			Expect(resp.CapacityBytes).To(Equal(int64(0)))
		})
		It("Volume is expanded successfully", func() {
			var (
				volumeCrd = &vcrd.Volume{}
				err       error
				capacity  = int64(1024)
			)
			fillCache(controller, uuid, testNs)
			err = controller.k8sclient.CreateCR(testCtx, testAC1Name, testAC1.DeepCopy())
			Expect(err).To(BeNil())
			err = controller.k8sclient.ReadCR(testCtx, uuid, testNs, volumeCrd)
			Expect(err).To(BeNil())
			go testutils.VolumeReconcileImitation(controller.k8sclient, uuid, testNs, apiV1.Resized)
			resp, err := controller.ControllerExpandVolume(context.Background(),
				&csi.ControllerExpandVolumeRequest{
					VolumeId:         uuid,
					VolumeCapability: &csi.VolumeCapability{},
					CapacityRange:    &csi.CapacityRange{RequiredBytes: capacity},
				})
			Expect(resp).ToNot(BeNil())
			Expect(err).To(BeNil())
			Expect(resp.CapacityBytes).To(Equal(capacityplanner.AlignSizeByPE(capacity)))

		})
	})
})

// create and instance of CSIControllerService with scheme for working with CRD
// create and instance of CSIControllerService with scheme for working with CRD
func newSvc() *CSIControllerService {
	kubeclient, err := k8s.GetFakeKubeClient(testNs, testLogger)
	if err != nil {
		panic(err)
	}
	nSvc := NewControllerService(kubeclient, testLogger, featureconfig.NewFeatureConfig())

	return nSvc
}

func fillCache(controller *CSIControllerService, volumeID, namespace string) {
	c := cache.NewMemCache()
	c.Set(volumeID, namespace)
	controller.svc = common.NewVolumeOperationsImpl(controller.k8sclient, testLogger, c, featureconfig.NewFeatureConfig())
}

// return CreateVolumeRequest based on provided parameters
func getCreateVolumeRequest(name string, cap int64, preferredNode string, claimName string, needBlock, needPart bool, mountFlags ...string) *csi.CreateVolumeRequest {
	parameters := map[string]string{
		util.ClaimNamespaceKey: testNs,
		util.ClaimNameKey:      claimName,
	}
	if needPart {
		parameters[RawPartModeKey] = RawPartModeValue
	}

	req := &csi.CreateVolumeRequest{
		Name:          name,
		CapacityRange: &csi.CapacityRange{RequiredBytes: cap},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		Parameters: parameters,
	}

	if !needBlock {
		accessType := &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{
				FsType:     string(fs.XFS),
				MountFlags: mountFlags,
			},
		}
		req.VolumeCapabilities[0].AccessType = accessType
	} else {
		accessType := &csi.VolumeCapability_Block{
			Block: &csi.VolumeCapability_BlockVolume{},
		}
		req.VolumeCapabilities[0].AccessType = accessType
	}

	if preferredNode != "" {
		req.AccessibilityRequirements = &csi.TopologyRequirement{
			Preferred: []*csi.Topology{
				{
					Segments: map[string]string{csibmnodeconst.NodeIDTopologyLabelKey: preferredNode},
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
