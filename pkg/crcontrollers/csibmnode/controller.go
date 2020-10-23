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

package csibmnode

import (
	"context"
	"errors"
	"reflect"
	"strings"

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
	nodecrd "github.com/dell/csi-baremetal/api/v1/csibmnodecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

const (
	// NodeIDAnnotationKey hold key for annotation for node object
	NodeIDAnnotationKey = "csibmnodes.csi-baremetal.dell.com/uuid"
	// namePrefix it is a prefix for CSIBMNode CR name
	namePrefix = "csibmnode-"
)

// Controller is a controller for CSIBMNode CR
type Controller struct {
	k8sClient *k8s.KubeClient
	cache     nodesMapping

	log *logrus.Entry
}

// nodesMapping it is not a thread safety cache that holds mapping between names for k8s node and BMCSINode CR objects
type nodesMapping struct {
	k8sToBMNode map[string]string // k8s node name to CSIBMNode CR name
	bmToK8sNode map[string]string // CSIBMNode CR name to k8s node name
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
func NewController(k8sClient *k8s.KubeClient, logger *logrus.Logger) (*Controller, error) {
	return &Controller{
		k8sClient: k8sClient,
		cache: nodesMapping{
			k8sToBMNode: make(map[string]string),
			bmToK8sNode: make(map[string]string),
		},
		log: logger.WithField("component", "Controller"),
	}, nil
}

// SetupWithManager registers Controller to k8s controller manager
func (bmc *Controller) SetupWithManager(m ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(m).
		For(&nodecrd.CSIBMNode{}). // primary resource
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1, // reconcile all object by turn, concurrent reconciliation isn't supported
		}).
		Watches(&source.Kind{Type: &coreV1.Node{}}, &handler.EnqueueRequestForObject{}). // secondary resource
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				if _, ok := e.ObjectOld.(*nodecrd.CSIBMNode); ok {
					return true
				}

				nodeOld, ok := e.ObjectOld.(*coreV1.Node)
				if !ok {
					return false
				}
				nodeNew := e.ObjectNew.(*coreV1.Node)

				annotationAreTheSame := reflect.DeepEqual(nodeOld.GetAnnotations(), nodeNew.GetAnnotations())
				addressesAreTheSame := reflect.DeepEqual(nodeOld.Status.Addresses, nodeNew.Status.Addresses)
				bmc.log.Debugf("UpdateEvent for k8s node %s. Annotations are the same - %v. Addresses are the same - %v.",
					nodeOld.Name, annotationAreTheSame, addressesAreTheSame)

				return !annotationAreTheSame || !addressesAreTheSame
			},
		}).
		Complete(bmc)
}

