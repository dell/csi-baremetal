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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schedulerapi "k8s.io/kube-scheduler/extender/v1"

	genV1 "github.com/dell/csi-baremetal/api/generated/v1"
	v1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	volcrd "github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/capacityplanner"
	baseerr "github.com/dell/csi-baremetal/pkg/base/error"
	fc "github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/base/util"
	annotations "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
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
	nodeSelector   string
	sync.Mutex
	logger                 *logrus.Entry
	capacityManagerBuilder capacityplanner.CapacityManagerBuilder
}

// NewExtender returns new instance of Extender struct
func NewExtender(logger *logrus.Logger, kubeClient *k8s.KubeClient,
	kubeCache *k8s.KubeCache, provisioner string, featureConf fc.FeatureChecker, annotationKey, nodeselector string) (*Extender, error) {
	return &Extender{
		k8sClient:              kubeClient,
		k8sCache:               kubeCache,
		provisioner:            provisioner,
		featureChecker:         featureConf,
		annotationKey:          annotationKey,
		nodeSelector:           nodeselector,
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

	ll = ll.WithFields(logrus.Fields{
		"pod": extenderArgs.Pod.Name,
	})

	ll.Info("Filtering")
	ctxWithVal := context.WithValue(req.Context(), base.RequestUUID, sessionUUID)
	pod := extenderArgs.Pod
	requests, err := e.gatherCapacityRequestsByProvisioner(ctxWithVal, pod)
	if err != nil {
		// not found error is re-triable
		if err != baseerr.ErrorNotFound {
			extenderRes.Error = err.Error()
		}
		if err := resp.Encode(extenderRes); err != nil {
			ll.Errorf("Unable to write response %v: %v", extenderRes, err)
		}
		return
	}
	ll.Debugf("Required capacity: %v", requests)

	matchedNodes, failedNodes, err := e.filter(ctxWithVal, pod, extenderArgs.Nodes.Items, requests)

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

// gatherCapacityRequestsByProvisioner search all volumes in pod' spec that should be provisioned
// by provisioner e.provisioner and construct genV1.Volume struct for each of such volume
func (e *Extender) gatherCapacityRequestsByProvisioner(ctx context.Context, pod *coreV1.Pod) ([]*genV1.CapacityRequest, error) {
	ll := e.logger.WithFields(logrus.Fields{
		"sessionUUID": ctx.Value(base.RequestUUID),
		"method":      "gatherCapacityRequestsByProvisioner",
		"pod":         pod.Name,
	})

	scs, err := e.buildSCChecker(ctx, ll)
	if err != nil {
		ll.Errorf("Unable to collect storage classes: %v", err)
		return nil, err
	}

	requests := make([]*genV1.CapacityRequest, 0)
	// TODO - refactor repeatable code - https://github.com/dell/csi-baremetal/issues/760
	for _, v := range pod.Spec.Volumes {
		ll.Debugf("Volume %s details: %+v", v.Name, v)
		// check whether volume Ephemeral or not
		if v.Ephemeral != nil {
			claimSpec := v.Ephemeral.VolumeClaimTemplate.Spec
			if claimSpec.StorageClassName == nil {
				ll.Warningf("PVC %s skipped due to empty StorageClass", v.Ephemeral.VolumeClaimTemplate.Name)
				continue
			}

			storageType, scType := scs.check(*claimSpec.StorageClassName)
			switch scType {
			case unknown:
				ll.Warningf("SC %s is not found in cache, wait until update", *claimSpec.StorageClassName)
				return nil, baseerr.ErrorNotFound
			case unmanagedSC:
				ll.Infof("SC %s is not provisioned by CSI Baremetal driver, skip this volume", *claimSpec.StorageClassName)
				continue
			case managedSC:
				requests = append(requests, createRequestFromPVCSpec(
					generateEphemeralVolumeName(pod.GetName(), v.Name),
					storageType,
					claimSpec.Resources,
					ll,
				))
			default:
				return nil, fmt.Errorf("scChecker return code is unfound: %d", scType)
			}
		}
		if v.PersistentVolumeClaim != nil {
			pvc := &coreV1.PersistentVolumeClaim{}
			err := e.k8sCache.ReadCR(ctx, v.PersistentVolumeClaim.ClaimName, pod.Namespace, pvc)
			if err != nil {
				ll.Errorf("Unable to read PVC %s in NS %s: %v. ", v.PersistentVolumeClaim.ClaimName, pod.Namespace, err)
				// PVC can be created later. csi-provisioner repeat request if not error.
				return nil, baseerr.ErrorNotFound
			}

			if pvc.Spec.StorageClassName == nil {
				ll.Warningf("PVC %s skipped due to empty StorageClass", pvc.Name)
				continue
			}

			ll.Debugf("PVC %s status: %+v spec: %+v SC: %s", pvc.Name, pvc.Status, pvc.Spec, *pvc.Spec.StorageClassName)

			if pvc.Status.Phase == coreV1.ClaimBound {
				ll.Infof("PVC %s is Bound", pvc.Name)
				continue
			}
			if pvc.Status.Phase == coreV1.ClaimLost {
				ll.Warningf("PVC %s is Lost", pvc.Name)
				continue
			}

			// We need to check related Volume CR here, but it's no option to receive the right one (PVC
			// has PV name only when it's in Bound. It may leads to possible races, when ACR is removed in
			// CreateVolume request, but recreated if it filter request repeats due to some reasons.
			// Workaround is realized in CSI Operator ACRValidator. It checks all ACR and removed ones for
			// Running pods.

			storageType, scType := scs.check(*pvc.Spec.StorageClassName)
			switch scType {
			case unknown:
				ll.Warningf("SC %s is not found in cache, wait until update", *pvc.Spec.StorageClassName)
				return nil, baseerr.ErrorNotFound
			case unmanagedSC:
				ll.Infof("SC %s is not provisioned by CSI Baremetal driver, skip PVC %s", *pvc.Spec.StorageClassName, pvc.Name)
				continue
			case managedSC:
				requests = append(requests, createRequestFromPVCSpec(
					pvc.Name,
					storageType,
					pvc.Spec.Resources,
					ll,
				))
			default:
				return nil, fmt.Errorf("scChecker return code is unfound: %d", scType)
			}
		}
	}
	return requests, nil
}

// createCapacityRequest constructs genV1.CapacityRequest based on coreV1.Volume.Name and fields from coreV1.Volume.CSI
func (e *Extender) createCapacityRequest(ctx context.Context, podName string, volume coreV1.Volume) (request *genV1.CapacityRequest, err error) {
	ll := e.logger.WithFields(logrus.Fields{
		"sessionUUID": ctx.Value(base.RequestUUID),
		"method":      "gatherCapacityRequestsByProvisioner",
		"pod":         podName,
	})

	// if some parameters aren't parsed for some reason
	// empty volume will be returned in order count that volume
	requestName := generateEphemeralVolumeName(podName, volume.Name)
	request = &genV1.CapacityRequest{Name: requestName, StorageClass: v1.StorageClassAny}

	v := volume.CSI
	sc, ok := v.VolumeAttributes[base.StorageTypeKey]
	if !ok {
		return request, fmt.Errorf("unable to detect storage class from attributes %v", v.VolumeAttributes)
	}
	request.StorageClass = util.ConvertStorageClass(sc)

	sizeStr, ok := v.VolumeAttributes[base.SizeKey]
	if !ok {
		return request, fmt.Errorf("unable to detect size from attributes %v", v.VolumeAttributes)
	}

	size, err := util.StrToBytes(sizeStr)
	if err != nil {
		return request, fmt.Errorf("unable to convert string %s to bytes: %v", sizeStr, err)
	}
	request.Size = size

	ll.Debugf("Request %s with %s SC and %d size created", request.Name, request.StorageClass, request.Size)

	return request, nil
}

// filter is an algorithm for defining whether requested volumes could be provisioned on particular node or no
// nodes - list of node candidate, volumes - requested volumes
// returns: matchedNodes - list of nodes on which volumes could be provisioned
// filteredNodes - represents the filtered out nodes, with node names and failure messages
func (e *Extender) filter(ctx context.Context, pod *coreV1.Pod, nodes []coreV1.Node, capacities []*genV1.CapacityRequest) (matchedNodes []coreV1.Node,
	filteredNodes schedulerapi.FailedNodesMap, err error) {
	// ignore when no storage allocation requests
	if len(capacities) == 0 {
		return nodes, nil, nil
	}

	// construct ACR name
	reservationName := getReservationName(pod)
	// read reservation
	reservation := &acrcrd.AvailableCapacityReservation{}
	err = e.k8sClient.ReadCR(ctx, reservationName, "", reservation)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// create new reservation
			if err := e.createReservation(ctx, pod.Namespace, reservationName, nodes, capacities); err != nil {
				// cannot create reservation
				return nil, nil, err
			}
			// not an error - reservation requested
			return nil, nil, nil
		}
		// issue with reading reservation
		return nil, nil, err
	}

	// reservation found
	return e.handleReservation(ctx, reservation, nodes)
}

