package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api/v1"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
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
	logger    *logrus.Entry
}

const (
	namespace      = "kube-system"
	pluginNameMask = "baremetal"
)

// NewExtender returns new instance of Extender struct
func NewExtender(logger *logrus.Logger) (*Extender, error) {
	k8sClient, err := k8s.GetK8SClient()
	if err != nil {
		return nil, err
	}
	kubeClient := k8s.NewKubeClient(k8sClient, logger, namespace)
	return &Extender{
		k8sClient: kubeClient,
		logger:    logger.WithField("component", "Extender"),
	}, nil
}

// FilterHandler extracts ExtenderArgs struct from req and writes ExtenderFilterResult to the w
func (e *Extender) FilterHandler(w http.ResponseWriter, req *http.Request) {
	ll := e.logger.WithField("method", "FilterHandler")
	ll.Debugf("Processing request: %v. With context %v", req, req.Context())

	w.Header().Set("Content-Type", "application/json")
	resp := json.NewEncoder(w)

	var extenderArgs schedulerapi.ExtenderArgs

	if err := json.NewDecoder(req.Body).Decode(&extenderArgs); err != nil {
		ll.Errorf("Unable to decode request body: %v", err)
		e.encodeResults(resp, &schedulerapi.ExtenderFilterResult{Error: err.Error()})
		return
	}

	ll.Info("Filtering")
	volumes, err := e.gatherVolumesByProvisioner(req.Context(), extenderArgs.Pod)
	if err != nil {
		e.encodeResults(resp, &schedulerapi.ExtenderFilterResult{Error: err.Error()})
		return
	}
	ll.Debugf("Required volumes: %v", volumes)

	matchedNodes, failedNodes, err := e.filter(extenderArgs.Nodes.Items, volumes)
	errMsg := ""
	if err != nil {
		ll.Errorf("filter finished with error: %v", err)
		errMsg = err.Error()
	} else if len(matchedNodes) == 0 {
		errMsg = "There are no nodes that matched requested volumes"
		ll.Error(errMsg)
	}

	extenderRes := &schedulerapi.ExtenderFilterResult{
		Nodes: &k8sV1.NodeList{
			TypeMeta: extenderArgs.Nodes.TypeMeta,
			Items:    matchedNodes,
		},
		FailedNodes: failedNodes,
		Error:       errMsg,
	}

	e.encodeResults(resp, extenderRes)
}

func (e *Extender) encodeResults(resp *json.Encoder, res *schedulerapi.ExtenderFilterResult) {
	ll := e.logger.WithField("method", "encodeResults")

	ll.Infof("Writing ExtenderFilterResult, suitable nodes: %v, not suitable nodes: %v, error: %v",
		res.NodeNames, res.FailedNodes, res.Error)
	if err := resp.Encode(res); err != nil {
		ll.Errorf("Unable to write response %v: %v", resp, err)
	}
}

