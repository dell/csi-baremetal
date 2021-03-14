# Proposal: Node label management

Last updated: 11.03.2021


## Abstract

Removing additional node labels/annotations to prevent possible errors with non-consistent information

## Background

Currently `csi-baremetal-operator` and `node-driver-registrar` sidecar container from `csi-baremetal-driver` use other ways to get special nodeID.

All labels/annotations with UUID on a node, which was created with csi-baremetal:

```
Labels:
	nodes.csi-baremetal.dell.com/uuid=98ad16dd-5702-4dfe-bbca-008b65422831
Annotations:
	csi.volume.kubernetes.io/nodeid: {"csi-baremetal":"98ad16dd-5702-4dfe-bbca-008b65422831"}
	nodes.csi-baremetal.dell.com/uuid: 98ad16dd-5702-4dfe-bbca-008b65422831
```

csi-baremetal operator:

- Annotations: **nodes.csi-baremetal.dell.com/uuid: 98ad16dd-5702-4dfe-bbca-008b65422831**

node-driver-registrar + csi-provisioner:

- Labels: **nodes.csi-baremetal.dell.com/uuid=98ad16dd-5702-4dfe-bbca-008b65422831**

- Annotations: csi.volume.kubernetes.io/nodeid: {"csi-baremetal":"98ad16dd-5702-4dfe-bbca-008b65422831"}

There are 2 copies of the same information in label section and annotation section.

Problem:

```
csi-baremetal-operator/controller : create csibmnode
csi-baremetal-operator/controller : mark k8sNode with annotation
csi-baremetal-node/node : check k8sNode annotation
csi-baremetal-node/node : send nodeID to node-driver-registrar
csi-baremetal-node/node-driver-registrar : get nodeID from node
csi-baremetal-node/node-driver-registrar : create or check k8sNode label with nodeID
---Error if nodeID from node and nodeID in label are different---
csi-baremetal-node/node-driver-registrar : use nodeID in topology
```

csi-baremetal-operator has to remove k8sNode UUID-annotation after its uninstallation. UUID-label have no responsible controller and it remains on a node.

## Proposal

1. Remove label `nodes.csi-baremetal.dell.com/uuid` after `csi-baremetal-controller` and `node-daemonset` uninstallation.

It can be done in `csi-baremetal-node/node` while uninstalling csi-baremetal CR

2. Add external annotation usage feature:

In `csi-baremetal-operator`, `csi-baremetal-node/node`, `extender`, `loopbackmanager`: 

```
check if annotationKey is exist
use annotaionValue as UUID
```

Update Helm charts:

```
feature:
  useexternalannotation: false
  nodeIDAnnotation:
```

Set parameters as additional flags:

```
--useexternalannotation={{ .Values.feature.useexternalannotation }}
--nodeidannotation={{ .Values.feature.nodeIDAnnotation }}
```

## Rationale

Removing label `nodes.csi-baremetal.dell.com/uuid`:

- in `csi-baremetal-node/node` after receiving SIGTERM (if node-container has error and is deleted, label will be removed)
- in `csi-baremetal-operator` after uninstalling csibmnode (node-components may exist, when user or other service delete csibmnode)
- in `csi-baremetal-CR-operator` while uninstalling csi-baremetal CR after all other components

## Compatibility

There is no problem with compatibility

## Implementation

Create common function with the follow signature:

```
func GetNodeID(k8sNode corev1.Node, annotationKey string, featureChecker featureconfig.FeatureChecker) (string, error)
```

or

```
func GetNodeIDByName(client k8sClient.Client, nodeName string, annotationKey string, featureChecker featureconfig.FeatureChecker) (string, error)

```

## Open issues

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | What means csibmnode delete action? | Now csibmnode-CR can be deleted by user, when node-components are running | Open | Node-component can't be restarted, if there is no csibmnode for this k8sNode