func getReservationName(pod *coreV1.Pod) string {
	namespace := pod.Namespace
	if namespace == "" {
		namespace = "default"
	}

	return namespace + "-" + pod.Name
}

func (e *Extender) createReservation(ctx context.Context, namespace string, name string, nodes []coreV1.Node,
	capacities []*genV1.CapacityRequest) error {
	// ACR CRD
	reservation := genV1.AvailableCapacityReservation{
		Namespace: namespace,
		Status:    v1.ReservationRequested,
	}

	// fill in reservation requests
	reservation.ReservationRequests = make([]*genV1.ReservationRequest, len(capacities))
	for i, capacity := range capacities {
		reservation.ReservationRequests[i] = &genV1.ReservationRequest{CapacityRequest: capacity}
	}

	// fill in node requests
	reservation.NodeRequests = &genV1.NodeRequests{}
	reservation.NodeRequests.Requested = e.prepareListOfRequestedNodes(nodes)
	if len(reservation.NodeRequests.Requested) == 0 {
		return nil
	}

	// create new reservation
	reservationResource := &acrcrd.AvailableCapacityReservation{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1.AvailableCapacityReservationKind,
			APIVersion: v1.APIV1Version,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       reservation,
	}

	if err := e.k8sClient.CreateCR(ctx, name, reservationResource); err != nil {
		// cannot create reservation
		return err
	}
	return nil
}

