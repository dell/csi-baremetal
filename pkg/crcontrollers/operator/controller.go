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

package operator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/crcontrollers/operator/common"
)

const (
	// namePrefix it is a prefix for Node CR name
	namePrefix = "csibmnode-"
	// finalizer for Node custom resource
	csibmNodeFinalizer = "dell.emc.csi/csibmnode-cleanup"
)

// Controller is a controller for Node CR
type Controller struct {
	k8sClient    *k8s.KubeClient
	nodeSelector *label
	cache        nodesMapping

	// holds k8s node names for which special ID settings is enabled,
	// it is used in Node CR deletion for avoiding recreation
	enabledForNode map[string]bool
	enabledMu      sync.RWMutex

	log *logrus.Entry

	// if used external annotations
	externalAnnotation bool
	// holds annotation which contains node UUID
	annotationKey string
}

type label struct {
	key   string
	value string
}

// nodesMapping it is not a thread safety cache that holds mapping between names for k8s node and BMCSINode CR objects
type nodesMapping struct {
	k8sToBMNode map[string]string // k8s node name to Node CR name
	bmToK8sNode map[string]string // Node CR name to k8s node name
}

func (nc *nodesMapping) getK8sNodeName(bmNodeName string) (string, bool) {
	res, ok := nc.bmToK8sNode[bmNodeName]
	return res, ok
}

func (nc *nodesMapping) getCSIBMNodeName(k8sNodeName string) (string, bool) {
	res, ok := nc.k8sToBMNode[k8sNodeName]
	return res, ok
}

func (nc *nodesMapping) put(k8sNodeName, bmNodeName string) {
	nc.k8sToBMNode[k8sNodeName] = bmNodeName
	nc.bmToK8sNode[bmNodeName] = k8sNodeName
}

// NewController returns instance of Controller
func NewController(nodeSelector string, useExternalAnnotaion bool, nodeAnnotaion string,
	k8sClient *k8s.KubeClient, logger *logrus.Logger) (*Controller, error) {
	c := &Controller{
		k8sClient: k8sClient,
		cache: nodesMapping{
			k8sToBMNode: make(map[string]string),
			bmToK8sNode: make(map[string]string),
		},
		enabledForNode:     make(map[string]bool, 3), // a little optimization, if cluster has 3 worker nodes this map won't be extended
		log:                logger.WithField("component", "Controller"),
		externalAnnotation: useExternalAnnotaion,
	}

	if nodeSelector != "" {
		splitted := strings.Split(nodeSelector, ":")
		if len(splitted) != 2 {
			return nil, fmt.Errorf("unable to parse nodeSelector %s", nodeSelector)
		}
		c.nodeSelector = &label{key: splitted[0], value: splitted[1]}
		c.log.Infof("Controller will be working with nodes that matched next selector: %v", c.nodeSelector)
	}

	if c.externalAnnotation {
		c.annotationKey = nodeAnnotaion
		c.log.Infof("External annotation feature is enabled. Annotation: %s", c.annotationKey)
	} else {
		c.annotationKey = common.DeafultNodeIDAnnotationKey
		c.log.Infof("External annotation feature is disabled. Annotation: %s", c.annotationKey)
	}

	return c, nil
}

func (bmc *Controller) enableForNode(nodeName string) {
	bmc.enabledMu.Lock()
	bmc.enabledForNode[nodeName] = true
	bmc.enabledMu.Unlock()
}

func (bmc *Controller) disableForNode(nodeName string) {
	bmc.enabledMu.Lock()
	bmc.enabledForNode[nodeName] = false
	bmc.enabledMu.Unlock()
}

func (bmc *Controller) isEnabledForNode(nodeName string) bool {
	var enabled, ok bool
	bmc.enabledMu.RLock()
	defer bmc.enabledMu.RUnlock()
	if enabled, ok = bmc.enabledForNode[nodeName]; !ok {
		return false
	}

	return enabled
}

func (bmc *Controller) isMatchSelector(k8sNode *coreV1.Node) bool {
	if bmc.nodeSelector == nil {
		return true
	}

	val, ok := k8sNode.GetLabels()[bmc.nodeSelector.key]
	matched := ok && val == bmc.nodeSelector.value
	bmc.log.WithField("method", "isMatchSelector").
		Debugf("Node %s matches node selector %v: %v", k8sNode.Name, bmc.nodeSelector, matched)

	return matched
}

