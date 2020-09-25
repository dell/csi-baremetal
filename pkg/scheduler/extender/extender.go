package extender

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api/v1"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	"github.com/dell/csi-baremetal/pkg/common"
)

// Extender holds http handlers for scheduler extender endpoints and implements logic for nodes filtering
// based on pod volumes requirements and Available Capacities
type Extender struct {
	k8sClient *k8s.KubeClient
	crHelper  *k8s.CRHelper
	// namespace in which Extender will be search Available Capacity
	namespace   string
	provisioner string
	// TODO: remove that key
	useACRs bool
	sync.Mutex
	logger *logrus.Entry
}

// NewExtender returns new instance of Extender struct
// TODO: remove useACRs flag
func NewExtender(logger *logrus.Logger, namespace, provisioner string, useACRs bool) (*Extender, error) {
	k8sClient, err := k8s.GetK8SClient()
	if err != nil {
		return nil, err
	}
	kubeClient := k8s.NewKubeClient(k8sClient, logger, namespace)
	return &Extender{
		k8sClient:   kubeClient,
		crHelper:    k8s.NewCRHelper(kubeClient, logger),
		provisioner: provisioner,
		logger:      logger.WithField("component", "Extender"),
		useACRs:     useACRs,
	}, nil
}

// FilterHandler extracts ExtenderArgs struct from req and writes ExtenderFilterResult to the w
func (e *Extender) FilterHandler(w http.ResponseWriter, req *http.Request) {
	sessionUUID := uuid.New().String()
	ll := e.logger.WithFields(logrus.Fields{
		"sessionUUID": sessionUUID,
		"method":      "FilterHandler",
	})
	ll.Infof("Processing request: %v", req)

	w.Header().Set("Content-Type", "application/json")
	resp := json.NewEncoder(w)

	var (
		extenderArgs schedulerapi.ExtenderArgs
		extenderRes  = &schedulerapi.ExtenderFilterResult{}
	)

	if err := json.NewDecoder(req.Body).Decode(&extenderArgs); err != nil {
		ll.Errorf("Unable to decode request body: %v", err)
		extenderRes.Error = err.Error()
		if err := resp.Encode(extenderRes); err != nil {
			ll.Errorf("Unable to write response %v: %v", extenderRes, err)
		}
		return
	}

	ll.Info("Filtering")
	ctxWithVal := context.WithValue(req.Context(), k8s.RequestUUID, sessionUUID)
	volumes, err := e.gatherVolumesByProvisioner(ctxWithVal, extenderArgs.Pod)
	if err != nil {
		extenderRes.Error = err.Error()
		if err := resp.Encode(extenderRes); err != nil {
			ll.Errorf("Unable to write response %v: %v", extenderRes, err)
		}
		return
	}
	ll.Debugf("Required volumes: %v", volumes)

	e.Lock()
	defer e.Unlock()
	matchedNodes, failedNodes, err := e.filter(ctxWithVal, extenderArgs.Nodes.Items, volumes)
	if err != nil {
		ll.Errorf("filter finished with error: %v", err)
		extenderRes.Error = err.Error()
	} else {
		ll.Infof("Construct response. Get %d nodes in request. Among them suitable nodes count is %d. Filtered out nodes - %v",
			len(extenderArgs.Nodes.Items), len(matchedNodes), failedNodes)
	}

	extenderRes.Nodes = &coreV1.NodeList{
		TypeMeta: extenderArgs.Nodes.TypeMeta,
		Items:    matchedNodes,
	}
	extenderRes.FailedNodes = failedNodes

	if err := resp.Encode(extenderRes); err != nil {
		ll.Errorf("Unable to write response %v: %v", extenderRes, err)
	}
}

// PrioritizeHandler assigns scores to the nodes
// todo not implemented
func (e *Extender) PrioritizeHandler(w http.ResponseWriter, req *http.Request) {
	panic("implement me")
}

