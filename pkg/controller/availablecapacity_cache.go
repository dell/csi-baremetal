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
	items map[string]*accrd.AvailableCapacity
	sync.RWMutex
	log *logrus.Entry
}

func (c *AvailableCapacityCache) SetLogger(logger *logrus.Logger) {
	c.log = logger.WithField("component", "AvailableCapacityCache")
}

func (c *AvailableCapacityCache) Get(id string) *accrd.AvailableCapacity {
	c.RLock()
	defer c.RUnlock()
	crd, ok := c.items[id]
	if ok {
		logrus.Infof("Available capacity %s is found in items", id)
	} else {
		logrus.Infof("Available capacity %s is not found in items", id)
	}
	return crd
}

func (c *AvailableCapacityCache) Create(obj *accrd.AvailableCapacity, id string) error {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.items[id]; ok {
		logrus.Errorf("AvailableCapacity %s already exists in items", id)
		return status.Errorf(codes.AlreadyExists, "AvailableCapacity with the same id: %s already exist", id)
	}
	c.items[id] = obj
	logrus.Infof("AvailableCapacity %s is added to items", id)
	return nil
}

func (c *AvailableCapacityCache) Update(obj *accrd.AvailableCapacity, id string) {
	c.Lock()
	defer c.Unlock()
	c.items[id] = obj
	logrus.Infof("AvailableCapacity %s is added to items", id)
}

func (c *AvailableCapacityCache) Delete(id string) {
	c.Lock()
	defer c.Unlock()
	delete(c.items, id)
}