// Reconcile reconcile CSIBMNode CR and k8s CSIBMNode objects
// at first define for which object current Reconcile is triggered and then run corresponding reconciliation method
func (bmc *Controller) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ll := bmc.log.WithFields(logrus.Fields{
		"method": "Reconcile",
		"name":   req.Name,
	})

	var err error
	// if name in request doesn't start with namePrefix controller tries to read k8s node object at first
	// however if it get NotFound error it tries to read CSIBMNode object as well
	if !strings.HasPrefix(req.Name, namePrefix) {
		k8sNode := new(coreV1.Node)
		err = bmc.k8sClient.ReadCR(context.Background(), req.Name, k8sNode)
		switch {
		case err == nil:
			ll.Infof("Reconcile k8s node %s", k8sNode.Name)
			return bmc.reconcileForK8sNode(k8sNode)
		case !k8sError.IsNotFound(err):
			ll.Errorf("Unable to read node object: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	// try to read CSIBMNode
	bmNode := new(nodecrd.CSIBMNode)
	err = bmc.k8sClient.ReadCR(context.Background(), req.Name, bmNode)
	switch {
	case err == nil:
		ll.Infof("Reconcile CSIBMNode %s", bmNode.Name)
		return bmc.reconcileForCSIBMNode(bmNode)
	case !k8sError.IsNotFound(err):
		ll.Errorf("Unable to read CSIBMNode object: %v", err)
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

	// get corresponding CSIBMNode CR name from cache
	var bmNode = &nodecrd.CSIBMNode{}
	if bmNodeName, ok := bmc.cache.getCSIBMNodeName(k8sNode.Name); ok {
		if err := bmc.k8sClient.ReadCR(context.Background(), bmNodeName, bmNode); err != nil {
			ll.Errorf("Unable to read CSIBMNode %s: %v", bmNodeName, err)
			return ctrl.Result{Requeue: true}, err
		}
		return bmc.updateAnnotation(k8sNode, bmNode.Spec.UUID)
	}

	// search corresponding CSIBMNode CR name in k8s API
	bmNodeCRs := new(nodecrd.CSIBMNodeList)
	if err := bmc.k8sClient.ReadList(context.Background(), bmNodeCRs); err != nil {
		ll.Errorf("Unable to read CSIBMNode CRs list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	matchedCRs := make([]string, 0)
	for i := range bmNodeCRs.Items {
		matchedAddresses := bmc.matchedAddressesCount(&bmNodeCRs.Items[i], k8sNode)
		if len(bmNodeCRs.Items[i].Spec.Addresses) > 0 && matchedAddresses == len(bmNodeCRs.Items[i].Spec.Addresses) {
			bmNode = &bmNodeCRs.Items[i]
			matchedCRs = append(matchedCRs, bmNode.Name)
			continue
		}
		if matchedAddresses > 0 {
			ll.Errorf("There is CSIBMNode %s that partially match k8s node %s. CSIBMNode.Spec: %v, k8s node addresses: %v. "+
				"CSIBMNode Spec should be edited to match exactly one kubernetes node",
				bmNodeCRs.Items[i].Name, k8sNode.Name, bmNodeCRs.Items[i].Spec, k8sNode.Status.Addresses)
			return ctrl.Result{}, nil
		}
	}

	if len(matchedCRs) > 1 {
		ll.Warnf("More then one CSIBMNode CR corresponds to the current k8s node (%d). Matched CSIBMNode CRs: %v", len(matchedCRs), matchedCRs)
		return ctrl.Result{}, nil
	}

	// create CSIBMNode CR
	if len(matchedCRs) == 0 {
		id := uuid.New().String()
		bmNodeName := namePrefix + id
		bmNode = bmc.k8sClient.ConstructCSIBMNodeCR(bmNodeName, api.CSIBMNode{
			UUID:      id,
			Addresses: bmc.constructAddresses(k8sNode),
		})
		if err := bmc.k8sClient.CreateCR(context.Background(), bmNodeName, bmNode); err != nil {
			ll.Errorf("Unable to create CSIBMNode CR: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	bmc.cache.put(k8sNode.Name, bmNode.Name)
	return bmc.updateAnnotation(k8sNode, bmNode.Spec.UUID)
}

func (bmc *Controller) reconcileForCSIBMNode(bmNode *nodecrd.CSIBMNode) (ctrl.Result, error) {
	ll := bmc.log.WithFields(logrus.Fields{
		"method": "reconcileForCSIBMNode",
		"name":   bmNode.Name,
	})

	if len(bmNode.Spec.Addresses) == 0 {
		err := errors.New("addresses are missing for current CSIBMNode instance")
		ll.Error(err)
		return ctrl.Result{Requeue: false}, err
	}

	// get corresponding k8s node name from cache
	var k8sNode = &coreV1.Node{}
	if k8sNodeName, ok := bmc.cache.getK8sNodeName(bmNode.Name); ok {
		if err := bmc.k8sClient.ReadCR(context.Background(), k8sNodeName, k8sNode); err != nil {
			ll.Errorf("Unable to read k8s node %s: %v", k8sNodeName, err)
			return ctrl.Result{Requeue: true}, err
		}
		return bmc.updateAnnotation(k8sNode, bmNode.Spec.UUID)
	}

	// search corresponding k8s node name in k8s API
	k8sNodes := new(coreV1.NodeList)
	if err := bmc.k8sClient.ReadList(context.Background(), k8sNodes); err != nil {
		ll.Errorf("Unable to read k8s nodes list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	matchedNodes := make([]string, 0)
	for i := range k8sNodes.Items {
		matchedAddresses := bmc.matchedAddressesCount(bmNode, &k8sNodes.Items[i])
		if matchedAddresses == len(bmNode.Spec.Addresses) {
			k8sNode = &k8sNodes.Items[i]
			matchedNodes = append(matchedNodes, k8sNode.Name)
			continue
		}
		if matchedAddresses > 0 {
			ll.Errorf("There is k8s node %s that partially match CSIBMNode CR %s. CSIBMNode.Spec: %v, k8s node addresses: %v",
				k8sNodes.Items[i].Name, bmNode.Name, bmNode.Spec, k8sNodes.Items[i].Status.Addresses)
			return ctrl.Result{}, nil
		}
	}

	if len(matchedNodes) == 1 {
		bmc.cache.put(k8sNode.Name, bmNode.Name)
		return bmc.updateAnnotation(k8sNode, bmNode.Spec.UUID)
	}

	ll.Warnf("Unable to detect k8s node that corresponds to CSIBMNode %v, matched nodes: %v", bmNode, matchedNodes)
	return ctrl.Result{}, nil
}

// updateAnnotation checks NodeIDAnnotationKey annotation value for provided k8s CSIBMNode and compare that value with goalValue
// update k8s CSIBMNode object if needed, method is used as a last step of Reconcile
func (bmc *Controller) updateAnnotation(k8sNode *coreV1.Node, goalValue string) (ctrl.Result, error) {
	ll := bmc.log.WithField("method", "updateAnnotation")
	val, ok := k8sNode.GetAnnotations()[NodeIDAnnotationKey]
	switch {
	case ok && val == goalValue:
		// nothing to do
	case ok && val != goalValue:
		ll.Warnf("%s value for node %s is %s, however should have (according to corresponding CSIBMNode's UUID) %s, going to update annotation's value.",
			NodeIDAnnotationKey, k8sNode.Name, val, goalValue)
		fallthrough
	default:
		if k8sNode.ObjectMeta.Annotations == nil {
			k8sNode.ObjectMeta.Annotations = make(map[string]string, 1)
		}
		k8sNode.ObjectMeta.Annotations[NodeIDAnnotationKey] = goalValue
		if err := bmc.k8sClient.UpdateCR(context.Background(), k8sNode); err != nil {
			ll.Errorf("Unable to update node object: %v", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

// matchedAddressesCount return amount of k8s node addresses that has corresponding address in bmNodeCR.Spec.Addresses map
func (bmc *Controller) matchedAddressesCount(bmNodeCR *nodecrd.CSIBMNode, k8sNode *coreV1.Node) int {
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
