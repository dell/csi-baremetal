package unit

import (
	"testing"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/driver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDriver(t *testing.T) {
	// This is not a unit test. Need to move it
	//TODO-8110 Investigate how to run driver properly
	/*go exec.Command("./../../build/_output/baremetal_csi", "--startrest=false", "--nodeid=test").Run()

	config := &sanity.Config{
		TargetPath:  "/tmp/target_path",
		StagingPath: "/tmp/staging_path",
		Address:     "unix:///tmp/csi.sock",
	}
	//TODO-8110 Tests fail. Because of REST server that monitors disks though k8s API. Driver doesn't work without k8s cluster
	sanity.Test(t, config)*/
	RegisterFailHandler(Fail)
	RunSpecs(t, "Disk Allocation Spec")
}

var _ = Describe("Allocator", func() {
	var node = "localhost"
	var requestedCapacity int64
	var disk util.HalDisk
	disk.Capacity = "128G"
	disk.Path = "/dev/sdb"
	disk.PartitionCount = 0

	var disk2 util.HalDisk
	disk2.Capacity = "16G"
	disk2.Path = "/dev/sdc"
	disk2.PartitionCount = 0

	var disk3 util.HalDisk
	disk3.Capacity = "20G"
	disk3.Path = "/dev/sdd"
	disk3.PartitionCount = 0

	var disk4 util.HalDisk
	disk4.Capacity = "311.8G"
	disk4.Path = "/dev/sde"
	disk4.PartitionCount = 0


	var NodeAllocatedDisks = make(map[string]map[util.HalDisk]bool)
	NodeAllocatedDisks[node] = make(map[util.HalDisk]bool)
	NodeAllocatedDisks[node][disk] = false
	NodeAllocatedDisks[node][disk2] = false
	NodeAllocatedDisks[node][disk3] = false
	NodeAllocatedDisks[node][disk4] = false

	var volumeID string
	var nodeID string
	var capacity int64

	Context("Required bytes", func() {
		It("First disk must be allocated", func() {
			// allocate first disk
			requestedCapacity = 100 * (1024 * 1024 * 1024) // 100Gi
			capacity, nodeID, volumeID = driver.AllocateDisk(NodeAllocatedDisks, node, requestedCapacity)

			Expect(capacity).Should(BeNumerically(">=", requestedCapacity))
			Expect(volumeID).To(Equal(node + "_" + disk.Path))
			Expect(nodeID).To(Equal(node))
		})

		It("Second disk must be allocated", func() {
			// allocate second disk
			requestedCapacity = 10 * (1024 * 1024 * 1024) // 10Gi
			capacity, nodeID, volumeID = driver.AllocateDisk(NodeAllocatedDisks, node, requestedCapacity)

			Expect(capacity).Should(BeNumerically(">=", requestedCapacity))
			Expect(volumeID).To(Equal(node + "_" + disk2.Path))
			Expect(nodeID).To(Equal(node))
		})

		It("Third disk allocation must fail", func() {
			// no available resources to allocate
			requestedCapacity = 500 * (1024 * 1024 * 1024) // 50Gi
			capacity, nodeID, volumeID = driver.AllocateDisk(NodeAllocatedDisks, node, requestedCapacity)

			Expect(capacity).Should(BeNumerically("==", 0))
		})
	})
})