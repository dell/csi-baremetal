package controller

import (
	"sync"

	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//AvailableCapacityCache store AvailableCapacity CRD for controller service
type AvailableCapacityCache struct {
	//key - Node ID, value - map with key - drive location (S/N for hdd/sdd) and value - pointer to the available capacity obj
	items map[string]map[string]*accrd.AvailableCapacity
	sync.RWMutex
	log *logrus.Entry
}

func (c *AvailableCapacityCache) SetLogger(logger *logrus.Logger) {
	c.log = logger.WithField("component", "AvailableCapacityCache")
}

func (c *AvailableCapacityCache) Get(nodeID string, location string) *accrd.AvailableCapacity {
	c.RLock()
	defer c.RUnlock()
	if c.items[nodeID] == nil {
		logrus.Infof("Available capacity %s, %s is not found in items", nodeID, location)
		return nil
	}
	crd, ok := c.items[nodeID][location]
	if ok {
		logrus.Infof("Available capacity %s, %s is found in items", nodeID, location)
	} else {
		logrus.Infof("Available capacity %s, %s is not found in items", nodeID, location)
	}
	return crd
}

func (c *AvailableCapacityCache) Create(obj *accrd.AvailableCapacity, nodeID string, location string) error {
	c.Lock()
	defer c.Unlock()
	if c.items[nodeID] == nil {
		c.items[nodeID] = make(map[string]*accrd.AvailableCapacity)
	}
	if _, ok := c.items[nodeID][location]; ok {
		logrus.Errorf("AvailableCapacity %s, %s already exists in items", nodeID, location)
		return status.Errorf(codes.AlreadyExists, "AvailableCapacity with the same id: %s, %s already exist", nodeID, location)
	}
	c.items[nodeID][location] = obj
	logrus.Infof("AvailableCapacity %s, %s is added to items", nodeID, location)
	return nil
}

func (c *AvailableCapacityCache) Update(obj *accrd.AvailableCapacity, nodeID string, location string) {
	c.Lock()
	defer c.Unlock()
	if c.items[nodeID] != nil {
		c.items[nodeID][location] = obj
		logrus.Infof("AvailableCapacity %s, %s is added to items", nodeID, location)
	}
}

func (c *AvailableCapacityCache) Delete(nodeID string, location string) {
	c.Lock()
	defer c.Unlock()
	if c.items[nodeID] != nil {
		delete(c.items[nodeID], location)
	}
}
