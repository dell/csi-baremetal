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
	"reflect"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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
	NodeIDAnnotationKey = "dell.csi-baremetal.node/id"
)

type CSIBMController struct {
	k8sClient *k8s.KubeClient

	log *logrus.Entry
}

func NewCSIBMController(namespace string, logger *logrus.Logger) (*CSIBMController, error) {
	k8sClient, err := k8s.GetK8SClient()
	if err != nil {
		return nil, err
	}
	kubeClient := k8s.NewKubeClient(k8sClient, logger, namespace)

	return &CSIBMController{
		k8sClient: kubeClient,
		log:       logrus.WithField("component", "CSIBMController"),
	}, nil
}

func (bmc *CSIBMController) SetupWithManager(m ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(m).
		For(&nodecrd.CSIBMNode{}).
		Watches(&source.Kind{Type: &coreV1.Node{}}, &handler.EnqueueRequestForObject{}).
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

				return !reflect.DeepEqual(nodeOld.GetAnnotations(), nodeNew.GetAnnotations()) ||
					!reflect.DeepEqual(nodeOld.Status.Addresses, nodeNew.Status.Addresses)
			},
		}).
		Complete(bmc)
}

func (bmc *CSIBMController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ll := bmc.log.WithFields(logrus.Fields{
		"method":  "Reconcile",
		"reqName": req.Name,
	})

	ll.Info("Reconciling ...")

	var err error

	k8sNodesList := new(coreV1.NodeList)
	if err = bmc.k8sClient.ReadList(context.Background(), k8sNodesList); err != nil {
		ll.Errorf("Unable to read k8s nodes list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	csiNodesList := new(nodecrd.CSIBMNodeList)
	if err = bmc.k8sClient.ReadList(context.Background(), csiNodesList); err != nil {
		ll.Errorf("Unable to read CSIBMNode CRs list: %v", err)
		return ctrl.Result{Requeue: true}, err
	}

	var k8sNodeToCSINode = make(map[*coreV1.Node]*nodecrd.CSIBMNode, len(k8sNodesList.Items))
	for i := 0; i < len(k8sNodesList.Items); i++ {
		exist := false
		k8sNode := &k8sNodesList.Items[i]
		for j := 0; j < len(csiNodesList.Items); j++ {
			csiNode := &csiNodesList.Items[j]

			matchedAddresses := bmc.matchedAddressesCount(csiNode, k8sNode)
			if matchedAddresses == len(k8sNode.Status.Addresses) {
				// TODO: what if we have more then 1 CR points on same k8s node?
				k8sNodeToCSINode[&k8sNodesList.Items[i]] = &csiNodesList.Items[j]
				exist = true
				break
			}
			if matchedAddresses > 0 {
				// TODO: match but not fully
			}
		}
		if !exist {
			k8sNodeToCSINode[&k8sNodesList.Items[i]] = nil
		}
	}

	for k8sNode, csiNode := range k8sNodeToCSINode {
		if csiNode == nil {
			name := uuid.New().String()
			toCreate := bmc.k8sClient.ConstructCSIBMNodeCR(name, api.CSIBMNode{
				UUID:        uuid.New().String(),
				NodeAddress: bmc.constructAddresses(k8sNode),
			})

			ll.Infof("Going to create CSIBMNode CR with spec: %v", toCreate.Spec)
			err := bmc.k8sClient.CreateCR(context.Background(), toCreate.Spec.UUID, toCreate)
			if err != nil {
				ll.Errorf("Unable to create CSIBMNode CR %v: %v", toCreate, err)
			}
			return ctrl.Result{Requeue: true}, err
		}

		// check annotation on the node object
		needToUpdateK8sNode := true
		val, ok := k8sNode.GetAnnotations()[NodeIDAnnotationKey]
		switch {
		case !ok:
			ll.Infof("K8s node %s doesn't have %s annotation. Going to set it to %s.",
				k8sNode.Name, NodeIDAnnotationKey, csiNode.Spec.UUID)
		case val != csiNode.Spec.UUID:
			ll.Infof("K8s node %s has another value for annotation %s. Current - %s, in CSIBMNode CR - %s. Going to update it.",
				k8sNode.Name, NodeIDAnnotationKey, val, csiNode.Spec.UUID)
		default:
			needToUpdateK8sNode = false
		}
		if needToUpdateK8sNode {
			k8sNode.GetAnnotations()[NodeIDAnnotationKey] = csiNode.Spec.UUID
			err := bmc.k8sClient.UpdateCR(context.Background(), k8sNode)
			if err != nil {
				ll.Errorf("Unable to update node object: %v", err)
			}
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
}

func (bmc *CSIBMController) getK8sNodeAddr(k8sNode *coreV1.Node, addrType coreV1.NodeAddressType) string {
	for _, addr := range k8sNode.Status.Addresses {
		if addr.Type == addrType {
			return addr.Address
		}
	}
	return ""
}

func (bmc *CSIBMController) matchedAddressesCount(bmNodeCR *nodecrd.CSIBMNode, k8sNode *coreV1.Node) int {
	matchedCount := 0
	for _, addr := range k8sNode.Status.Addresses {
		crAddr, ok := bmNodeCR.Spec.NodeAddress[string(addr.Type)]
		if ok && crAddr == addr.Address {
			matchedCount++
		}
	}

	return matchedCount
}

func (bmc *CSIBMController) constructAddresses(k8sNode *coreV1.Node) map[string]string {
	res := make(map[string]string, len(k8sNode.Status.Addresses))
	for _, addr := range k8sNode.Status.Addresses {
		res[string(addr.Type)] = addr.Address
	}

	return res
}