// BindHandler does bind of a pod to specific node
// todo - not implemented. Was used for testing purposes ONLY (fault injection)!
func (e *Extender) BindHandler(w http.ResponseWriter, req *http.Request) {
	sessionUUID := uuid.New().String()
	ll := e.logger.WithFields(logrus.Fields{
		"sessionUUID": sessionUUID,
		"method":      "BindHandler",
	})
	ll.Infof("Processing request: %v", req)

	w.Header().Set("Content-Type", "application/json")
	resp := json.NewEncoder(w)

	var (
		extenderBindingArgs schedulerapi.ExtenderBindingArgs
		extenderBindingRes  = &schedulerapi.ExtenderBindingResult{}
	)

	if err := json.NewDecoder(req.Body).Decode(&extenderBindingArgs); err != nil {
		ll.Errorf("Unable to decode request body: %v", err)
		extenderBindingRes.Error = err.Error()
		if err := resp.Encode(extenderBindingRes); err != nil {
			ll.Errorf("Unable to write response %v: %v", extenderBindingRes, err)
		}
		return
	}

	extenderBindingRes.Error = "don't know how to use bind API"
	if err := resp.Encode(extenderBindingRes); err != nil {
		ll.Errorf("Unable to write response %v: %v", extenderBindingRes, err)
	}
}

// gatherVolumesByProvisioner search all volumes in pod' spec that should be provisioned
// by provisioner e.provisioner and construct genV1.Volume struct for each of such volume
func (e *Extender) gatherVolumesByProvisioner(ctx context.Context, pod *coreV1.Pod) ([]*genV1.Volume, error) {
	ll := e.logger.WithFields(logrus.Fields{
		"sessionUUID": ctx.Value(k8s.RequestUUID),
		"method":      "gatherVolumesByProvisioner",
		"pod":         pod.Name,
	})

	scs, err := e.scNameStorageTypeMapping(ctx)
	if err != nil {
		ll.Errorf("Unable to collect storage classes: %v", err)
		return nil, err
	}

	volumes := make([]*genV1.Volume, 0)
	for _, v := range pod.Spec.Volumes {
		// check whether there are Ephemeral volumes or no
		if v.CSI != nil {
			if v.CSI.Driver == e.provisioner {
				volume, err := e.constructVolumeFromCSISource(v.CSI)
				if err != nil {
					ll.Errorf("Unable to construct API Volume for Ephemeral volume: %v", err)
				}
				// need to apply any result for getting at leas amount of volumes
				volumes = append(volumes, volume)
			}
			continue
		}
		if v.PersistentVolumeClaim != nil {
			pvc := &coreV1.PersistentVolumeClaim{}
			err := e.k8sClient.Get(ctx,
				k8sCl.ObjectKey{Name: v.PersistentVolumeClaim.ClaimName, Namespace: pod.Namespace},
				pvc)
			if err != nil {
				ll.Errorf("Unable to read PVC %s in NS %s: %v. ", v.PersistentVolumeClaim.ClaimName, pod.Namespace, err)
				return nil, err
			}
			if pvc.Spec.StorageClassName == nil {
				continue
			}
			if _, ok := scs[*pvc.Spec.StorageClassName]; !ok {
				continue
			}
			if pvc.Status.Phase == coreV1.ClaimBound || pvc.Status.Phase == coreV1.ClaimLost {
				continue
			}
			if storageType, ok := scs[*pvc.Spec.StorageClassName]; ok {
				storageReq, ok := pvc.Spec.Resources.Requests[coreV1.ResourceStorage]
				if !ok {
					ll.Errorf("There is no key for storage resource for PVC %s", pvc.Name)
					storageReq = resource.Quantity{}
				}

				mode := ""
				if pvc.Spec.VolumeMode != nil {
					mode = string(*pvc.Spec.VolumeMode)
				}

				volumes = append(volumes, &genV1.Volume{
					Id:           pvc.Name,
					StorageClass: util.ConvertStorageClass(storageType),
					Size:         storageReq.Value(),
					Mode:         mode,
					Ephemeral:    false,
				})
			}
		}
	}
	return volumes, nil
}

