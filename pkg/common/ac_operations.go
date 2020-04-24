// Package common is for common operations with CSI resources such as AvailableCapacity or Volume
package common

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

// AvailableCapacityOperations is the interface for interact with AvailableCapacity CRs from Controller
type AvailableCapacityOperations interface {
	SearchAC(ctx context.Context, node string, requiredBytes int64, sc string) *accrd.AvailableCapacity
	DeleteIfEmpty(ctx context.Context, acLocation string) error
}

// AcSizeMinThresholdBytes means that if AC size becomes lower then AcSizeMinThresholdBytes that AC should be deleted
const AcSizeMinThresholdBytes = int64(base.MBYTE) // 1MB

// ACOperationsImpl is the basic implementation of AvailableCapacityOperations interface
type ACOperationsImpl struct {
	k8sClient *base.KubeClient
	log       *logrus.Entry
}

// NewACOperationsImpl is the constructor for ACOperationsImpl struct
// Receives an instance of base.KubeClient and logrus logger
// Returns an instance of ACOperationsImpl
func NewACOperationsImpl(k8sClient *base.KubeClient, l *logrus.Logger) *ACOperationsImpl {
	return &ACOperationsImpl{
		k8sClient: k8sClient,
		log:       l.WithField("component", "ACOperations"),
	}
}

// SearchAC searches appropriate available capacity and remove it's CR
// if SC is in LVM and there is no AC with such SC then LVG should be created based
// on non-LVM AC's and new AC should be created on point in LVG
// method shouldn't be used in separate goroutines without synchronizations.
// Receives golang context, node string which means the node where to find AC, required bytes for volume
// and storage class for created volume (For example HDD, HDDLVG, SSD, SSDLVG).
// Returns found AvailableCapacity CR instance
func (a *ACOperationsImpl) SearchAC(ctx context.Context,
	node string, requiredBytes int64, sc string) *accrd.AvailableCapacity {
	ll := a.log.WithFields(logrus.Fields{
		"method":        "SearchAC",
		"volumeID":      ctx.Value(base.RequestUUID),
		"requiredBytes": fmt.Sprintf("%.3fG", float64(requiredBytes)/float64(base.GBYTE)),
	})

	ll.Info("Search appropriate available ac")

	var (
		allocatedCapacity int64 = math.MaxInt64
		foundAC           *accrd.AvailableCapacity
		acList            = &accrd.AvailableCapacityList{}
		acNodeMap         map[string][]*accrd.AvailableCapacity
	)

	err := a.k8sClient.ReadList(context.Background(), acList)
	if err != nil {
		ll.Errorf("Unable to read Available Capacity list, error: %v", err)
		return nil
	}

	acNodeMap = a.acNodeMapping(acList.Items)

	if node == "" {
		node = a.balanceAC(acNodeMap, requiredBytes, sc)
	}

	ll.Infof("Search AvailableCapacity on node %s with size not less %d bytes with storage class %s",
		node, requiredBytes, sc)

	for _, ac := range acNodeMap[node] {
		if ac.Spec.Size < allocatedCapacity && ac.Spec.Size >= requiredBytes &&
			(sc == apiV1.StorageClassAny || sc == ac.Spec.StorageClass) {
			foundAC = ac
			allocatedCapacity = ac.Spec.Size
		}
	}

	if sc == apiV1.StorageClassHDDLVG || sc == apiV1.StorageClassSSDLVG {
		if foundAC != nil {
			// check whether LVG being deleted or no
			lvgCR := &lvgcrd.LVG{}
			err := a.k8sClient.ReadCR(context.Background(), foundAC.Spec.Location, lvgCR)
			if err == nil && lvgCR.DeletionTimestamp.IsZero() {
				return foundAC
			}
		}
		// if storageClass is related to LVG and there is no AC with that storageClass
		// search drive with subclass on which LVG will be creating
		subSC := apiV1.StorageClassHDD
		if sc == apiV1.StorageClassSSDLVG {
			subSC = apiV1.StorageClassSSD
		}
		ll.Infof("StorageClass is in LVG, search AC with subStorageClass %s", subSC)
		foundAC = a.SearchAC(ctx, node, requiredBytes, subSC)
		if foundAC == nil {
			return nil
		}
		ll.Infof("Got AC %v", foundAC)
		return a.recreateACToLVGSC(sc, foundAC)
	}

	return foundAC
}

// DeleteIfEmpty search AC by it's location and remove if it size is less then threshold
// Receives golang context and AC Location as a string (For example Location could be Drive uuid in case of HDD SC)
// Returns error if something went wrong
func (a *ACOperationsImpl) DeleteIfEmpty(ctx context.Context, acLocation string) error {
	var acList = accrd.AvailableCapacityList{}
	_ = a.k8sClient.ReadList(ctx, &acList)

	for _, ac := range acList.Items {
		if ac.Spec.Location == acLocation {
			if ac.Spec.Size < AcSizeMinThresholdBytes {
				return a.k8sClient.DeleteCR(ctx, &ac)
			}
			return nil
		}
	}

	return fmt.Errorf("unable to find AC by location %s", acLocation)
}