// SetupWithManager registers Controller to k8s controller manager
func (bmc *Controller) SetupWithManager(m ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(m).
		For(&nodecrd.Node{}). // primary resource
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1, // reconcile all object by turn, concurrent reconciliation isn't supported
		}).
		Watches(&source.Kind{Type: &coreV1.Node{}}, &handler.EnqueueRequestForObject{}). // secondary resource
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				if _, ok := e.Object.(*nodecrd.Node); ok {
					return true
				}

				k8sNode, ok := e.Object.(*coreV1.Node)
				if !ok || !bmc.isMatchSelector(k8sNode) {
					return false
				}

				bmc.enableForNode(k8sNode.Name)
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				if _, ok := e.ObjectOld.(*nodecrd.Node); ok {
					return true
				}

				nodeOld, ok := e.ObjectOld.(*coreV1.Node)
				if !ok {
					return false
				}
				nodeNew := e.ObjectNew.(*coreV1.Node)

				if !bmc.isMatchSelector(nodeNew) {
					return false
				}

				if !bmc.isEnabledForNode(nodeNew.Name) {
					bmc.enableForNode(nodeNew.Name)
				}

				annotationAreTheSame := reflect.DeepEqual(nodeOld.GetAnnotations(), nodeNew.GetAnnotations())
				addressesAreTheSame := reflect.DeepEqual(nodeOld.Status.Addresses, nodeNew.Status.Addresses)
				labelsAreTheSame := bmc.nodeSelector == nil || reflect.DeepEqual(nodeOld.GetLabels(), nodeNew.GetLabels())

				return !annotationAreTheSame || !addressesAreTheSame || !labelsAreTheSame
			},
		}).
		Complete(bmc)
}