// constructVolumeFromCSISource constructs genV1.Volume based on fields from coreV1.Volume.CSI
func (e *Extender) constructVolumeFromCSISource(v *coreV1.CSIVolumeSource) (vol *genV1.Volume, err error) {
	// if some parameters aren't parsed for some reason
	// empty volume will be returned in order count that volume
	vol = &genV1.Volume{
		StorageClass: v1.StorageClassAny,
		Ephemeral:    true,
	}

	sc, ok := v.VolumeAttributes[base.StorageTypeKey]
	if !ok {
		return vol, fmt.Errorf("unable to detect storage class from attributes %v", v.VolumeAttributes)
	}
	vol.StorageClass = util.ConvertStorageClass(sc)

	sizeStr, ok := v.VolumeAttributes[base.SizeKey]
	if !ok {
		return vol, fmt.Errorf("unable to detect size from attributes %v", v.VolumeAttributes)
	}

	size, err := util.StrToBytes(sizeStr)
	if err != nil {
		return vol, fmt.Errorf("unable to convert string %s to bytes: %v", sizeStr, err)
	}
	vol.Size = size

	return vol, nil
}

// filter is an algorithm for defining whether requested volumes could be provisioned on particular node or no
// nodes - list of node candidate, volumes - requested volumes
// returns: matchedNodes - list of nodes on which volumes could be provisioned
// failedNodesMap - represents the filtered out nodes, with node names and failure messages
func (e *Extender) filter(ctx context.Context, nodes []coreV1.Node, volumes []*genV1.Volume) (matchedNodes []coreV1.Node,
	failedNodesMap schedulerapi.FailedNodesMap, err error) {
	if len(volumes) == 0 {
		return nodes, failedNodesMap, err
	}

	// TODO: do not read all ACs and ACRs for each request: https://github.com/dell/csi-baremetal/issues/89
	// map[NodeID]map[StorageClass]map[AC.Name]*accrd.AvailableCapacity{}
	acByNodeAndSCMap, err := e.unreservedACByNodeAndSCMap()
	if err != nil {
		return matchedNodes, failedNodesMap, err
	}
	// map[StorageClass]*Volume
	scVolumeMapping := e.scVolumeMapping(volumes)

	matched := false
	// map[NodeId]map[*Volume]*AC
	nodeVolumeACs := map[string]map[*genV1.Volume]*accrd.AvailableCapacity{}
	for _, node := range nodes {
		nodeUID := string(node.UID)
		nodeVolumeACs[nodeUID] = map[*genV1.Volume]*accrd.AvailableCapacity{}
		matched = true
		for sc, scVolumes := range scVolumeMapping {
			volACMap := e.searchSuitableACs(acByNodeAndSCMap[nodeUID], sc, scVolumes)
			if volACMap == nil {
				matched = false
				break
			}
			for vol, ac := range volACMap {
				nodeVolumeACs[nodeUID][vol] = ac
			}
		}

		if matched {
			matchedNodes = append(matchedNodes, node)
		} else {
			delete(nodeVolumeACs, nodeUID)
			if failedNodesMap == nil {
				failedNodesMap = map[string]string{}
			}
			failedNodesMap[node.Name] = "Node doesn't contain required amount of AvailableCapacity"
		}
	}

	if e.useACRs {
		err = e.createACRs(ctx, nodeVolumeACs)
	}

	return matchedNodes, failedNodesMap, err
}

// createACRs create ACR CRs based on provided map, map has next structure: map[NodeId]map[*Volume]*AvailableCapacity
// at first map with keys *Volume and values - list of AC names is built based on nodeVolumeACMap
// then corresponding ACR CRs is created (based on map that was built on previous step), if error occurs during ACRs creating it will be returned,
// and method will try to remove previously create ACR if any
func (e *Extender) createACRs(ctx context.Context, nodeVolumeACMap map[string]map[*genV1.Volume]*accrd.AvailableCapacity) error {
	volumeReservationMap := make(map[*genV1.Volume][]string) // value - list of AC names
	for _, volumeACMap := range nodeVolumeACMap {
		for volume, ac := range volumeACMap {
			if _, ok := volumeReservationMap[volume]; !ok {
				volumeReservationMap[volume] = []string{ac.Name}
				continue
			}
			volumeReservationMap[volume] = append(volumeReservationMap[volume], ac.Name)
		}
	}

	// create ACR CR based node ACs
	var (
		createdACRs = make([]string, len(volumeReservationMap))
		index       = 0
		createErr   error
	)

	for v, acs := range volumeReservationMap {
		acrCR := e.k8sClient.ConstructACRCR(genV1.AvailableCapacityReservation{
			Name:         uuid.New().String(),
			StorageClass: v.StorageClass,
			Size:         v.Size,
			Reservations: acs,
		})
		if createErr = e.k8sClient.CreateCR(ctx, acrCR.Name, acrCR); createErr != nil {
			createErr = fmt.Errorf("unable to create ACR CR %v for volume %v: %v", acrCR.Spec, v, createErr)
			break
		}
		createdACRs[index] = acrCR.Name
		index++
	}

	if createErr == nil {
		return nil
	}

	// try to remove all created ACR
	for _, acrName := range createdACRs {
		if err := e.crHelper.DeleteObjectByName(ctx, acrName, &acrcrd.AvailableCapacityReservation{}); err != nil {
			e.logger.WithField("method", "createACRs").Errorf("Unable to remove ACR %s: %v", acrName, err)
		}
	}

	return createErr
}

