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

package util

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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
	annotation "github.com/dell/csi-baremetal/pkg/crcontrollers/node/common"
)

// SchedulerUtils implements logic for nodes filtering based on pod volumes requirements and Available Capacities
type SchedulerUtils struct {
	k8sClient *k8s.KubeClient
	k8sCache  *k8s.KubeCache
	// namespace in which SchedulerUtils will be search Available Capacity
	namespace     string
	provisioner   string
	annotation    annotation.NodeAnnotation
	annotationKey string
	nodeSelector  string
	sync.Mutex
	isSchedulerPlugin      bool
	logger                 *logrus.Entry
	capacityManagerBuilder capacityplanner.CapacityManagerBuilder
}

// NewSchedulerUtils returns new instance of SchedulerUtils struct
func NewSchedulerUtils(logger *logrus.Logger, kubeClient *k8s.KubeClient,
	kubeCache *k8s.KubeCache, provisioner string, featureConf fc.FeatureChecker, annotationKey, nodeselector string) (*SchedulerUtils, error) {
	// TODO refactor annotation service
	// initialize with annotationKey and nodeselector
	// and use those params in the all related function
	annotationSrv := annotation.New(
		kubeClient,
		logger,
		annotation.WithFeatureConfig(featureConf),
		annotation.WithRetryDelay(3*time.Second),
		annotation.WithRetryNumber(20),
	)
	return &SchedulerUtils{
		k8sClient:              kubeClient,
		k8sCache:               kubeCache,
		provisioner:            provisioner,
		annotation:             annotationSrv,
		annotationKey:          annotationKey,
		nodeSelector:           nodeselector,
		isSchedulerPlugin:      true,
		logger:                 logger.WithField("component", "SchedulerUtils"),
		capacityManagerBuilder: &capacityplanner.DefaultCapacityManagerBuilder{},
	}, nil
}

