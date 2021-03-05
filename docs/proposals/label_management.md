# Proposal: Node label management

Last updated: 05.03.2021


## Abstract

Removing additional node labels/annotations to prevent possible errors with non-consistent information

## Background

Currently `csi-baremetal-operator` and `node-driver-registrar` sidecar container from `csi-baremetal-driver` use other ways to get special nodeID.

All labels/annotations on a node, which was created with csi-baremetal:

```
Labels:
				nodes.csi-baremetal.dell.com/os-name=ubuntu
				nodes.csi-baremetal.dell.com/os-version=19.10
				nodes.csi-baremetal.dell.com/uuid=98ad16dd-5702-4dfe-bbca-008b65422831
Annotations:
				csi.volume.kubernetes.io/nodeid: {"csi-baremetal":"98ad16dd-5702-4dfe-bbca-008b65422831"}
				nodes.csi-baremetal.dell.com/uuid: 98ad16dd-5702-4dfe-bbca-008b65422831
```

csi-baremetal operator:

- Annotations: **nodes.csi-baremetal.dell.com/uuid: 98ad16dd-5702-4dfe-bbca-008b65422831**

node-driver-registrar:

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

csi-baremetal-operator has to remove k8sNode annotation after its uninstallation. Labels have no responsible controller and it remains on a node.

## Proposal

Use labels instead of annotations to mark work nodes in a cluster.

Proposed algorithm:

```
csi-baremetal-operator/controller : create csibmnode
csi-baremetal-operator/controller : mark k8sNode with `label`
csi-baremetal-node/node : check k8sNode `label`
csi-baremetal-node/node : send nodeID to node-driver-registrar
csi-baremetal-node/node-driver-registrar : get nodeID from node
csi-baremetal-node/node-driver-registrar : create or check k8sNode label with nodeID
csi-baremetal-node/node-driver-registrar : use nodeID in topology
```

## Rationale

Disadvantages:
- It is possible to appear label removing logic in new versions of node-driver-registrar or csi-provisioner. Like `csi.volume.kubernetes.io/nodeid` annotation is removed after node-driver-registrar deletion.

## Compatibility

node-driver-registrar can be upgraded to v2.1.0 (current latest)

## Implementation

In csi-baremetal-operator/controller:

```
updateNodeLabelsAndAnnotation :: AddNodeIdAnnotation() -> updateNodeLabels :: AddNodeIdLabel()
removeLabelsAndAnnotation :: removeNodeIdAnnotation() -> removeNodeLabels :: removeNodeIdLabel()
```

In csi-baremetal-node/node:

```
getNodeId :: k8sNode.GetAnnotations() -> getNodeId :: k8sNode.GetLabels()
```

In extender:

```
getNodeId :: k8sNode.GetAnnotations() -> getNodeId :: k8sNode.GetLabels()
```

In loopbackmanager:

```
getNodeId :: k8sNode.GetAnnotations() -> getNodeId :: k8sNode.GetLabels()
```