// searchSuitableACs searches the most appropriate AC for each volume from volumes in scACMap
// scACMap - map that represents available capacities and has next structure: map[StorageClass][AC.Name]*AC
func (e *Extender) searchSuitableACs(scACMap map[string]map[string]*accrd.AvailableCapacity,
	sc string, volumes []*genV1.Volume) map[*genV1.Volume]*accrd.AvailableCapacity { // list based on which ACR will be created
	resultingACs := make(map[*genV1.Volume]*accrd.AvailableCapacity, len(volumes))
	for _, volume := range volumes {
		subSC := util.GetSubStorageClass(sc)
		LVM := util.IsStorageClassLVG(sc)

		if len(scACMap[sc]) == 0 &&
			len(scACMap[subSC]) == 0 &&
			sc != v1.StorageClassAny {
			return nil
		}
		// make copy for temp transformations
		size := volume.GetSize()

		if LVM {
			// TODO: use non default PE size - https://github.com/dell/csi-baremetal/issues/85
			size = common.AlignSizeByPE(size)
		}
		var ac *accrd.AvailableCapacity
		ac = e.searchClosestAC(scACMap[sc], size)
		if ac == nil {
			if LVM {
				// for the new lvg we need some extra space
				size += common.LvgDefaultMetadataSize

				// search AC in sub storage class
				ac = e.searchClosestAC(scACMap[subSC], size)
			} else if sc == v1.StorageClassAny {
				for _, acs := range scACMap {
					ac = e.searchClosestAC(acs, size)
					if ac != nil {
						break
					}
				}
			}

			if ac == nil {
				// as soon as for some volume in some SC there are no any AC - consider
				// that whole volumes suite couldn't be provisioned based on available capacities
				return nil
			}
		}
		// here ac != nil
		vol := volume
		resultingACs[vol] = ac
		if ac.Spec.StorageClass != sc { // sc relates to LVG or sc == ANY
			if util.IsStorageClassLVG(ac.Spec.StorageClass) || LVM {
				if LVM {
					// remove AC with subSC
					delete(scACMap[subSC], ac.Name)
					ac.Spec.StorageClass = sc // e.g. HDD -> HDDLVG
					if _, ok := scACMap[sc]; !ok {
						scACMap[sc] = map[string]*accrd.AvailableCapacity{}
					}
					scACMap[sc][ac.Name] = ac
				}
				ac.Spec.Size -= size
			} else {
				// sc == ANY && ac.Spec.StorageClass doesn't relate to LVG
				delete(scACMap[ac.Spec.StorageClass], ac.Name)
			}
		} else {
			if LVM {
				ac.Spec.Size -= size
			} else {
				delete(scACMap[sc], ac.Name)
			}
		}
	}

	return resultingACs
}

// searchClosestAC search AC that match all requirements from volume (size)
func (e *Extender) searchClosestAC(acs map[string]*accrd.AvailableCapacity, size int64) *accrd.AvailableCapacity {
	var (
		maxSize  int64 = math.MaxInt64
		pickedAC *accrd.AvailableCapacity
	)

	for _, ac := range acs {
		if ac.Spec.Size >= size && ac.Spec.Size < maxSize {
			pickedAC = ac
			maxSize = ac.Spec.Size
		}
	}
	return pickedAC
}

