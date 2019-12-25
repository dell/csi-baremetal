package storagemanager

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	r "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/rest"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = BeforeSuite(func() {
	logrus.Info("Activating httpmock")
	httpmock.Activate()
})

var _ = AfterSuite(func() {
	logrus.Info("Deactivating httpmock")
	// print mock server statistic
	for k, v := range httpmock.GetCallCountInfo() {
		logrus.Infof("%s - %d", k, v)
	}
	httpmock.Deactivate()
})

func TestControllerSpec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LVMRestCommunicator Testing")
}

var st = make(StorageTopology, 1)

var _ = Describe("LVMVolumeManager testing", func() {
	var restUrl1 = "http://10.10.10.10:9999"
	var node1 = "node1"
	var lv1 = &lvm.LogicalVolume{
		Name:   "lv01",
		VGName: "csivg",
		LVSize: "10G",
	}
	var vg1 = &lvm.VolumeGroup{
		Name:         "csivg",
		SizeInGb:     float64(100),
		FreeSizeInGb: float64(80),
		DiskFilter:   nil,
	}
	var fs = r.NewFakeServer(restUrl1)
	fs.PrepareVGRespondersCode200(vg1)
	fs.PrepareLVRespondersCode200(lv1)

	var LVMVM = &LVMVolumeManager{
		LVMTopology: make(StorageTopology, 1),
		NodesCommunicators: map[string]*LVMRestCommunicator{
			node1: {
				client: &r.RClient{
					URL:        restUrl1,
					HTTPClient: &http.Client{Timeout: time.Second * 30},
				},
			},
		},
		Initialized: true,
	}

	Context("Check name generation", func() {
		It("Check generation", func() {
			ng := nameGen{val: 2}
			expectedName := "lv03"
			generatedName := ng.GetName()
			Expect(generatedName).To(Equal(expectedName))
			ng = nameGen{val: 0}
			expectedName = "lv01"
			generatedName = ng.GetName()
			Expect(generatedName).To(Equal(expectedName))
		})
	})

	Context("GetStorageTopology testing", func() {
		It("Interface should cast to an VolumeGroup object for each node", func() {
			st = LVMVM.GetStorageTopology(true)
			for _, value := range st {
				_, ok := value.(lvm.VolumeGroup)
				Expect(ok).To(Equal(true))
			}
		})
	})

	Context("LVMRestCommunicator.GetNodeStorageInfo testing", func() {
		It("Should send GET /vg1 and got interface that cast to VolumeGroup object", func() {
			// httpmock.GetCallCountInfo() returns map with key "REQUEST_TYPE ENDPOINT" and value - integer (count of calls)
			// for GET vg key will look like: "GET http://SOME_IP:SOME_PORT/vg"
			getCallsBefore := httpmock.GetCallCountInfo()[fmt.Sprintf("GET %s", fs.Endpoints["vg"])]
			nsi, err := LVMVM.NodesCommunicators[node1].GetNodeStorageInfo()
			Expect(err).To(BeNil())
			_, ok := nsi.(lvm.VolumeGroup)
			Expect(ok).To(Equal(true))
			getCallsAfter := httpmock.GetCallCountInfo()[fmt.Sprintf("GET %s", fs.Endpoints["vg"])]
			Expect(getCallsAfter).To(Equal(getCallsBefore + 1))
		})
	})

	Context("LVMRestCommunicator.PrepareVolumeOnNode testing", func() {
		It("Should send PUT /lv1", func() {
			getCallsBefore := httpmock.GetCallCountInfo()[fmt.Sprintf("PUT %s", fs.Endpoints["lv"])]
			vi := VolumeInfo{Name: lv1.Name}
			volumeID, err := LVMVM.NodesCommunicators[node1].PrepareVolumeOnNode(vi)
			Expect(err).To(BeNil())
			expectedVolumeID := fmt.Sprintf("%s_%s", vg1.Name, vi.Name)
			Expect(volumeID).To(Equal(expectedVolumeID))
			getCallsAfter := httpmock.GetCallCountInfo()[fmt.Sprintf("PUT %s", fs.Endpoints["lv"])]
			Expect(getCallsAfter).To(Equal(getCallsBefore + 1))
		})
	})

	Context("LVMRestCommunicator.ReleaseVolumeOnNode testing", func() {
		It("Should send DELETE /lv1", func() {
			getCallsBefore := httpmock.GetCallCountInfo()[fmt.Sprintf("DELETE %s", fs.Endpoints["lv"])]
			err := LVMVM.NodesCommunicators[node1].ReleaseVolumeOnNode(VolumeInfo{
				Name:       "test-vg1/lv01",
				CapacityGb: float64(10),
			})
			Expect(err).To(BeNil())
			getCallsAfter := httpmock.GetCallCountInfo()[fmt.Sprintf("DELETE %s", fs.Endpoints["lv"])]
			Expect(getCallsAfter).To(Equal(getCallsBefore + 1))
		})
	})

	Context("LVMStorageManager.PrepareVolume testing", func() {
		It("Happy pass with preferredNode", func() {
			node, volumeID, err := LVMVM.PrepareVolume(float64(10), node1)
			expectedVolumeID := fmt.Sprintf("%s_%s_%s", node1, vg1.Name, lv1.Name)
			Expect(err).To(BeNil())
			Expect(node).To(Equal(node1))
			Expect(volumeID).To(Equal(expectedVolumeID))
		})
		It("Rest server returned 500 on PUT /lv", func() {
			fs.PrepareLVRespondersCode500()
			node, volumeID, err := LVMVM.PrepareVolume(float64(10), node1)
			Expect(err).NotTo(BeNil())
			Expect(node).To(Equal(""))
			Expect(volumeID).To(Equal(""))
		})
	})

	Context("LVMStorageManager.Release testing", func() {
		It("Happy pass", func() {
			fs.PrepareLVRespondersCode200(lv1)
			volumeID := fmt.Sprintf("%s_%s_%s", node1, vg1.Name, lv1.Name)
			err := LVMVM.ReleaseVolume(node1, volumeID)
			Expect(err).To(BeNil())
		})
		It("Rest server returned 500 on DELETE /lv", func() {
			fs.PrepareLVRespondersCode500()
			volumeID := fmt.Sprintf("%s_%s_%s", node1, vg1.Name, lv1.Name)
			err := LVMVM.ReleaseVolume(node1, volumeID)
			Expect(err).To(BeNil())
		})
	})
})
