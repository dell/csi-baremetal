package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testCRDCache = AvailableCapacityCache{items: make(map[string]map[string]*accrd.AvailableCapacity)}

const (
	nodeID        = "node"
	driveLocation = "drive"
)

func TestCRDCache_Create(t *testing.T) {
	ns := "default"
	capacity := &accrd.AvailableCapacity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: "availablecapacity.dell.com/metav1",
		},
		ObjectMeta: metav1.ObjectMeta{
			//Currently capacityID is node id
			Name:      nodeID + "-" + driveLocation,
			Namespace: ns,
		},
		Spec: api.AvailableCapacity{
			Size:     1000,
			Type:     api.StorageClass_ANY,
			Location: "drive",
			NodeId:   "node",
		},
	}
	err := testCRDCache.Create(capacity, nodeID, driveLocation)
	assert.Nil(t, err)
	assert.Equal(t, testCRDCache.items[nodeID][driveLocation], capacity, "Capacities are not equal")
	err = testCRDCache.Create(capacity, nodeID, driveLocation)
	assert.NotNil(t, err)
}

func TestCRDCache_Get(t *testing.T) {
	capacity := testCRDCache.Get(nodeID, driveLocation)
	assert.Equal(t, testCRDCache.items[nodeID][driveLocation], capacity, "Capacities are not equal")
	capacity = testCRDCache.Get("nodeid", "location")
	assert.Nil(t, capacity)
}

func TestCRDCache_Update(t *testing.T) {
	capacity := testCRDCache.Get(nodeID, driveLocation)
	capacity.Spec.Size = 2000
	testCRDCache.Update(capacity, nodeID, driveLocation)
	assert.Equal(t, testCRDCache.items[nodeID][driveLocation], capacity, "Capacities are not equal")
}

func TestCRDCache_Delete(t *testing.T) {
	object := testCRDCache.items[nodeID][driveLocation]
	testCRDCache.Delete(nodeID, driveLocation)
	assert.NotContains(t, c.items, object)
	beforeDelete := testCRDCache.items
	testCRDCache.Delete(nodeID, driveLocation)
	assert.Equal(t, beforeDelete, testCRDCache.items)
}
