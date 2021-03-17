/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package extender

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api/v1"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	volcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	fc "github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	annotations "github.com/dell/csi-baremetal/pkg/crcontrollers/operator/common"
)

// Extender holds http handlers for scheduler extender endpoints and implements logic for nodes filtering
// based on pod volumes requirements and Available Capacities
type Extender struct {
	k8sClient *k8s.KubeClient
	k8sCache  *k8s.KubeCache
	// namespace in which Extender will be search Available Capacity
	namespace      string
	provisioner    string
	featureChecker fc.FeatureChecker
	annotationKey  string
	sync.Mutex
	logger                 *logrus.Entry
	capacityManagerBuilder capacityplanner.CapacityManagerBuilder
}

// NewExtender returns new instance of Extender struct
func NewExtender(logger *logrus.Logger, kubeClient *k8s.KubeClient,
	kubeCache *k8s.KubeCache, provisioner string, featureConf fc.FeatureChecker, annotationKey string) (*Extender, error) {
	return &Extender{
		k8sClient:              kubeClient,
		k8sCache:               kubeCache,
		provisioner:            provisioner,
		featureChecker:         featureConf,
		annotationKey:          annotationKey,
		logger:                 logger.WithField("component", "Extender"),
		capacityManagerBuilder: &capacityplanner.DefaultCapacityManagerBuilder{},
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
	ctxWithVal := context.WithValue(req.Context(), base.RequestUUID, sessionUUID)
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

// PrioritizeHandler helps with even distribution of the volumes across the nodes.
// It will set priority based on the formula:
// rank of node X = max number of volumes - number of volume on node X.
func (e *Extender) PrioritizeHandler(w http.ResponseWriter, req *http.Request) {
	sessionUUID := uuid.New().String()
	ll := e.logger.WithFields(logrus.Fields{
		"sessionUUID": sessionUUID,
		"method":      "PrioritizeHandler",
	})
	ll.Infof("Processing request: %v", req)

	w.Header().Set("Content-Type", "application/json")
	resp := json.NewEncoder(w)

	var (
		extenderArgs schedulerapi.ExtenderArgs
	)

	if err := json.NewDecoder(req.Body).Decode(&extenderArgs); err != nil {
		ll.Errorf("Unable to decode request body: %v", err)
		return
	}

	ll.Info("Scoring")

	e.Lock()
	defer e.Unlock()

	hostPriority, err := e.score(extenderArgs.Nodes.Items)
	if err != nil {
		ll.Errorf("Unable to score %v", err)
		return
	}
	ll.Infof("Score results: %v", hostPriority)
	extenderRes := (schedulerapi.HostPriorityList)(hostPriority)

	if err := resp.Encode(&extenderRes); err != nil {
		ll.Errorf("Unable to write response %v: %v", extenderRes, err)
	}
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
		"sessionUUID": ctx.Value(base.RequestUUID),
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
			err := e.k8sCache.ReadCR(ctx, v.PersistentVolumeClaim.ClaimName, pod.Namespace, pvc)
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
	acReader := capacityplanner.NewACReader(e.k8sClient, e.logger, true)
	acrReader := capacityplanner.NewACRReader(e.k8sClient, e.logger, true)
	reservedCapReader := capacityplanner.NewUnreservedACReader(e.logger, acReader, acrReader)
	capManager := e.capacityManagerBuilder.GetCapacityManager(e.logger, reservedCapReader)

	placingPlan, err := capManager.PlanVolumesPlacing(ctx, volumes)
	if err != nil {
		return matchedNodes, failedNodesMap, err
	}

	noACForNodeMsg := "Node doesn't contain required amount of AvailableCapacity"

	failedNodesMap = schedulerapi.FailedNodesMap{}
	for _, node := range nodes {
		if placingPlan == nil {
			failedNodesMap[node.Name] = noACForNodeMsg
			continue
		}

		node := node
		nodeID, err := annotations.GetNodeID(&node, e.annotationKey, e.featureChecker)
		if err != nil {
			e.logger.Errorf("failed to get NodeID: %s", err)
		}

		placingForNode := placingPlan.GetVolumesToACMapping(nodeID)
		if placingForNode == nil {
			failedNodesMap[node.Name] = noACForNodeMsg
			continue
		}
		matchedNodes = append(matchedNodes, node)
	}
	if len(matchedNodes) != 0 {
		reservationHelper := capacityplanner.NewReservationHelper(e.logger, e.k8sClient, acReader, acrReader)
		err = reservationHelper.CreateReservation(ctx, placingPlan)
		if err != nil {
			e.logger.Errorf("failed to create reservation: %s", err.Error())
		}
	}

	return matchedNodes, failedNodesMap, err
}

func (e *Extender) score(nodes []coreV1.Node) ([]schedulerapi.HostPriority, error) {
	ll := e.logger.WithFields(logrus.Fields{
		"method": "score",
	})

	var volumeList = &volcrd.VolumeList{}
	if err := e.k8sCache.ReadList(context.Background(), volumeList); err != nil {
		err = fmt.Errorf("unable to read volumes list: %v", err)
		return nil, err
	}

	ll.Debugf("Got %d volumes", len(volumeList.Items))

	nodeMapping := nodeVolumeCountMapping(volumeList)

	priorityFromVolumes, maxVolumeCount := nodePrioritize(nodeMapping)

	ll.Debugf("nodes were ranked by their volumes %+v", priorityFromVolumes)
	hostPriority := make([]schedulerapi.HostPriority, 0, len(nodes))
	for _, node := range nodes {
		// set the highest priority if node doesn't have any volumes
		rank := maxVolumeCount

		node := node
		nodeID, err := annotations.GetNodeID(&node, e.annotationKey, e.featureChecker)
		if err != nil {
			e.logger.Errorf("failed to get NodeID: %s", err)
		}

		if r, ok := priorityFromVolumes[nodeID]; ok {
			rank = r
		}
		hostPriority = append(hostPriority, schedulerapi.HostPriority{
			Host:  node.GetName(),
			Score: rank,
		})
	}
	return hostPriority, nil
}

func nodeVolumeCountMapping(vollist *volcrd.VolumeList) map[string][]volcrd.Volume {
	nodeMapping := make(map[string][]volcrd.Volume)
	for _, volume := range vollist.Items {
		nID := volume.Spec.NodeId
		if _, found := nodeMapping[nID]; found {
			nodeMapping[nID] = append(nodeMapping[nID], volume)
			continue
		}
		nodeMapping[nID] = []volcrd.Volume{volume}
	}
	return nodeMapping
}

// nodePrioritize will set priority for nodes and also return the maximum priority
func nodePrioritize(nodeMapping map[string][]volcrd.Volume) (map[string]int, int) {
	var maxCount int
	for _, volumes := range nodeMapping {
		volCount := len(volumes)
		if maxCount < volCount {
			maxCount = volCount
		}
	}
	nrank := make(map[string]int, len(nodeMapping))
	for node, volumes := range nodeMapping {
		nrank[node] = maxCount - len(volumes)
	}
	return nrank, maxCount
}

// scNameStorageTypeMapping reads k8s storage class resources and collect map with key storage class name
// and value .parameters.storageType for that sc, collect only sc that have provisioner e.provisioner
func (e *Extender) scNameStorageTypeMapping(ctx context.Context) (map[string]string, error) {
	scs := storageV1.StorageClassList{}

	if err := e.k8sCache.ReadList(ctx, &scs); err != nil {
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
