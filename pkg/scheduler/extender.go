package scheduler

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
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

// Extender holds http handlers for scheduler extender endpoints and implements logic for nodes filtering
// based on pod volumes requirements and Available Capacities
type Extender struct {
	k8sClient *k8s.KubeClient
	// namespace in which Extender will be search Available Capacity
	namespace   string
	provisioner string
	sync.Mutex
	logger *logrus.Entry
}

// NewExtender returns new instance of Extender struct
func NewExtender(logger *logrus.Logger, namespace, provisioner string) (*Extender, error) {
	k8sClient, err := k8s.GetK8SClient()
	if err != nil {
		return nil, err
	}
	kubeClient := k8s.NewKubeClient(k8sClient, logger, namespace)
	return &Extender{
		k8sClient:   kubeClient,
		provisioner: provisioner,
		logger:      logger.WithField("component", "Extender"),
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
	matchedNodes, failedNodes, err := e.filter(extenderArgs.Nodes.Items, volumes)
	if err != nil {
		ll.Errorf("filter finished with error: %v", err)
		extenderRes.Error = err.Error()
	}
	if len(matchedNodes) == 0 {
		ll.Warn("No one node match requested volumes")
	} else {
		ll.Infof("Construct response. There are acceptance nodes. "+
			"Nodes that don't match requested volumes: %v", failedNodes)
	}

	extenderRes.Nodes = &coreV1.NodeList{
		TypeMeta: extenderArgs.Nodes.TypeMeta,
		Items:    matchedNodes,
	}
	extenderRes.FailedNodes = failedNodes

	if err := resp.Encode(extenderRes); err != nil {
		e.logger.WithField("method", "encodeResults").
			Errorf("Unable to write response %v: %v", extenderRes, err)
	}
}

// gatherVolumesByProvisioner search all volumes in pod' spec that should be provisioned
// by provisioner that match provisionerMask and construct genV1.Volume struct for each of such volume
func (e *Extender) gatherVolumesByProvisioner(ctx context.Context, pod *coreV1.Pod) ([]*genV1.Volume, error) {
	ll := e.logger.WithFields(logrus.Fields{
		"sessionUUID": ctx.Value(k8s.RequestUUID),
		"method":      "gatherVolumesByProvisioner",
		"pod":         pod.Name,
	})
	ll.Debug("Processing ...")

	scs, err := e.scNameStorageTypeMapping(ctx)
	if err != nil {
		ll.Errorf("Unable to collect storage classes: %v", err)
		return nil, err
	}

	volumes := make([]*genV1.Volume, 0)
	for _, v := range pod.Spec.Volumes {
		e.logger.Tracef("Inspecting pod volume %+v", v)
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
func (e *Extender) filter(nodes []coreV1.Node, volumes []*genV1.Volume) (matchedNodes []coreV1.Node,
	failedNodesMap schedulerapi.FailedNodesMap, err error) {
	/**
	represent volumes as a next structure:
	map[StorageClass][]*Volume
	{
		StorageClass1: [volume1, ..., volumeN]
	    .............
	}
	**/
	var scVolumeMap = map[string][]*genV1.Volume{}
	for _, v := range volumes {
		sc := v.StorageClass
		if _, ok := scVolumeMap[sc]; !ok {
			scVolumeMap[sc] = []*genV1.Volume{v}
		} else {
			scVolumeMap[sc] = append(scVolumeMap[sc], v)
		}
	}

	var acList = &accrd.AvailableCapacityList{}
	if err = e.k8sClient.ReadList(context.Background(), acList); err != nil {
		err = fmt.Errorf("unable to read AvailableCapacity list: %v", err)
		return
	}

	/**
	construct map with next structure:
	map[NodeID]map[StorageClass]map[AC.Name]accrd.AvailableCapacity{}
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
	var acByNodeAndSCMap = map[string]map[string]map[string]*accrd.AvailableCapacity{}
	for _, ac := range acList.Items {
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

	matched := false
	for _, node := range nodes {
		matched = true
		nodeID := string(node.UID)
		for sc, scVolumes := range scVolumeMap {
			for _, volume := range scVolumes {
				subSC := util.GetSubStorageClass(sc) // returns empty string for non LVM storage classes
				forLVM := util.IsStorageClassLVG(sc)

				if len(acByNodeAndSCMap[nodeID][sc]) == 0 &&
					len(acByNodeAndSCMap[nodeID][subSC]) == 0 &&
					sc != v1.StorageClassAny {
					matched = false
					goto CheckMatched
				}

				var ac *accrd.AvailableCapacity
				ac = e.searchClosestAC(acByNodeAndSCMap[nodeID][sc], volume)
				if ac == nil {
					if forLVM {
						// search AC in sub storage class
						ac = e.searchClosestAC(acByNodeAndSCMap[nodeID][subSC], volume)
					} else if sc == v1.StorageClassAny {
						for _, acs := range acByNodeAndSCMap[nodeID] {
							ac = e.searchClosestAC(acs, volume)
							if ac != nil {
								break
							}
						}
					}
					if ac == nil {
						// as soon as for some volume in some SC there are no any AC - consider
						// that node doesn't match volumes requests
						matched = false
						goto CheckMatched
					}
				}
				// here ac != nil
				if ac.Spec.StorageClass != sc { // sc relates to LVG or sc == ANY
					if util.IsStorageClassLVG(ac.Spec.StorageClass) || forLVM {
						if forLVM {
							// remove AC with subSC
							delete(acByNodeAndSCMap[nodeID][subSC], ac.Name)
							ac.Spec.StorageClass = sc // e.g. HDD -> HDDLVG
							if _, ok := acByNodeAndSCMap[nodeID][sc]; !ok {
								acByNodeAndSCMap[nodeID][sc] = map[string]*accrd.AvailableCapacity{}
							}
							acByNodeAndSCMap[nodeID][sc][ac.Name] = ac
						}
						ac.Spec.Size -= volume.Size
					} else {
						// sc == ANY && ac.Spec.StorageClass doesn't relate to LVG
						delete(acByNodeAndSCMap[nodeID][ac.Spec.StorageClass], ac.Name)
					}
				} else {
					if forLVM {
						ac.Spec.Size -= volume.Size
					} else {
						delete(acByNodeAndSCMap[nodeID][sc], ac.Name)
					}
				}
			}
		}
	CheckMatched:
		if matched {
			matchedNodes = append(matchedNodes, node)
		} else {
			if failedNodesMap == nil {
				failedNodesMap = map[string]string{}
			}
			failedNodesMap[node.Name] = fmt.Sprintf("Node doesn't contain required amount of AvailableCapacity")
		}
	}

	return
}

// searchClosestAC search AC that match all requirements from volume (size)
func (e *Extender) searchClosestAC(acs map[string]*accrd.AvailableCapacity, volume *genV1.Volume) *accrd.AvailableCapacity {
	var (
		maxSize  int64 = math.MaxInt64
		pickedAC *accrd.AvailableCapacity
	)

	for _, ac := range acs {
		if ac.Spec.Size >= volume.Size && ac.Spec.Size < maxSize {
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