// scNameStorageTypeMapping reads k8s storage class resources and collect map with key storage class name
// and value .parameters.storageType for that sc, collect only sc that have provisioner e.provisioner
func (e *Extender) scNameStorageTypeMapping(ctx context.Context) (map[string]string, error) {
	scs := storageV1.StorageClassList{}

	if err := e.k8sClient.List(ctx, &scs); err != nil {
		return nil, err
	}

	scNameTypeMap := map[string]string{}
	for _, sc := range scs.Items {
		if sc.Provisioner == e.provisioner {
			scNameTypeMap[sc.Name] = strings.ToUpper(sc.Parameters[base.StorageTypeKey])
		}
	}
	if len(scNameTypeMap) == 0 {
		return nil, fmt.Errorf("there are no any storage classes with provisioner %s", e.provisioner)
	}
	return scNameTypeMap, nil
}

/**
	scVolumeMapping constructs map with next structure:
	map[StorageClass][]*Volume
	{
		StorageClass1: [volume1, ..., volumeN]
		.............
	}
**/
func (e *Extender) scVolumeMapping(volumes []*genV1.Volume) map[string][]*genV1.Volume {
	var scVolumeMap = map[string][]*genV1.Volume{}
	for _, v := range volumes {
		sc := v.StorageClass
		if _, ok := scVolumeMap[sc]; !ok {
			scVolumeMap[sc] = []*genV1.Volume{v}
		} else {
			scVolumeMap[sc] = append(scVolumeMap[sc], v)
		}
	}
	return scVolumeMap
}

/**
	construct map based on list of ACs and list of ACRs,
	if there is ACR that points on some AC that AC will not appeared in resulting map

	resulting map has next structure:
	map[NodeID]map[StorageClass]map[AC.Name]*accrd.AvailableCapacity{}
	{
		NodeID_1: {
			StorageClass_1: {
				AC1Name: ACCRD_1,
				ACnName: ACCRD_n
			},
			StorageClass_M: {
				AC1Name: ACCRD_1,
				ACkName: ACCRD_k
			},
		NodeID_l: {
			...................
		}
	}
**/
func (e *Extender) unreservedACByNodeAndSCMap() (map[string]map[string]map[string]*accrd.AvailableCapacity, error) {
	var (
		acList  = &accrd.AvailableCapacityList{}
		acrList = &acrcrd.AvailableCapacityReservationList{}
		err     error
	)

	if err = e.k8sClient.ReadList(context.Background(), acList); err != nil {
		return nil, fmt.Errorf("unable to read AvailableCapacity list: %v", err)
	}
	if err = e.k8sClient.ReadList(context.Background(), acrList); err != nil {
		return nil, fmt.Errorf("unable to read AvailableCapacityReservation list: %v", err)
	}

	var (
		acByNodeAndSCMap = map[string]map[string]map[string]*accrd.AvailableCapacity{}
		// key - AC name, holds AC names that were reserved (are in ACR.Spec.Locations)
		reservedAC = map[string]struct{}{}
	)

	// fill reservedAC map
	for _, acr := range acrList.Items {
		for _, acName := range acr.Spec.Reservations {
			reservedAC[acName] = struct{}{}
		}
	}

	for _, ac := range acList.Items {
		if _, ok := reservedAC[ac.Name]; ok {
			// that AC was reserved before
			continue
		}
		node := ac.Spec.NodeId
		if _, ok := acByNodeAndSCMap[node]; !ok {
			acByNodeAndSCMap[node] = map[string]map[string]*accrd.AvailableCapacity{}
		}
		sc := ac.Spec.StorageClass
		ac := ac // ac uses in range and represent different value on each iteration but we need to put pointer in map
		if _, ok := acByNodeAndSCMap[node][sc]; !ok {
			acByNodeAndSCMap[node][sc] = map[string]*accrd.AvailableCapacity{ac.Name: &ac}
		} else {
			acByNodeAndSCMap[node][sc][ac.Name] = &ac
		}
	}

	return acByNodeAndSCMap, nil
}