// gatherVolumesByProvisioner search all volumes in pod' spec that should be provisioned
// by provisioner that match pluginNameMask and construct getV1.Volume struct for each of such volume
func (e *Extender) gatherVolumesByProvisioner(ctx context.Context, pod *k8sV1.Pod) ([]*genV1.Volume, error) {
	ll := e.logger.WithFields(logrus.Fields{
		"method": "gatherVolumesByProvisioner",
		"pod":    pod.Name,
	})
	ll.Debug("Processing ...")

	volumes := make([]*genV1.Volume, 0)
	for _, v := range pod.Spec.Volumes {
		e.logger.Tracef("Inspecting pod volume %+v", v)
		// check whether there are Ephemeral volumes or no
		if v.CSI != nil {
			if strings.Contains(v.CSI.Driver, pluginNameMask) {
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
			pvc := &k8sV1.PersistentVolumeClaim{}
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
			if strings.Contains(*pvc.Spec.StorageClassName, pluginNameMask) {
				storageRes, ok := pvc.Spec.Resources.Requests[k8sV1.ResourceStorage]
				if !ok {
					ll.Errorf("There is no key for storage resource for PVC %s", pvc.Name)
					storageRes = resource.Quantity{}
				}

				mode := ""
				if pvc.Spec.VolumeMode != nil {
					mode = string(*pvc.Spec.VolumeMode)
				}

				volumes = append(volumes, &genV1.Volume{
					Id:           pvc.Name,
					StorageClass: *pvc.Spec.StorageClassName,
					Size:         storageRes.Value(),
					Mode:         mode,
					Ephemeral:    false,
				})
			}
		}
	}
	return volumes, nil
}

// constructVolumeFromCSISource constructs genV1.Volume based on fields from k8sV1.Volume.CSI
func (e *Extender) constructVolumeFromCSISource(v *k8sV1.CSIVolumeSource) (vol *genV1.Volume, err error) {
	// if some parameters aren't parsed for some reason empty volume will be returned in order count that volume
	vol = &genV1.Volume{
		StorageClass: v1.StorageClassAny,
		Ephemeral:    true,
	}

	sc, ok := v.VolumeAttributes[base.StorageTypeKey]
	if !ok {
		return vol, fmt.Errorf("unable to detect storage class from attributes %v", v.VolumeAttributes)
	}
	vol.StorageClass = sc

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

func (e *Extender) filter(nodes []k8sV1.Node, volumes []*genV1.Volume) (matchedNodes []k8sV1.Node,
	failedNodesMap schedulerapi.FailedNodesMap, err error) {
	ll := e.logger.WithField("method", "filter")
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
		ll.Errorf("Unable to read AvailableCapacity list: %v", err)
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
		ac := ac  // ac uses in range and represent different value on each iteration but we need to put pointer in map
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
		for sc, volumes := range scVolumeMap {
			for _, volume := range volumes {
				subSC := util.GetSubStorageClass(sc) // returns empty string for non LVM storage classes
				forLVM := sc == v1.StorageClassSSDLVG || sc == v1.StorageClassHDDLVG

				if len(acByNodeAndSCMap[nodeID][sc]) == 0 && len(acByNodeAndSCMap[nodeID][subSC]) == 0 {
					matched = false
					goto CheckMatched
				}

				var ac *accrd.AvailableCapacity
				ac = e.searchClosestAC(acByNodeAndSCMap[nodeID][sc], volume)
				if ac == nil {
					if forLVM {
						// search AC in sub storage class
						ac = e.searchClosestAC(acByNodeAndSCMap[nodeID][subSC], volume)
						if ac != nil { // found
							// mark such AC by real SC (switch sc to subSC) and just change AC volume
							ac.Spec.StorageClass = subSC
							ac.Spec.Size -= volume.Size
							delete(acByNodeAndSCMap[nodeID][subSC], ac.Name)
							if ac.Spec.Size > common.AcSizeMinThresholdBytes {
								if _, ok := acByNodeAndSCMap[nodeID][sc]; !ok {
									acByNodeAndSCMap[nodeID][sc] = map[string]*accrd.AvailableCapacity{}
								}
								acByNodeAndSCMap[nodeID][sc][ac.Name] = ac
							}
							continue
						}
					}
				} else {
					if forLVM {
						// update corresponding AC volume
						ac.Spec.Size -= volume.Size
						if ac.Spec.Size > common.AcSizeMinThresholdBytes {
							acByNodeAndSCMap[nodeID][sc][ac.Name] = ac
						} else {
							delete(acByNodeAndSCMap[nodeID][sc], ac.Name)
						}
					} else {
						// picked up AC that was found (remove from AC list)
						delete(acByNodeAndSCMap[nodeID][sc], ac.Name)
					}
				}
				if ac == nil {
					// as soon as for some volume in some SC there are no any AC - consider
					// that node doesn't match volumes requests
					matched = false
					goto CheckMatched
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
