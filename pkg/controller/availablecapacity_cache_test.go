package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var crdCache = AvailableCapacityCache{items: make(map[string]*accrd.AvailableCapacity)}

const capacityID = "node_drive"

func TestCRDCache_Create(t *testing.T) {
	ns := "default"
	capacity := &accrd.AvailableCapacity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: "availablecapacity.dell.com/metav1",
		},
		ObjectMeta: metav1.ObjectMeta{
			//Currently capacityID is node id
			Name:      capacityID,
			Namespace: ns,
		},
		Spec: api.AvailableCapacity{
			Size:     1000,
			Type:     api.StorageClass_ANY,
			Location: "drive",
			NodeId:   "node",
		},
	}
	err := crdCache.Create(capacity, capacityID)
	assert.Nil(t, err)
	assert.Equal(t, crdCache.items[capacityID], capacity, "Capacities are not equal")
	err = crdCache.Create(capacity, capacityID)
	assert.NotNil(t, err)
}

func TestCRDCache_Get(t *testing.T) {
	capacity := crdCache.Get(capacityID)
	assert.Equal(t, crdCache.items[capacityID], capacity, "Capacities are not equal")
	capacity = crdCache.Get("id")
	assert.Nil(t, capacity)
}

func TestCRDCache_Update(t *testing.T) {
	capacity := crdCache.Get(capacityID)
	capacity.Spec.Size = 2000
	crdCache.Update(capacity, capacityID)
	assert.Equal(t, crdCache.items[capacityID], capacity, "Capacities are not equal")
}

func TestCRDCache_Delete(t *testing.T) {
	object := crdCache.items[capacityID]
	crdCache.Delete(capacityID)
	assert.NotContains(t, c.items, object)
	beforeDelete := crdCache.items
	crdCache.Delete(capacityID)
	assert.Equal(t, beforeDelete, crdCache.items)
}