// acNodeMapping constructs map with key - nodeID(hostname), value - AC CRs based on Spec.NodeID field of AC
// Receives slice of AvailableCapacity custom resources
// Returns map of AvailableCapacities where key is nodeID
func (a *ACOperationsImpl) acNodeMapping(acs []accrd.AvailableCapacity) map[string][]*accrd.AvailableCapacity {
	var (
		acNodeMap = make(map[string][]*accrd.AvailableCapacity)
		node      string
	)

	for _, ac := range acs {
		node = ac.Spec.NodeId
		if _, ok := acNodeMap[node]; !ok {
			acNodeMap[node] = make([]*accrd.AvailableCapacity, 0)
		}
		acTmp := ac
		acNodeMap[node] = append(acNodeMap[node], &acTmp)
	}
	return acNodeMap
}

// balanceAC looks for a node with appropriate AC and choose node with maximum AC, return node
// Receives acNodeMap gathered from acNodeMapping method, size requested by a volume, and appropriate storage class
// Returns the most unloaded node according to input parameters
func (a *ACOperationsImpl) balanceAC(acNodeMap map[string][]*accrd.AvailableCapacity,
	size int64, sc string) (node string) {
	maxLen := 0
	for nodeID, acs := range acNodeMap {
		if len(acs) > maxLen {
			// ensure that there is at least one AC with size not less than requiredBytes and with the same SC
			for _, ac := range acs {
				if ac.Spec.Size >= size && (ac.Spec.StorageClass == sc || sc == apiV1.StorageClassAny) {
					node = nodeID
					maxLen = len(acs)
					break
				}
			}
		}
	}

	return node
}

// recreateACToLVGSC creates LVG(based on ACs), ensure it become ready,
// creates AC based on that LVG and removes provided ACs.
// Receives sc as string (HDDLVG or SSDLVG) and AvailableCapacities where LVG should be based
// Returns created AC or nil
func (a *ACOperationsImpl) recreateACToLVGSC(sc string, acs ...*accrd.AvailableCapacity) *accrd.AvailableCapacity {
	ll := a.log.WithField("method", "recreateACToLVGSC")

	lvgLocations := make([]string, len(acs))
	var lvgSize int64
	for i, ac := range acs {
		lvgLocations[i] = ac.Spec.Location
		lvgSize += ac.Spec.Size
	}

	var (
		err    error
		name   = uuid.New().String()
		apiLVG = api.LogicalVolumeGroup{
			Node:      acs[0].Spec.NodeId, // all ACs are from the same node
			Name:      name,
			Locations: lvgLocations,
			Size:      lvgSize,
			Status:    apiV1.Creating,
		}
	)

	// remove ACs at the first because if LVG creation fails some drives could be
	// corrupted and that mean that ACs based on that drive will not be working
	// if LVG creation fails and drives were not corrupted, ACs based on that drives
	// will be recreated by particular node manager
	for _, ac := range acs {
		if err = a.k8sClient.DeleteCR(context.Background(), ac); err != nil {
			ll.Errorf("Unable to remove AC %v, error: %v. Two ACs that and LVG have location in the same drive.",
				ac, err)
		}
	}

	// create LVG CR based on ACs
	lvg := a.k8sClient.ConstructLVGCR(name, apiLVG)
	if err = a.k8sClient.CreateCR(context.Background(), name, lvg); err != nil {
		ll.Errorf("Unable to create LVG CR: %v", err)
		return nil
	}
	ll.Infof("LVG %v was created. Wait until it become ready.", apiLVG)
	// here we should to wait until VG is reconciled by volumemgr
	ctx, cancelFn := context.WithTimeout(context.Background(), base.DefaultTimeoutForOperations)
	defer cancelFn()
	var newAPILVG *api.LogicalVolumeGroup
	if newAPILVG = a.waitUntilLVGWillBeCreated(ctx, name); newAPILVG == nil {
		if err = a.k8sClient.DeleteCR(context.Background(), lvg); err != nil {
			ll.Errorf("LVG creation failed and unable to remove LVG %v: %v", lvg.Spec, err)
		}
		return nil
	}

	// create new AC
	newACCRName := acs[0].Spec.NodeId + "-" + lvg.Name
	newACCR := a.k8sClient.ConstructACCR(newACCRName, api.AvailableCapacity{
		Location:     lvg.Name,
		NodeId:       acs[0].Spec.NodeId,
		StorageClass: sc,
		Size:         newAPILVG.Size,
	})
	if err = a.k8sClient.CreateCR(context.Background(), newACCRName, newACCR); err != nil {
		ll.Errorf("Unable to create AC %v, error: %v", newACCRName, err)
		return nil
	}

	ll.Infof("AC was created: %v", newACCR)
	return newACCR
}

// waitUntilLVGWillBeCreated checks LVG CR status during timeout provided in context
// Receives golang context with timeout and name of LVG
// Returns LVG.Spec if LVG.Spec.Status == created, or return nil instead
func (a *ACOperationsImpl) waitUntilLVGWillBeCreated(ctx context.Context, lvgName string) *api.LogicalVolumeGroup {
	ll := a.log.WithFields(logrus.Fields{
		"method":  "waitUntilLVGWillBeCreated",
		"lvgName": lvgName,
	})
	ll.Infof("Pulling LVG")

	var (
		lvg = &lvgcrd.LVG{}
		err error
	)

	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done and LVG still not become created, consider that it was failed")
			return nil
		case <-time.After(1 * time.Second):
			err = a.k8sClient.ReadCR(ctx, lvgName, lvg)
			switch {
			case err != nil:
				ll.Errorf("Unable to read LVG CR: %v", err)
			case lvg.Spec.Status == apiV1.Created:
				ll.Info("LVG was created")
				return &lvg.Spec
			case lvg.Spec.Status == apiV1.Failed:
				ll.Warn("LVG was reached FailedToCreate status")
				return nil
			}
		}
	}
}