func (e *Extender) prepareListOfRequestedNodes(nodes []coreV1.Node) []string {
	requestedNodes := []string{}

	for _, node := range nodes {
		n := node
		nodeID, err := annotations.GetNodeID(&n, e.annotationKey, e.nodeSelector, e.featureChecker)
		if err != nil {
			e.logger.Errorf("node:%s cant get NodeID error: %s", n.Name, err)
			continue
		}
		if nodeID == "" {
			continue
		}
		requestedNodes = append(requestedNodes, nodeID)
	}

	return requestedNodes
}

func (e *Extender) handleReservation(ctx context.Context, reservation *acrcrd.AvailableCapacityReservation,
	nodes []coreV1.Node) (matchedNodes []coreV1.Node, filteredNodes schedulerapi.FailedNodesMap, err error) {
	// handle reservation status
	switch reservation.Spec.Status {
	case v1.ReservationRequested:
		// not an error - reservation requested. need to retry
		return nil, nil, nil
	case v1.ReservationConfirmed:
		// need to filter nodes here
		filteredNodes = schedulerapi.FailedNodesMap{}
		for _, requestedNode := range nodes {
			isFound := false
			// node ID
			node := requestedNode
			nodeID, err := annotations.GetNodeID(&node, e.annotationKey, e.nodeSelector, e.featureChecker)
			if err != nil {
				e.logger.Errorf("failed to get NodeID: %s", err)
				continue
			}
			if nodeID == "" {
				continue
			}
			for _, node := range reservation.Spec.NodeRequests.Reserved {
				if node == nodeID {
					matchedNodes = append(matchedNodes, requestedNode)
					isFound = true
					break
				}
			}
			// node name
			name := requestedNode.Name
			if !isFound {
				filteredNodes[name] = fmt.Sprintf("No available capacity found on the node %s", name)
			}
		}
		// requested nodes has changed. need to update reservation with the new list of nodes
		if len(matchedNodes) == 0 {
			return nil, nil, e.resendReservationRequest(ctx, reservation, nodes)
		}

		return matchedNodes, filteredNodes, nil
	case v1.ReservationRejected:
		// no available capacity
		// request reservation again
		return nil, nil, e.resendReservationRequest(ctx, reservation, nodes)
	}

	return nil, nil, errors.New("unsupported reservation status: " + reservation.Spec.Status)
}

