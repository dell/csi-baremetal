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

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
		Owns(&coreV1.Node{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				switch e.Object.(type) {
				case *coreV1.Node:
					bmc.log.Infof("CreateEvent. Type - Node, name - %s", e.Object.(*coreV1.Node).Name)
				case *nodecrd.CSIBMNode:
					bmc.log.Infof("CreateEvent. Type - CSIBMNode, name - %s", e.Object.(*nodecrd.CSIBMNode).Name)
				default:
					bmc.log.Infof("CreateEvent. Not interesting object, kind - ", e.Object.GetObjectKind())
				}
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				switch e.Object.(type) {
				case *coreV1.Node:
					bmc.log.Infof("DeleteEvent. Type - Node, name - %s", e.Object.(*coreV1.Node).Name)
				case *nodecrd.CSIBMNode:
					bmc.log.Infof("DeleteEvent. Type - CSIBMNode, name - %s", e.Object.(*nodecrd.CSIBMNode).Name)
				default:
					bmc.log.Infof("DeleteEvent. Not interesting object, kind - ", e.Object.GetObjectKind())
				}
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				switch e.ObjectOld.(type) {
				case *coreV1.Node:
					bmc.log.Infof("UpdateEvent. Type - Node, name - %s", e.ObjectOld.(*coreV1.Node).Name)
				case *nodecrd.CSIBMNode:
					bmc.log.Infof("UpdateEvent. Type - CSIBMNode, name - %s", e.ObjectOld.(*nodecrd.CSIBMNode).Name)
				default:
					bmc.log.Infof("UpdateEvent. Not interesting object, kind - ", e.ObjectOld.GetObjectKind())
				}
				return true
			},
			GenericFunc: nil,
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
		for j := 0; j < len(csiNodesList.Items); j++ {
			if bmc.isCRRepresentsNode(&csiNodesList.Items[j], &k8sNodesList.Items[i]) {
				// TODO: what if we have more then 1 CR points on same k8s node?
				k8sNodeToCSINode[&k8sNodesList.Items[i]] = &csiNodesList.Items[j]
				exist = true
				break
			}
		}
		if !exist {
			k8sNodeToCSINode[&k8sNodesList.Items[i]] = nil
		}
	}

	for k8sNode, csiNode := range k8sNodeToCSINode {
		if csiNode == nil {
			toCreate := bmc.k8sClient.ConstructCSIBMNodeCR(api.CSIBMNode{
				UUID:     uuid.New().String(),
				IPs:      []string{bmc.getK8sNodeAddr(k8sNode, coreV1.NodeInternalIP)},
				Hostname: bmc.getK8sNodeAddr(k8sNode, coreV1.NodeHostName),
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

func (bmc *CSIBMController) isCRRepresentsNode(bmNodeCR *nodecrd.CSIBMNode, k8sNode *coreV1.Node) bool {
	comparesCount := 0
	for _, addr := range k8sNode.Status.Addresses {
		switch addr.Type {
		case coreV1.NodeHostName:
			if addr.Address != bmNodeCR.Spec.Hostname {
				return false
			}
			comparesCount++
		case coreV1.NodeInternalIP:
			if addr.Address != bmNodeCR.Spec.IPs[0] {
				return false
			}
			comparesCount++
		}
	}
	return comparesCount == 2 // expect that hostname and Internal IP address are the same
}