// Reconcile reconciles Node CR and k8s Node objects
// at first define for which object current Reconcile is triggered and then run corresponding reconciliation method
func (bmc *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ll := bmc.log.WithFields(logrus.Fields{
		"method": "Reconcile",
		"name":   req.Name,
	})

	var err error
	// if name in request doesn't start with namePrefix controller tries to read k8s node object at first
	// however if it get NotFound error it tries to read Node object as well
	if !strings.HasPrefix(req.Name, namePrefix) {
		k8sNode := new(coreV1.Node)
		err = bmc.k8sClient.ReadCR(context.Background(), req.Name, "", k8sNode)
		switch {
		case err == nil:
			ll.Infof("Reconcile k8s node %s", k8sNode.Name)
			return bmc.reconcileForK8sNode(k8sNode)
		case !k8sError.IsNotFound(err):
			ll.Errorf("Unable to read node object: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	// try to read Node
	bmNode := new(nodecrd.Node)
	err = bmc.k8sClient.ReadCR(context.Background(), req.Name, "", bmNode)
	switch {
	case err == nil:
		ll.Infof("Reconcile Node %s", bmNode.Name)
		return bmc.reconcileForCSIBMNode(bmNode)
	case !k8sError.IsNotFound(err):
		ll.Errorf("Unable to read Node object: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	ll.Warnf("unable to detect for which object (%s) that reconcile is. The object may have been deleted", req.String())
	return ctrl.Result{}, nil
}

func (bmc *Controller) reconcileForK8sNode(k8sNode *coreV1.Node) (ctrl.Result, error) {
	ll := bmc.log.WithFields(logrus.Fields{
		"method": "reconcileForK8sNode",
		"name":   k8sNode.Name,
	})

	if len(k8sNode.Status.Addresses) == 0 {
		err := errors.New("addresses are missing for current k8s node instance")
		ll.Error(err)
		return ctrl.Result{Requeue: false}, err
	}

	var (
		bmNode          = &nodecrd.Node{}
		bmNodeFromCache bool
		bmNodeName      string
		bmNodes         []nodecrd.Node
	)
	// get corresponding Node CR name from cache
	if bmNodeName, bmNodeFromCache = bmc.cache.getCSIBMNodeName(k8sNode.Name); bmNodeFromCache {
		if err := bmc.k8sClient.ReadCR(context.Background(), bmNodeName, "", bmNode); err != nil {
			ll.Errorf("Unable to read Node %s: %v", bmNodeName, err)
			return ctrl.Result{Requeue: true}, err
		}
		bmNodes = []nodecrd.Node{*bmNode}
	}

	if !bmNodeFromCache {
		bmNodeCRs := new(nodecrd.NodeList)
		if err := bmc.k8sClient.ReadList(context.Background(), bmNodeCRs); err != nil {
			ll.Errorf("Unable to read Node CRs list: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
		bmNodes = bmNodeCRs.Items
	}

	matchedCRs := make([]string, 0)
	for i := range bmNodes {
		matchedAddresses := bmc.matchedAddressesCount(&bmNodes[i], k8sNode)
		if len(bmNodes[i].Spec.Addresses) > 0 && matchedAddresses == len(bmNodes[i].Spec.Addresses) {
			bmNode = &bmNodes[i]
			matchedCRs = append(matchedCRs, bmNode.Name)
			continue
		}
		if matchedAddresses > 0 {
			ll.Errorf("There is Node %s that partially match k8s node %s. Node.Spec: %v, k8s node addresses: %v. "+
				"Node Spec should be edited to match exactly one kubernetes node",
				bmNodes[i].Name, k8sNode.Name, bmNodes[i].Spec, k8sNode.Status.Addresses)
			return ctrl.Result{}, nil
		}
	}

	if len(matchedCRs) > 1 {
		ll.Errorf("More then one Node CR corresponds to the current k8s node (%d). Matched Node CRs: %v", len(matchedCRs), matchedCRs)
		return ctrl.Result{}, nil
	}

	// create Node CR
	if len(matchedCRs) == 0 {
		id := bmc.constructNodeID(k8sNode)
		bmNodeName := namePrefix + id
		bmNode = bmc.k8sClient.ConstructCSIBMNodeCR(bmNodeName, api.Node{
			UUID:      id,
			Addresses: bmc.constructAddresses(k8sNode),
		})
		bmNode.Finalizers = []string{csibmNodeFinalizer}
		if err := bmc.k8sClient.CreateCR(context.Background(), bmNodeName, bmNode); err != nil {
			ll.Errorf("Unable to create Node CR: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	bmc.cache.put(k8sNode.Name, bmNode.Name)
	return bmc.updateNodeLabelsAndAnnotation(k8sNode, bmNode.Spec.UUID)
}

func (bmc *Controller) reconcileForCSIBMNode(bmNode *nodecrd.Node) (ctrl.Result, error) {
	ll := bmc.log.WithFields(logrus.Fields{
		"method": "reconcileForCSIBMNode",
		"name":   bmNode.Name,
	})

	if len(bmNode.Spec.Addresses) == 0 {
		err := errors.New("addresses are missing for current Node instance")
		ll.Error(err)
		return ctrl.Result{Requeue: false}, err
	}

	var (
		k8sNode          = &coreV1.Node{}
		k8sNodes         []coreV1.Node
		k8sNodeFromCache bool
	)

	// get corresponding k8s node name from cache
	if k8sNodeName, k8sNodeFromCache := bmc.cache.getK8sNodeName(bmNode.Name); k8sNodeFromCache {
		if err := bmc.k8sClient.ReadCR(context.Background(), k8sNodeName, "", k8sNode); err != nil {
			ll.Errorf("Unable to read k8s node %s: %v", k8sNodeName, err)
			return ctrl.Result{Requeue: true}, err
		}
		k8sNodes = []coreV1.Node{*k8sNode}
	}

	if !k8sNodeFromCache {
		k8sNodeCRs := new(coreV1.NodeList)
		if err := bmc.k8sClient.ReadList(context.Background(), k8sNodeCRs); err != nil {
			ll.Errorf("Unable to read k8s nodes list: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
		k8sNodes = k8sNodeCRs.Items
	}

	matchedNodes := make([]string, 0)
	for i := range k8sNodes {
		matchedAddresses := bmc.matchedAddressesCount(bmNode, &k8sNodes[i])
		if matchedAddresses == len(bmNode.Spec.Addresses) {
			k8sNode = &k8sNodes[i]
			matchedNodes = append(matchedNodes, k8sNode.Name)
			continue
		}
		if matchedAddresses > 0 {
			ll.Errorf("There is k8s node %s that partially match Node CR %s. Node.Spec: %v, k8s node addresses: %v",
				k8sNodes[i].Name, bmNode.Name, bmNode.Spec, k8sNodes[i].Status.Addresses)
			return ctrl.Result{}, nil
		}
	}

	if !bmNode.GetDeletionTimestamp().IsZero() {
		bmc.disableForNode(k8sNode.Name)
		if err := bmc.removeLabelsAndAnnotation(k8sNode); err != nil {
			ll.Errorf("Unable to remove annotations or labels from node %s: %v", k8sNode.Name, err)
			bmc.enableForNode(k8sNode.Name)
			return ctrl.Result{Requeue: true}, err
		}

		ll.Infof("Annotations and labels from node %s was removed. Removing finalizer from %s.", k8sNode.Name, bmNode.Name)
		bmNode.Finalizers = nil
		err := bmc.k8sClient.UpdateCR(context.Background(), bmNode)
		if err != nil {
			ll.Errorf("Unable to update Node %s: %v", bmNode.Name, err)
		}
		return ctrl.Result{}, err
	}

	if len(matchedNodes) == 1 {
		bmc.cache.put(k8sNode.Name, bmNode.Name)
		return bmc.updateNodeLabelsAndAnnotation(k8sNode, bmNode.Spec.UUID)
	}

	ll.Warnf("Unable to detect k8s node that corresponds to Node %v, matched nodes: %v", bmNode, matchedNodes)
	return ctrl.Result{}, nil
}

// updateNodeLabelsAndAnnotation checks nodeIDAnnotationKey annotation value for provided k8s Node and compare that value with goalValue
// parses OS Image info and put/update os-name and os-version labels if needed
func (bmc *Controller) updateNodeLabelsAndAnnotation(k8sNode *coreV1.Node, nodeUUID string) (ctrl.Result, error) {
	ll := bmc.log.WithField("method", "updateNodeLabelsAndAnnotation")

	toUpdate := false
	// initialize labels map if needed
	if k8sNode.Labels == nil {
		k8sNode.ObjectMeta.Labels = make(map[string]string, 1)
	}
	// check for annotations
	val, ok := k8sNode.GetAnnotations()[bmc.annotationKey]
	if bmc.externalAnnotation && !ok {
		ll.Errorf("external annotaion %s is not accesible on node %s", bmc.annotationKey, k8sNode)
	}
	if !bmc.externalAnnotation && ok {
		if val == nodeUUID {
			ll.Tracef("%s value for node %s is already %s", bmc.annotationKey, k8sNode.Name, nodeUUID)
		} else {
			ll.Warnf("%s value for node %s is %s, however should have (according to corresponding Node's UUID) %s, going to update annotation's value.",
				bmc.annotationKey, k8sNode.Name, val, nodeUUID)
			k8sNode.ObjectMeta.Annotations[bmc.annotationKey] = nodeUUID
			toUpdate = true
		}
	}
	if !bmc.externalAnnotation && !ok {
		ll.Errorf("annotaion %s is not accesible on node %s", bmc.annotationKey, k8sNode)
		if k8sNode.ObjectMeta.Annotations == nil {
			k8sNode.ObjectMeta.Annotations = make(map[string]string, 1)
		}
		k8sNode.ObjectMeta.Annotations[bmc.annotationKey] = nodeUUID
		toUpdate = true
	}

	if toUpdate {
		if err := bmc.k8sClient.UpdateCR(context.Background(), k8sNode); err != nil {
			ll.Errorf("Unable to update node object: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

func (bmc *Controller) removeLabelsAndAnnotation(k8sNode *coreV1.Node) error {
	toUpdate := false
	// check annotations
	annotations := k8sNode.GetAnnotations()
	if _, ok := annotations[bmc.annotationKey]; ok {
		if !bmc.externalAnnotation {
			delete(annotations, bmc.annotationKey)
			toUpdate = true
		}
	}

	// check labels
	labels := k8sNode.GetLabels()
	// external csi-provisioner label
	// TODO https://github.com/dell/csi-baremetal/issues/319 Rework after operator implementation
	if _, ok := labels[common.NodeIDTopologyLabelKey]; ok {
		delete(labels, common.NodeIDTopologyLabelKey)
		toUpdate = true
	}

	if toUpdate {
		k8sNode.Annotations = annotations
		k8sNode.Labels = labels
		return bmc.k8sClient.UpdateCR(context.Background(), k8sNode)
	}

	return nil
}

// matchedAddressesCount return amount of k8s node addresses that has corresponding address in bmNodeCR.Spec.Addresses map
func (bmc *Controller) matchedAddressesCount(bmNodeCR *nodecrd.Node, k8sNode *coreV1.Node) int {
	matchedCount := 0
	for _, addr := range k8sNode.Status.Addresses {
		crAddr, ok := bmNodeCR.Spec.Addresses[string(addr.Type)]
		if ok && crAddr == addr.Address {
			matchedCount++
		}
	}

	return matchedCount
}

// constructAddresses converts k8sNode.Status.Addresses into the the map[string]string, key - address type, value - address
func (bmc *Controller) constructAddresses(k8sNode *coreV1.Node) map[string]string {
	res := make(map[string]string, len(k8sNode.Status.Addresses))
	for _, addr := range k8sNode.Status.Addresses {
		res[string(addr.Type)] = addr.Address
	}

	return res
}

func (bmc *Controller) constructNodeID(k8sNode *coreV1.Node) string {
	if bmc.externalAnnotation {
		if val, ok := k8sNode.GetAnnotations()[bmc.annotationKey]; ok {
			return val
		}
	}

	return uuid.New().String()
}
