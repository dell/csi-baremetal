package controller

import (
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

var crdCache = AvailableCapacityCache{items: make(map[string]*accrd.AvailableCapacity)}

const capacityID = "node_drive"

func TestCRDCache_Create(t *testing.T) {
	capacity := &accrd.AvailableCapacity{
		TypeMeta: v1.TypeMeta{
			Kind:       "AvailableCapacity",
			APIVersion: "availablecapacity.dell.com/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			//Currently capacityID is node id
			Name:      capacityID,
			Namespace: namespace,
		},
		Spec: v1api.AvailableCapacity{
			Size:     1000,
			Type:     v1api.StorageClass_ANY,
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