func (e *Extender) resendReservationRequest(ctx context.Context, reservation *acrcrd.AvailableCapacityReservation,
	nodes []coreV1.Node) error {
	reservation.Spec.Status = v1.ReservationRequested
	// update nodes
	reservation.Spec.NodeRequests.Requested = e.prepareListOfRequestedNodes(nodes)
	if len(reservation.Spec.NodeRequests.Requested) == 0 {
		return nil
	}
	// remove reservations if any
	for i := range reservation.Spec.ReservationRequests {
		reservation.Spec.ReservationRequests[i].Reservations = nil
	}

	if err := e.k8sClient.UpdateCR(ctx, reservation); err != nil {
		// cannot update reservation
		return err
	}

	return nil
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
		nodeID, err := annotations.GetNodeID(&node, e.annotationKey, e.nodeSelector, e.featureChecker)
		if err != nil {
			e.logger.Errorf("failed to get NodeID: %s", err)
			continue
		}

		if nodeID == "" {
			continue
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
func nodePrioritize(nodeMapping map[string][]volcrd.Volume) (map[string]int64, int64) {
	var maxCount int64
	for _, volumes := range nodeMapping {
		volCount := int64(len(volumes))
		if maxCount < volCount {
			maxCount = volCount
		}
	}
	nrank := make(map[string]int64, len(nodeMapping))
	for node, volumes := range nodeMapping {
		nrank[node] = maxCount - int64(len(volumes))
	}
	return nrank, maxCount
}

func generateEphemeralVolumeName(podName, volumeName string) string {
	return podName + "-" + volumeName
}

func createRequestFromPVCSpec(volumeName, storageType string, resourceRequirements coreV1.ResourceRequirements, log *logrus.Entry) *genV1.CapacityRequest {
	storageReq, ok := resourceRequirements.Requests[coreV1.ResourceStorage]
	if !ok {
		log.Errorf("There is no key for storage resource for PVC %s", volumeName)
		storageReq = resource.Quantity{}
	}
	return &genV1.CapacityRequest{
		Name:         volumeName,
		StorageClass: util.ConvertStorageClass(storageType),
		Size:         storageReq.Value(),
	}
}

// scChecker keeps info about the related SCs (provisioned by CSI Baremetal) and
// the unrelated ones (prvisioned by other CSI drivers)
type scChecker struct {
	managedSCs   map[string]string
	unmanagedSCs map[string]bool
}

// buildSCChecker creates an instance of scChecker
func (e *Extender) buildSCChecker(ctx context.Context, log *logrus.Entry) (*scChecker, error) {
	ll := log.WithFields(logrus.Fields{
		"method": "buildSCChecker",
	})

	var (
		result = &scChecker{managedSCs: map[string]string{}, unmanagedSCs: map[string]bool{}}
		scs    = storageV1.StorageClassList{}
	)

	if err := e.k8sCache.ReadList(ctx, &scs); err != nil {
		return nil, err
	}

	for _, sc := range scs.Items {
		if sc.Provisioner == e.provisioner {
			result.managedSCs[sc.Name] = strings.ToUpper(sc.Parameters[base.StorageTypeKey])
		} else {
			result.unmanagedSCs[sc.Name] = true
		}
	}

	ll.Debugf("related SCs: %+v", result.managedSCs)
	ll.Debugf("unrelated SCs: %+v", result.unmanagedSCs)

	if len(result.managedSCs) == 0 {
		return nil, fmt.Errorf("there are no any storage classes with provisioner %s", e.provisioner)
	}

	return result, nil
}

// scChecker.check return codes
const (
	managedSC = iota
	unmanagedSC
	unknown
)

// check returns storageType and scType, return codes:
// relatedSC - SC is provisioned by CSI baremetal
// unrelatedSC - SC is provisioned by another CSI driver
// unknown - SC is not found in cache
func (ch *scChecker) check(name string) (string, int) {
	if storageType, ok := ch.managedSCs[name]; ok {
		return storageType, managedSC
	}

	if _, ok := ch.unmanagedSCs[name]; ok {
		return "", unmanagedSC
	}

	return "", unknown
}