// gatherCapacityRequestsByProvisioner search all volumes in pod' spec that should be provisioned
// by provisioner e.provisioner and construct genV1.Volume struct for each of such volume
func (su *SchedulerUtils) GatherCapacityRequestsByProvisioner(ctx context.Context, pod *coreV1.Pod) ([]*genV1.CapacityRequest, error) {
	ll := su.logger.WithFields(logrus.Fields{
		"sessionUUID": ctx.Value(base.RequestUUID),
		"method":      "gatherCapacityRequestsByProvisioner",
		"pod":         pod.Name,
	})

	scs, err := su.buildSCChecker(ctx, ll)
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
			err := su.k8sCache.ReadCR(ctx, v.PersistentVolumeClaim.ClaimName, pod.Namespace, pvc)
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
func (su *SchedulerUtils) createCapacityRequest(ctx context.Context, podName string, volume coreV1.Volume) (request *genV1.CapacityRequest, err error) {
	ll := su.logger.WithFields(logrus.Fields{
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

func (su *SchedulerUtils) Filter(ctx context.Context, pod *coreV1.Pod, nodes []coreV1.Node, capacities []*genV1.CapacityRequest) (matchedNodes []coreV1.Node,
	err error) {
	matchedNodes, _, err = su.filter(ctx, pod, nodes, capacities)
	return matchedNodes, err
}

// filter is an algorithm for defining whether requested volumes could be provisioned on particular node or no
// nodes - list of node candidate, volumes - requested volumes
// returns: matchedNodes - list of nodes on which volumes could be provisioned
// filteredNodes - represents the filtered out nodes, with node names and failure messages
func (su *SchedulerUtils) filter(ctx context.Context, pod *coreV1.Pod, nodes []coreV1.Node, capacities []*genV1.CapacityRequest) (matchedNodes []coreV1.Node,
	filteredNodes schedulerapi.FailedNodesMap, err error) {
	ll := su.logger.WithFields(logrus.Fields{
		"sessionUUID": ctx.Value(base.RequestUUID),
		"method":      "filter",
		"pod":         pod.Name,
	})
	// ignore when no storage allocation requests
	if len(capacities) == 0 {
		return nodes, nil, nil
	}

	// construct ACR name
	reservationName := getReservationName(pod)
	if su.isSchedulerPlugin {
		node := nodes[0]
		nodeID, err := su.annotation.GetNodeID(&node, su.annotationKey, su.nodeSelector)
		if err == nil && nodeID != "" {
			reservationName = reservationName + "-" + nodeID
		} else if err != nil {
			su.logger.Errorf("failed to get NodeID: %s", err)
		}
	}
	// read reservation
	reservation := &acrcrd.AvailableCapacityReservation{}
	err = su.k8sClient.ReadCR(ctx, reservationName, "", reservation)
	if err != nil {
		ll.Debugf("ACR %s not found!", reservationName)
		if k8serrors.IsNotFound(err) {
			// create new reservation
			if err := su.createReservation(ctx, pod.Namespace, reservationName, nodes, capacities); err != nil {
				// cannot create reservation
				return nil, nil, err
			}
			return su.handleReservation(ctx, reservation, nodes)
		}
		// issue with reading reservation
		return nil, nil, err
	}

	// reservation found
	return su.handleReservation(ctx, reservation, nodes)
}

func getReservationName(pod *coreV1.Pod) string {
	namespace := pod.Namespace
	if namespace == "" {
		namespace = "default"
	}

	return namespace + "-" + pod.Name
}

func (su *SchedulerUtils) createReservation(ctx context.Context, namespace string, name string, nodes []coreV1.Node,
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
	if su.isSchedulerPlugin {
		node := nodes[0]
		nodeID, err := su.annotation.GetNodeID(&node, su.annotationKey, su.nodeSelector)
		if err != nil {
			su.logger.Errorf("failed to get NodeID: %s", err)
		} else {
			for _, request := range reservation.ReservationRequests {
				request.CapacityRequest.Name = request.CapacityRequest.Name + "-" + nodeID
			}
		}
	}

	// fill in node requests
	reservation.NodeRequests = &genV1.NodeRequests{}
	reservation.NodeRequests.Requested = su.prepareListOfRequestedNodes(nodes)
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

	if err := su.k8sClient.CreateCR(ctx, name, reservationResource); err != nil {
		// cannot create reservation
		return err
	}
	return nil
}

func (su *SchedulerUtils) prepareListOfRequestedNodes(nodes []coreV1.Node) (nodeNames []string) {
	nodeNames = []string{}
	for _, node := range nodes {
		n := node
		nodeID, err := su.annotation.GetNodeID(&n, su.annotationKey, su.nodeSelector)
		if err != nil {
			su.logger.Errorf("node:%s cant get NodeID error: %s", n.Name, err)
			continue
		}
		if nodeID == "" {
			continue
		}
		nodeNames = append(nodeNames, nodeID)
	}

	return nodeNames
}

func (su *SchedulerUtils) handleReservation(ctx context.Context, reservation *acrcrd.AvailableCapacityReservation,
	nodes []coreV1.Node) (matchedNodes []coreV1.Node, filteredNodes schedulerapi.FailedNodesMap, err error) {
	ll := su.logger.WithFields(logrus.Fields{
		"method":      "handleReservation",
		"reservation": reservation.Name,
	})

	ll.Infof("Pulling reservation %s status", reservation.Name)

	var (
		res                 = &acrcrd.AvailableCapacityReservation{}
		timeoutBetweenCheck = time.Second
	)
	for {
		select {
		case <-ctx.Done():
			ll.Warnf("Context is done but ACR still not reach RESERVED or REJECTED status")
			return nil, nil, fmt.Errorf("volume context is done")
		case <-time.After(timeoutBetweenCheck):
			if err = su.k8sClient.ReadCR(ctx, reservation.Name, "", res); err != nil {
				ll.Errorf("Unable to read ACR: %v", err)
				if k8serrors.IsNotFound(err) {
					ll.Error("ACR doesn't exist")
					return nil, nil, fmt.Errorf("volume isn't found")
				}
				continue
			}
			if res.Spec.Status == v1.ReservationRejected {
				if err := su.resendReservationRequest(ctx, res, nodes); err != nil {
					// cannot resend reservation
					return nil, nil, err
				}
			} else if res.Spec.Status == v1.ReservationConfirmed {
				// need to filter nodes here
				filteredNodes = schedulerapi.FailedNodesMap{}
				for _, requestedNode := range nodes {
					isFound := false
					// node ID
					node := requestedNode
					nodeID, err := su.annotation.GetNodeID(&node, su.annotationKey, su.nodeSelector)
					if err != nil {
						su.logger.Errorf("failed to get NodeID: %s", err)
						continue
					}
					if nodeID == "" {
						continue
					}
					for _, node := range res.Spec.NodeRequests.Reserved {
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
					if err := su.resendReservationRequest(ctx, res, nodes); err != nil {
						// cannot resend reservation
						return nil, nil, err
					}
					continue
				}

				return matchedNodes, filteredNodes, nil
			}
		}
	}
}

func (su *SchedulerUtils) resendReservationRequest(ctx context.Context, reservation *acrcrd.AvailableCapacityReservation,
	nodes []coreV1.Node) error {
	reservation.Spec.Status = v1.ReservationRequested
	// update nodes
	reservation.Spec.NodeRequests.Requested = su.prepareListOfRequestedNodes(nodes)
	if len(reservation.Spec.NodeRequests.Requested) == 0 {
		return nil
	}
	// remove reservations if any
	for i := range reservation.Spec.ReservationRequests {
		reservation.Spec.ReservationRequests[i].Reservations = nil
	}

	if err := su.k8sClient.UpdateCR(ctx, reservation); err != nil {
		// cannot update reservation
		return err
	}

	return nil
}

func (su *SchedulerUtils) score(nodes []coreV1.Node) ([]schedulerapi.HostPriority, error) {
	ll := su.logger.WithFields(logrus.Fields{
		"method": "score",
	})

	var volumeList = &volcrd.VolumeList{}
	if err := su.k8sCache.ReadList(context.Background(), volumeList); err != nil {
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
		nodeID, err := su.annotation.GetNodeID(&node, su.annotationKey, su.nodeSelector)
		if err != nil {
			su.logger.Errorf("failed to get NodeID: %s", err)
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
func (su *SchedulerUtils) buildSCChecker(ctx context.Context, log *logrus.Entry) (*scChecker, error) {
	ll := log.WithFields(logrus.Fields{
		"method": "buildSCChecker",
	})

	var (
		result = &scChecker{managedSCs: map[string]string{}, unmanagedSCs: map[string]bool{}}
		scs    = storageV1.StorageClassList{}
	)

	if err := su.k8sCache.ReadList(ctx, &scs); err != nil {
		return nil, err
	}

	for _, sc := range scs.Items {
		if sc.Provisioner == su.provisioner {
			result.managedSCs[sc.Name] = strings.ToUpper(sc.Parameters[base.StorageTypeKey])
		} else {
			result.unmanagedSCs[sc.Name] = true
		}
	}

	ll.Debugf("related SCs: %+v", result.managedSCs)
	ll.Debugf("unrelated SCs: %+v", result.unmanagedSCs)

	if len(result.managedSCs) == 0 {
		return nil, fmt.Errorf("there are no any storage classes with provisioner %s", su.provisioner)
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